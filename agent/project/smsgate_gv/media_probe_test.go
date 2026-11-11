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
						if len(mimeAt) > 0 {
							if value, ok := att[mimeAt[0]].(string); ok {
								shape["mime"] = value
							}
						}
						if len(att) > 2 {
							if value, ok := att[2].(float64); ok {
								shape["size"] = int64(value)
							}
						}
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
								partsShapes := []any{}
								for i := 0; i < len(parts) && i < 8; i++ {
									partsShapes = append(partsShapes, partShape(parts[i]))
								}
								shape["parts"] = partsShapes
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
	var dom []map[string]any
	if threadID != "" {
		_ = b.run(network.Enable(), chromedp.Navigate(b.base+"messages"))
		time.Sleep(6 * time.Second)
		var clicked struct {
			Found    bool `json:"found"`
			Scrolled int  `json:"scrolled"`
		}
		findJS := `(async () => {
			const key = ` + literal(threadID) + `;
			const sel = '[data-store-key="' + CSS.escape(key) + '"]';
			let el = document.querySelector(sel);
			if (el) { const inner = el.querySelector('a,[role="button"],button') || el; inner.click(); return {found:true, scrolled:0}; }
			const item = document.querySelector('li[data-store-key]');
			let box = item;
			while (box && !(box.scrollHeight > box.clientHeight + 50)) box = box.parentElement;
			let n = 0;
			for (; box && n < 40 && !el; n++) {
				box.scrollTop = box.scrollHeight;
				await new Promise(r => setTimeout(r, 600));
				el = document.querySelector(sel);
			}
			if (el) { const inner = el.querySelector('a,[role="button"],button') || el; inner.click(); return {found:true, scrolled:n}; }
			return {found:false, scrolled:n};
		})()`
		_ = b.run(chromedp.Evaluate(findJS, &clicked, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
		_ = b.run(chromedp.Evaluate(`(() => {
			const out = [];
			for (const e of document.querySelectorAll('[role="listitem"],[role="row"],gv-thread-item,gv-conversation-item,li')) {
				out.push({tag:e.tagName, role:e.getAttribute('role')||'', cls:String(e.className||'').slice(0,80), key:e.getAttribute('data-store-key')||'', attrs:[...e.attributes].map(a=>a.name+':'+a.value.length), textLen:(e.innerText||'').length});
				if (out.length >= 8) break;
			}
			return out;
		})()`, &dom))
		time.Sleep(14 * time.Second)
		_ = b.run(chromedp.Evaluate(`(() => { const s=[...document.querySelectorAll('*')].find(e=>e.scrollHeight>e.clientHeight+100 && /message|thread|conversation/i.test(e.className)); if(s){s.scrollTop=s.scrollHeight;} window.scrollTo(0,document.body.scrollHeight); return true; })()`, nil))
		time.Sleep(6 * time.Second)
		t.Logf("clicked found=%v scrolled=%d", clicked.Found, clicked.Scrolled)
	}
	var binaryFetch fetchResult
	var page struct {
		URL      string `json:"url"`
		Title    string `json:"title"`
		Messages int    `json:"messages"`
		Composer bool   `json:"composer"`
		Images   []struct {
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
		const messages = document.querySelectorAll('[data-message-id],gv-message-item,.message-item,.message').length;
		return {url: location.href, title: document.title, messages, composer: !!document.querySelector('textarea,[contenteditable="true"]'), images: imgs.slice(0, 20), media: media.slice(0, 8)};
	})()`, &page))
	pageShapes := map[string]any{"url": page.URL, "title": page.Title, "messages": page.Messages, "composer": page.Composer, "images": []map[string]any{}, "media": []string{}}
	var loadedMediaURL string
	for _, image := range page.Images {
		if u, e := url.Parse(image.Src); e == nil && u.Host != "" {
			entry := map[string]any{"host": u.Host, "path": u.Path[:min(40, len(u.Path))], "width": image.NaturalW, "height": image.NaturalH, "complete": image.Complete}
			pageShapes["images"] = append(pageShapes["images"].([]map[string]any), entry)
			if loadedMediaURL == "" && image.NaturalW > 0 && (strings.Contains(u.Path, "/media/") || strings.Contains(u.Path, "/attachment") || strings.Contains(u.Path, "/a/i/")) {
				loadedMediaURL = image.Src
			}
		}
	}
	for _, value := range page.Media {
		if u, e := url.Parse(value); e == nil && u.Host != "" {
			pageShapes["media"] = append(pageShapes["media"].([]string), u.Host+u.Path[:min(40, len(u.Path))])
		}
	}
	var lightboxImages []map[string]any
	var lightboxOpened bool
	_ = b.run(chromedp.Evaluate(`(() => { const img = [...document.images].find(i => (i.currentSrc || i.src).includes('/a/i/')); if (!img) return false; img.click(); return true; })()`, &lightboxOpened))
	time.Sleep(5 * time.Second)
	_ = b.run(chromedp.Evaluate(`(() => [...document.images].map(i => ({src: i.currentSrc || i.src, w: i.naturalWidth, h: i.naturalHeight})).filter(i => i.src.includes('/a/i/')).slice(0,8))()`, &lightboxImages))
	pageShapes["lightbox_opened"] = lightboxOpened
	pageShapes["lightbox"] = lightboxImages
	if loadedMediaURL != "" {
		base := strings.Split(loadedMediaURL, "?")[0]
		variants := []string{base}
		for _, size := range []string{"1", "2", "3", "4"} {
			variants = append(variants, base+"?s="+size)
		}
		for _, kind := range []string{"/a/d/", "/a/v/", "/a/a/", "/a/f/"} {
			variants = append(variants, strings.Replace(base, "/a/i/", kind, 1))
		}
		loaded := []fetchResult{}
		for _, variant := range variants {
			js := `(async()=>{try{const r=await fetch(` + literal(variant) + `,{credentials:'include',headers:{Accept:'image/*,*/*'}});const b=await r.arrayBuffer();return {status:r.status,type:r.headers.get('content-type')||'',len:b.byteLength}}catch(e){return {status:-1,type:String(e),len:0}}})()`
			var response struct {
				Status int    `json:"status"`
				Type   string `json:"type"`
				Len    int    `json:"len"`
			}
			_ = b.run(chromedp.Evaluate(js, &response, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
			loaded = append(loaded, fetchResult{URL: variant, Status: response.Status, ContentType: response.Type, Bytes: response.Len})
		}
		binaryFetch = loaded[0]
		pageShapes["loaded_variants"] = loaded
	}
	mu.Lock()
	hitCopy := append([]mediaHit(nil), hits...)
	mu.Unlock()
	if e = writeJSON(filepath.Join(".obj", "media_probe.json"), map[string]any{"shapes": shapes, "fetch": results, "thread_id_prefix": threadIDPrefix(threadID), "network": hitCopy, "binary_fetch": binaryFetch, "page": pageShapes, "dom": dom}); e != nil {
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

func partShape(value any) any {
	switch x := value.(type) {
	case []any:
		out := []any{}
		for i, child := range x {
			if i >= 6 {
				break
			}
			out = append(out, partShape(child))
		}
		return map[string]any{"len": len(x), "items": out}
	case string:
		return fmt.Sprintf("string:%d", len(x))
	case float64:
		return x
	default:
		return fmt.Sprintf("%T", x)
	}
}
