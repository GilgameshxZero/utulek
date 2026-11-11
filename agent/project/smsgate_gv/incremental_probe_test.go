package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func payloadShape(v any, depth int) any {
	if depth > 2 {
		switch x := v.(type) {
		case []any:
			return map[string]any{"array": len(x)}
		case map[string]any:
			return map[string]any{"object": len(x)}
		case string:
			return "string"
		default:
			return v
		}
	}
	switch x := v.(type) {
	case []any:
		out := []any{}
		for i, z := range x {
			if i >= 5 {
				break
			}
			out = append(out, payloadShape(z, depth+1))
		}
		return map[string]any{"length": len(x), "items": out}
	case map[string]any:
		out := map[string]any{}
		for k, z := range x {
			out[k] = payloadShape(z, depth+1)
		}
		return out
	case string:
		return "string"
	default:
		return v
	}
}
func targetMessages(v any, number string) []any {
	out := []any{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			if len(x) > 15 {
				id, _ := x[0].(string)
				stamp, _ := x[1].(float64)
				peer, _ := x[15].(string)
				if len(id) >= 30 && stamp > 1e12 && peer == number {
					out = append(out, x)
					return
				}
			}
			for _, z := range x {
				walk(z)
			}
		case map[string]any:
			for _, z := range x {
				walk(z)
			}
		}
	}
	walk(v)
	return out
}

func TestLiveIncrementalDiscovery(t *testing.T) {
	if os.Getenv("GV_LIVE_INCREMENTAL_DISCOVERY") != "1" {
		t.Skip("explicit opt-in; read only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	number := os.Getenv("GV_REMOTE_NUMBER")
	if !phonePattern.MatchString(number) {
		t.Fatal("explicit remote required")
	}
	c.SyncNumbers = []string{number}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	type event struct {
		ID   network.RequestID
		Path string
	}
	var mu sync.Mutex
	var events []event
	chromedp.ListenTarget(b.ctx, func(ev any) {
		if r, ok := ev.(*network.EventResponseReceived); ok {
			u, e := url.Parse(r.Response.URL)
			if e == nil && strings.Contains(u.Path, "/voice/v1/voiceclient/") && r.Type == network.ResourceTypeXHR {
				mu.Lock()
				events = append(events, event{r.RequestID, u.Path})
				mu.Unlock()
			}
		}
	})
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	// Wait only for the normal app bootstrap's own inbox sync, not an idle scan.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := false
		for _, r := range events {
			if strings.Contains(r.Path, "/inbox/") {
				found = true
			}
		}
		mu.Unlock()
		if found {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	mu.Lock()
	captured := append([]event(nil), events...)
	mu.Unlock()
	output := []map[string]any{}
	for _, r := range captured {
		if !strings.Contains(r.Path, "/inbox/") && !strings.Contains(r.Path, "threadinginfo") {
			continue
		}
		var body []byte
		var post string
		_ = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var e error
			post, e = network.GetRequestPostData(r.ID).Do(ctx)
			return e
		}))
		if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var e error
			body, e = network.GetResponseBody(r.ID).Do(ctx)
			return e
		})); e != nil {
			continue
		}
		var v any
		if e = json.Unmarshal(body, &v); e != nil {
			continue
		}
		output = append(output, map[string]any{"path": r.Path, "request": post, "response_shape": payloadShape(v, 0), "target_messages": targetMessages(v, number)})
	}
	var scripts []string
	if e = b.run(chromedp.Evaluate(`[...document.scripts].map(s=>s.src).filter(u=>u.startsWith('https://www.gstatic.com/_/voice/'))`, &scripts)); e != nil {
		t.Fatal(e)
	}
	if e = writeJSON(filepath.Join(".obj", "incremental_discovery.json"), map[string]any{"responses": output, "static_scripts": scripts}); e != nil {
		t.Fatal(e)
	}
	t.Logf("inspected %d sync response structures; retained only target messages", len(output))
}

func TestLiveWarmSyncProbe(t *testing.T) {
	if os.Getenv("GV_LIVE_WARM_PROBE") != "1" {
		t.Skip("explicit opt-in; read only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	number := os.Getenv("GV_REMOTE_NUMBER")
	c.SyncNumbers = []string{number}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	coldRequest := []any{[]any{[]int{1, 0, 0}, []int{2, 100, 50}, []int{3, 0, 0}, []int{4, 0, 0}, []int{5, 0, 0}, []int{6, 0, 0}, []int{7, 0, 0}, []int{8, 0, 0}, []int{9, 0, 0}, []int{10, 0, 0}}, []int{1, 1, 1, 1, 0}}
	body, e := b.voiceEndpoint("/voice/v1/voiceclient/inbox/coldsync", coldRequest)
	if e != nil {
		t.Fatal(e)
	}
	var response []json.RawMessage
	var token string
	if e = json.Unmarshal(body, &response); e != nil || len(response) < 2 || json.Unmarshal(response[1], &token) != nil || token == "" {
		t.Fatal("no cold sync cursor")
	}
	warmBody, e := b.voiceEndpoint("/voice/v1/voiceclient/inbox/warmsync", []any{100, token, []int{1, 1, 1, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	warm := struct {
		Status int
		Body   string
	}{200, string(warmBody)}
	var v any
	if e = json.Unmarshal([]byte(warm.Body), &v); e != nil {
		t.Fatal("warm response", warm.Status, e)
	}
	if e = writeJSON(filepath.Join(".obj", "warm_sync_probe.json"), map[string]any{"status": warm.Status, "body_bytes": len(warm.Body), "shape": payloadShape(v, 0), "target_messages": targetMessages(v, number)}); e != nil {
		t.Fatal(e)
	}
	if warm.Status != 200 {
		t.Fatal("warm sync HTTP", warm.Status)
	}
	t.Logf("cursor-based warm sync HTTP %d; response %d bytes", warm.Status, len(warm.Body))
}
