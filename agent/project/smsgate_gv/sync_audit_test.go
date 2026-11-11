package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This opt-in audit compares a fresh snapshot to the live cache without changing
// the live message journal or cursor, sending messages, or logging their content.
func TestLiveSyncAudit(t *testing.T) {
	if os.Getenv("GV_LIVE_SYNC_AUDIT") != "1" {
		t.Skip("explicit read-only opt-in")
	}
	home := os.Getenv("SMSGATE_GV_HOME")
	c, err := readConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, "messages.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cached map[string]Message
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, err := startBrowser(ctx, home, c)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	scratch, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.sync(scratch); err != nil {
		t.Fatal(err)
	}
	fresh := append(scratch.list(true), scratch.list(false)...)
	result := map[string]any{"checked_at": time.Now().UTC().Format(time.RFC3339), "cached_records": len(cached)}
	summarize := func(name string, rows []Message) {
		missing := 0
		newest := ""
		for _, row := range rows {
			if _, ok := cached[row.ID]; !ok {
				missing++
			}
			if row.Created > newest {
				newest = row.Created
			}
		}
		result[name] = map[string]any{"records": len(rows), "missing_from_cache": missing, "newest": newest}
		t.Logf("%s records=%d missing_from_cache=%d newest=%s", name, len(rows), missing, newest)
	}
	summarize("cold_snapshot", fresh)
	rows, err := b.refreshHistoryEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	summarize("thread_list", rows)
	if err := writeJSON(filepath.Join(".obj", "sync_audit_results.json"), result); err != nil {
		t.Fatal(err)
	}
}
