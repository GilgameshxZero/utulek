package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWarmSyncMergeAndCursorBoundary(t *testing.T) {
	account, number := "+15555550000", "+15555550123"
	item := make([]any, 30)
	item[0] = "native-id"
	item[1] = int64(1793545877539)
	item[2] = account
	item[3] = []any{number}
	item[4] = 10
	item[9] = "delta text"
	item[29] = "t." + number
	thread := make([]any, 15)
	thread[0] = "t." + number
	thread[1] = 1
	thread[14] = item
	body, _ := json.Marshal([]any{[]any{[]any{thread}, []any{item}, nil, "cursor-next", 0}})
	rows, token, more, e := decodeWarmSync(body, Config{Number: account, MaxThreads: 1})
	if e != nil || len(rows) != 1 || token != "cursor-next" || more || rows[0].Preview != "delta text" {
		t.Fatal("delta/last-item dedup failed", len(rows), e)
	}
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	b := &Browser{config: Config{Number: account}, cursor: "cursor-before", pendingCursor: token, updates: make(chan struct{}, 1)}
	if e = store.mergeImports(rows); e != nil {
		t.Fatal(e)
	}
	if e = b.commitCursor(store); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(filepath.Dir(store.path), "sync_cursor.json"))
	if e != nil {
		t.Fatal(e)
	}
	var saved syncCursor
	if json.Unmarshal(data, &saved) != nil || saved.Token != token || !b.watchReady.Load() {
		t.Fatal("cursor not committed")
	}
	b.pendingCursor = "must-not-advance"
	store.path = filepath.Join(t.TempDir(), "missing", "messages.json")
	if e = b.commitCursor(store); e == nil || b.cursor != token {
		t.Fatal("failed cursor write advanced state")
	}
	if _, _, _, e = decodeWarmSync([]byte(`[null,[1]]`), Config{}); e != errSyncReset {
		t.Fatal("cursor reset not recognized")
	}
	if _, _, _, e = decodeWarmSync([]byte(`[[null,null,null,""]]`), Config{}); e == nil {
		t.Fatal("empty cursor accepted")
	}
}

func TestSubscribersOnlyReceiveCommittedChanges(t *testing.T) {
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	ch, unsubscribe := store.subscribe()
	defer unsubscribe()
	m := Message{ID: "id", Body: &Text{Text: "original"}}
	if e = store.put(m); e != nil {
		t.Fatal(e)
	}
	m.Body.Text = "mutated after commit"
	rows := <-ch
	if rows[0].Body.Text != "original" {
		t.Fatal("published mutable state")
	}
	if e = store.mergeImports([]Message{rows[0]}); e != nil {
		t.Fatal(e)
	}
	select {
	case <-ch:
		t.Fatal("unchanged record generated notification")
	default:
	}
	store.path = filepath.Join(t.TempDir(), "missing", "messages.json")
	if e = store.put(m); e == nil {
		t.Fatal("failed write ignored")
	}
	select {
	case <-ch:
		t.Fatal("uncommitted write generated notification")
	default:
	}
}

func TestShortCodeHistoryColdAndWarm(t *testing.T) {
	account := "+15555550000"
	item := make([]any, 30)
	item[0], item[1], item[2], item[4], item[9], item[29] = "short-code-native", int64(1793545877539), account, 10, "new short code message", "t.72524"
	thread := []any{"t.72524", 1, []any{item}}
	cold, _ := json.Marshal([]any{[]any{[]any{2, nil, []any{thread}}}, "cold-token"})
	warm, _ := json.Marshal([]any{[]any{nil, []any{item}, nil, "warm-token", 0}})
	c := Config{Number: account, MaxThreads: 30}
	rows, _, err := decodeColdSync(cold, c)
	if err != nil || len(rows) != 1 || rows[0].Sender != "72524" {
		t.Fatalf("cold short code missing: rows=%v err=%v", rows, err)
	}
	delta, _, _, err := decodeWarmSync(warm, c)
	if err != nil || len(delta) != 1 || delta[0].ID != rows[0].ID || delta[0].Preview != rows[0].Preview {
		t.Fatalf("warm short code missing: rows=%v err=%v", delta, err)
	}
	c.SyncNumbers = []string{"+15555550123"}
	rows, _, err = decodeColdSync(cold, c)
	if err != nil || len(rows) != 0 {
		t.Fatal("short code bypassed explicit scope", err)
	}
	for _, invalid := range []string{"", "12", "1234567", "abcde", "group.72524"} {
		if historyPeer(invalid) {
			t.Fatalf("invalid peer %q accepted", invalid)
		}
	}
}
