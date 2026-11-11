package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"

	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestLiveEndpointRefresh(t *testing.T) {
	if os.Getenv("GV_LIVE_ENDPOINT") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	c.SyncNumbers = []string{os.Getenv("GV_REMOTE_NUMBER")}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	e = b.sync(store)
	if e != nil {
		t.Fatal(e)
	}
	if b.historyRequest == nil {
		t.Fatal("request structure not captured")
	}
	u, _ := url.Parse(b.historyRequest.URL)
	keys := []string{}
	for k := range u.Query() {
		keys = append(keys, k)
	}
	// Method/body/filter structure only; never retain credentials or API key values.
	if e = writeJSON(filepath.Join(".obj", "history_request_shape.json"), map[string]any{"method": b.historyRequest.Method, "query_keys": keys, "body": b.historyPostBody}); e != nil {
		t.Fatal(e)
	}
	var gapi bool
	if e = b.run(chromedp.Evaluate(`typeof gapi?.client?.request==='function'`, &gapi)); e != nil || !gapi {
		t.Fatal("gapi unavailable", e)
	}
	var documents atomic.Int64
	var endpointBytes atomic.Int64
	var requests atomic.Int64
	ids := make(map[network.RequestID]bool)
	var mu sync.Mutex
	stopCtx, stop := context.WithCancel(b.ctx)
	defer stop()
	chromedp.ListenTarget(stopCtx, func(ev any) {
		mu.Lock()
		defer mu.Unlock()
		switch event := ev.(type) {
		case *network.EventRequestWillBeSent:
			if event.Type == network.ResourceTypeDocument {
				documents.Add(1)
			}
			u, err := url.Parse(event.Request.URL)
			if err == nil && u.Path == warmSyncPath {
				ids[event.RequestID] = true
				requests.Add(1)
			}
		case *network.EventLoadingFinished:
			if ids[event.RequestID] {
				endpointBytes.Add(int64(event.EncodedDataLength))
				delete(ids, event.RequestID)
			}
		}
	})
	second, e := b.readNetworkHistory()
	if e != nil {
		t.Fatal(e)
	}
	if len(second) != 0 || documents.Load() != 0 {
		t.Fatal("endpoint refresh changed history or reloaded document")
	}
	if e = writeJSON(filepath.Join(".obj", "endpoint_refresh_results.json"), map[string]any{"checkedAt": time.Now().UTC().Format(time.RFC3339), "messages": len(second), "document_requests": documents.Load(), "history_requests": requests.Load(), "encoded_response_bytes": endpointBytes.Load(), "empty_delta": true, "stored_messages": len(store.list(true)) + len(store.list(false))}); e != nil {
		t.Fatal(e)
	}
	t.Logf("%d changed messages; %d incremental requests, %d encoded response bytes, zero document reloads", len(second), requests.Load(), endpointBytes.Load())
}
