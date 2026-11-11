package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var mimePattern = regexp.MustCompile(`^(image|video|audio|application|text)/[a-zA-Z0-9.+-]+$`)

// Read-only discovery of attachment metadata and media transport. Writes shapes,
// status codes and byte counts only; never writes attachment bytes or names.
func TestLiveMediaProbe(t *testing.T) {
	if os.Getenv("GV_LIVE_MEDIA") != "1" {
		t.Skip("explicit opt-in; read only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	body, e := b.voiceEndpoint(coldSyncPath, coldSyncRequest(c))
	if e != nil {
		t.Fatal(e)
	}
	var v any
	if e = json.Unmarshal(body, &v); e != nil {
		t.Fatal(e)
	}
	var shapes []map[string]any
	var ids []string
	var mediaURLs []string
	var threadID string
	var walk func(any, string)
	walk = func(v any, thread string) {
		switch x := v.(type) {
		case []any:
			if len(x) > 0 {
				if candidate, ok := x[0].(string); ok && strings.HasPrefix(candidate, "t.") {
					thread = candidate
				}
			}
			if len(x) >= 15 {
				media, _ := x[14].([]any)
				if len(media) >= 3 {
					atts, _ := media[2].([]any)
					for _, raw := range atts {
						att, ok := raw.([]any)
						if !ok || len(att) < 2 {
							continue
						}
						shape := map[string]any{"length": len(att)}
						fields := []string{}
						mimeAt := []int{}
						for i, field := range att {
							switch value := field.(type) {
							case string:
								kind := fmt.Sprintf("string:%d", len(value))
								if mimePattern.MatchString(value) {
									kind = "mime"
									mimeAt = append(mimeAt, i)
								}
								fields = append(fields, kind)
							case float64:
								fields = append(fields, "number")
							case nil:
								fields = append(fields, "null")
							case []any:
								fields = append(fields, fmt.Sprintf("array:%d", len(value)))
							default:
								fields = append(fields, fmt.Sprintf("%T", value))
							}
						}
						shape["fields"] = fields
						shape["mime_at"] = mimeAt
						if len(att) > 8 {
							if value, ok := att[8].(string); ok {
								shape["tail_len"] = len(value)
								if u, err := url.Parse(value); err == nil && u.Host != "" {
									shape["tail_host"] = u.Host
									shape["tail_path_prefix"] = u.Path[:min(24, len(u.Path))]
									shape["tail_query_len"] = len(u.RawQuery)
									if u.Scheme == "https" && len(mediaURLs) < 2 {
										mediaURLs = append(mediaURLs, value)
									}
								}
							}
						}
						if len(att) > 3 {
							if parts, ok := att[3].([]any); ok {
								kinds := []string{}
								for _, part := range parts {
									kinds = append(kinds, fmt.Sprintf("%T", part))
								}
								shape["part_types"] = kinds
							}
						}
						shapes = append(shapes, shape)
						if id, ok := att[1].(string); ok && id != "" && len(ids) < 4 {
							ids = append(ids, id)
						}
						if threadID == "" && thread != "" {
							threadID = thread
						}
					}
				}
			}
			for _, child := range x {
				walk(child, thread)
			}
		case map[string]any:
			for _, child := range x {
				walk(child, thread)
			}
		}
	}
	walk(v, "")
	if len(ids) == 0 {
		t.Fatal("no attachment IDs found")
	}
	urls := append([]string{}, mediaURLs...)
	for _, id := range ids[:1] {
		urls = append(urls,
			b.base+"media/"+id,
			"https://clients6.google.com/voice/v1/voiceclient/media/"+id,
			"https://clients6.google.com/voice/v1/voiceclient/attachment/"+id,
		)
	}
	type fetchResult struct {
		URL         string `json:"url"`
		Status      int    `json:"status"`
		ContentType string `json:"content_type"`
		Bytes       int    `json:"bytes"`
	}
	results := []fetchResult{}
	for _, u := range urls {
		var result fetchResult
		result.URL = u
		js := `(async()=>{try{const r=await fetch(` + literal(u) + `,{credentials:'include'});const b=await r.arrayBuffer();return {status:r.status,type:r.headers.get('content-type')||'',len:b.byteLength}}catch(e){return {status:-1,type:String(e),len:0}}})()`
		var response struct {
			Status int    `json:"status"`
			Type   string `json:"type"`
			Len    int    `json:"len"`
		}
		_ = b.run(chromedp.Evaluate(js, &response, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
		result.Status = response.Status
		result.ContentType = response.Type
		result.Bytes = response.Len
		results = append(results, result)
	}
	type mediaHit struct {
		URL      string `json:"url"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		QueryLen int    `json:"query_len"`
		MIME     string `json:"mime"`
		Status   int    `json:"status"`
	}
	hits := []mediaHit{}
	var mu sync.Mutex
	chromedp.ListenTarget(b.ctx, func(ev any) {
		response, ok := ev.(*network.EventResponseReceived)
		if !ok {
			return
		}
		u, e := url.Parse(response.Response.URL)
		if e != nil {
			return
		}
		mimeType := response.Response.MimeType
		if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") && !strings.HasPrefix(mimeType, "audio/") && !strings.Contains(u.Path, "/media/") && !strings.Contains(u.Path, "/attachment") {
			return
		}
		mu.Lock()
		hits = append(hits, mediaHit{URL: response.Response.URL, Host: u.Host, Path: u.Path[:min(40, len(u.Path))], QueryLen: len(u.RawQuery), MIME: mimeType, Status: int(response.Response.Status)})
		mu.Unlock()
	})
	if threadID != "" {
		peer := strings.TrimPrefix(threadID, "t.")
		_ = b.run(network.Enable(), chromedp.Navigate(b.base+"messages"))
		time.Sleep(6 * time.Second)
		var clicked int
		clickJS := `(() => {
			const peer = ` + literal(peer) + `;
			const nodes = [...document.querySelectorAll('a,[role="link"],[role="button"],div')];
			const hits = nodes.filter(e => ((e.getAttribute('aria-label')||'').includes(peer)) || ((e.href||'').includes(encodeURIComponent(peer))) || ((e.getAttribute('data-phone-number')||'').includes(peer)));
			if (hits.length) { hits[hits.length-1].click(); return hits.length; }
			return 0;
		})()`
		_ = b.run(chromedp.Evaluate(clickJS, &clicked))
		time.Sleep(14 * time.Second)
		_ = b.run(chromedp.Evaluate(`(() => { const s=[...document.querySelectorAll('*')].find(e=>e.scrollHeight>e.clientHeight+100 && /message|thread|conversation/i.test(e.className)); if(s){s.scrollTop=s.scrollHeight;} window.scrollTo(0,document.body.scrollHeight); return true; })()`, nil))
		time.Sleep(6 * time.Second)
		t.Logf("clicked candidates=%d", clicked)
	}
	var binaryFetch fetchResult
	var page struct {
		URL    string `json:"url"`
		Title  string `json:"title"`
		Images []struct {
			Src      string `json:"src"`
			NaturalW int    `json:"natural_width"`
			NaturalH int    `json:"natural_height"`
			Complete bool   `json:"complete"`
		} `json:"images"`
		Media []string `json:"media"`
	}
	_ = b.run(chromedp.Evaluate(`(() => {
		const imgs = [...document.images].map(i => ({src: i.currentSrc || i.src, natural_width: i.naturalWidth, natural_height: i.naturalHeight, complete: i.complete}));
		const media = performance.getEntriesByType('resource').map(r => r.name).filter(u => /media|attachment|googleusercontent/.test(u));
		return {url: location.href, title: document.title, images: imgs.slice(0, 20), media: media.slice(0, 8)};
	})()`, &page))
	pageShapes := map[string]any{"url": page.URL, "title": page.Title, "images": []map[string]any{}, "media": []string{}}
	var loadedMediaURL string
	for _, image := range page.Images {
		if u, e := url.Parse(image.Src); e == nil && u.Host != "" {
			entry := map[string]any{"host": u.Host, "path": u.Path[:min(40, len(u.Path))], "width": image.NaturalW, "height": image.NaturalH, "complete": image.Complete}
			pageShapes["images"] = append(pageShapes["images"].([]map[string]any), entry)
			if loadedMediaURL == "" && image.NaturalW > 0 && (strings.Contains(u.Path, "/media/") || strings.Contains(u.Path, "/attachment")) {
				loadedMediaURL = image.Src
			}
		}
	}
	for _, value := range page.Media {
		if u, e := url.Parse(value); e == nil && u.Host != "" {
			pageShapes["media"] = append(pageShapes["media"].([]string), u.Host+u.Path[:min(40, len(u.Path))])
		}
	}
	if loadedMediaURL != "" {
		js := `(async()=>{try{const r=await fetch(` + literal(loadedMediaURL) + `,{credentials:'include',headers:{Accept:'image/*,*/*'}});const b=await r.arrayBuffer();return {status:r.status,type:r.headers.get('content-type')||'',len:b.byteLength}}catch(e){return {status:-1,type:String(e),len:0}}})()`
		var response struct {
			Status int    `json:"status"`
			Type   string `json:"type"`
			Len    int    `json:"len"`
		}
		_ = b.run(chromedp.Evaluate(js, &response, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
		binaryFetch = fetchResult{URL: "loaded-image", Status: response.Status, ContentType: response.Type, Bytes: response.Len}
	}
	mu.Lock()
	hitCopy := append([]mediaHit(nil), hits...)
	mu.Unlock()
	if e = writeJSON(filepath.Join(".obj", "media_probe.json"), map[string]any{"shapes": shapes, "fetch": results, "thread_id_prefix": threadIDPrefix(threadID), "network": hitCopy, "binary_fetch": binaryFetch, "page": pageShapes}); e != nil {
		t.Fatal(e)
	}
	t.Logf("attachments=%d shapes=%d fetch=%d network=%d", len(ids), len(shapes), len(results), len(hitCopy))
}

func threadIDPrefix(threadID string) string {
	if len(threadID) > 2 {
		return threadID[:2]
	}
	return threadID
}
