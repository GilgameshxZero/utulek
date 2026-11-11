package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const maxAttachmentBytes = 64 * 1024 * 1024

type attachmentRequest struct {
	sourceID string
	result   chan attachmentResult
}

type attachmentResult struct {
	data        []byte
	contentType string
	err         error
}

// Google Voice serves attachment variants through /u/<account>/a/i/<id>?s=N,
// where s=1 is the original and 2..4 are smaller renditions.
func (b *Browser) attachment(sourceID string) ([]byte, string, error) {
	if sourceID == "" || len(sourceID) > 512 || strings.ContainsAny(sourceID, "/\\?#\"' \t\r\n") {
		return nil, "", errors.New("invalid attachment source ID")
	}
	address := b.base + "a/i/" + url.PathEscape(sourceID) + "?s=1"
	ctx, cancel := context.WithTimeout(b.ctx, 90*time.Second)
	defer cancel()
	var response struct {
		Status int    `json:"status"`
		Type   string `json:"type"`
		Data   string `json:"data"`
		Error  string `json:"error"`
	}
	js := `(async()=>{try{
		const r=await fetch(` + literal(address) + `,{credentials:'include'});
		if(!r.ok) return {status:r.status,type:'',data:'',error:''};
		const buffer=await r.arrayBuffer();
		if(buffer.byteLength>` + fmt.Sprint(maxAttachmentBytes) + `) return {status:r.status,type:'',data:'',error:'too large'};
		const bytes=new Uint8Array(buffer); let binary=''; const chunk=0x8000;
		for(let i=0;i<bytes.length;i+=chunk){binary+=String.fromCharCode.apply(null,bytes.subarray(i,i+chunk));}
		return {status:r.status,type:r.headers.get('content-type')||'',data:btoa(binary),error:''};
	}catch(e){return {status:-1,type:'',data:'',error:String(e)}}})()`
	if e := chromedp.Run(ctx, chromedp.Evaluate(js, &response, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) })); e != nil {
		return nil, "", errors.New("attachment transport unavailable")
	}
	if response.Error != "" {
		return nil, "", errors.New("attachment download failed: " + response.Error)
	}
	if response.Status != 200 {
		return nil, "", fmt.Errorf("attachment HTTP %d", response.Status)
	}
	data, e := base64.StdEncoding.DecodeString(response.Data)
	if e != nil || len(data) > maxAttachmentBytes {
		return nil, "", errors.New("attachment is incomplete or too large")
	}
	contentType := response.Type
	if cut := strings.Index(contentType, ";"); cut >= 0 {
		contentType = contentType[:cut]
	}
	return data, contentType, nil
}

func cacheName(sourceID string) string {
	sum := sha256.Sum256([]byte(sourceID))
	return hex.EncodeToString(sum[:])
}

// Attachments are fetched lazily and cached on disk, so opening a conversation
// does not download every media item and repeat views do not re-fetch.
func (s *Bridge) attachmentBytes(attachment Attachment) ([]byte, string, error) {
	if attachment.SourceID == "" {
		return nil, "", errors.New("attachment source unavailable")
	}
	path := filepath.Join(s.home, "media", cacheName(attachment.SourceID))
	if data, e := os.ReadFile(path); e == nil {
		return data, attachment.ContentType, nil
	}
	request := attachmentRequest{sourceID: attachment.SourceID, result: make(chan attachmentResult, 1)}
	select {
	case s.attachments <- request:
	case <-time.After(5 * time.Second):
		return nil, "", errors.New("attachment worker busy")
	}
	select {
	case result := <-request.result:
		if result.err != nil {
			return nil, "", result.err
		}
		if result.contentType != "" {
			attachment.ContentType = result.contentType
		}
		if e := privateDir(filepath.Dir(path)); e == nil {
			if e = writePrivate(path, result.data); e != nil {
				return result.data, attachment.ContentType, nil
			}
			_ = pruneMediaCache(filepath.Dir(path), cacheLimitBytes(s.config))
		}
		return result.data, attachment.ContentType, nil
	case <-time.After(100 * time.Second):
		return nil, "", errors.New("attachment download timed out")
	}
}

func writePrivate(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".media-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

func cacheLimitBytes(c Config) int64 {
	limit := c.CacheLimitMB
	if limit <= 0 {
		limit = 1024
	}
	return int64(limit) * 1024 * 1024
}

// Keep the lazy attachment cache bounded by removing the least recently
// modified files first. Only regular files in the dedicated media directory
// are candidates; state and credentials are never touched.
func pruneMediaCache(dir string, limit int64) error {
	entries, e := os.ReadDir(dir)
	if e != nil {
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	type item struct {
		path string
		size int64
		mod  time.Time
	}
	items := []item{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			continue
		}
		items = append(items, item{filepath.Join(dir, entry.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= limit {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.Before(items[j].mod) })
	for _, item := range items {
		if total <= limit {
			break
		}
		if e := os.Remove(item.path); e == nil {
			total -= item.size
		}
	}
	return nil
}
