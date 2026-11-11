package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Read-only discovery of cold-sync limits. Writes only shapes and counts, never
// message text or identifiers.
func TestLiveColdSyncLimits(t *testing.T) {
	if os.Getenv("GV_LIVE_PAGINATION") != "1" {
		t.Skip("explicit opt-in; read only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	c.SyncNumbers = nil
	c.MaxThreads = 10000
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	request := func(threads, messages int) []any {
		return []any{[]any{[]int{1, 0, 0}, []int{2, threads, messages}, []int{3, 0, 0}, []int{4, 0, 0}, []int{5, 0, 0}, []int{6, 0, 0}, []int{7, 0, 0}, []int{8, 0, 0}, []int{9, 0, 0}, []int{10, 0, 0}}, []int{1, 1, 1, 1, 0}}
	}
	type result struct {
		Threads  int            `json:"threads"`
		Messages int            `json:"messages"`
		PerPeer  map[string]int `json:"per_peer"`
		Bytes    int            `json:"bytes"`
		Token    bool           `json:"token"`
	}
	out := map[string]result{}
	for _, limits := range [][2]int{{1000, 1000}, {2000, 2000}, {5000, 5000}} {
		body, e := b.voiceEndpoint(coldSyncPath, request(limits[0], limits[1]))
		if e != nil {
			t.Fatal(e)
		}
		rows, token, e := decodeColdSync(body, c)
		if e != nil {
			t.Fatal(e)
		}
		perPeer := map[string]int{}
		threads := map[string]bool{}
		for _, m := range rows {
			peer := m.Sender
			if peer == "" && len(m.Recipients) > 0 {
				peer = m.Recipients[0].Number
			}
			perPeer[peer]++
			threads[peer] = true
		}
		out[fmtLimits(limits)] = result{Threads: len(threads), Messages: len(rows), PerPeer: perPeer, Bytes: len(body), Token: token != ""}
	}
	keys := []string{}
	for key := range out {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if e = writeJSON(filepath.Join(".obj", "pagination_probe.json"), map[string]any{"results": out, "limits": keys}); e != nil {
		t.Fatal(e)
	}
	for _, key := range keys {
		r := out[key]
		t.Logf("%s threads=%d messages=%d bytes=%d token=%v", key, r.Threads, r.Messages, r.Bytes, r.Token)
	}
}

func fmtLimits(v [2]int) string {
	return fmt.Sprintf("%dx%d", v[0], v[1])
}
