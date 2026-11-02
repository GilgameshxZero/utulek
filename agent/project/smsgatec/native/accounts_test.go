package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAccountSwitchIsolatesHistoryAndRestoresList(t *testing.T) {
	c := Config{Theme: "auto", Poll: 60, Timeout: 3, UpdateMode: "push", Reconcile: 3600, Cache: true, CacheDir: t.TempDir(), Accounts: []Account{{"phone", "Android", "http://127.0.0.1:1", "u", "p"}, {"gv", "Google Voice", "http://127.0.0.1:2", "u", "p"}}}
	c, _ = c.account("phone")
	path := filepath.Join(t.TempDir(), "config.json")
	a, err := newApp(c, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	if err = a.Cache.put([]Message{{ID: "same", Number: "+12025550123", Direction: "in", Text: "phone only"}}); err != nil {
		t.Fatal(err)
	}
	a.key('y', 24)
	if a.View != "accounts" {
		t.Fatal(a.View)
	}
	a.key(27, 24)
	if a.View != "index" {
		t.Fatal("cancel lost list")
	}
	a.key('y', 24)
	a.key('j', 24)
	a.key(keyEnter, 24)
	if a.Config.ActiveAccount != "gv" || a.View != "index" || len(a.Cache.records) != 0 {
		t.Fatal("account state leaked")
	}
	loaded, err := loadConfig(path)
	if err != nil || loaded.ActiveAccount != "gv" || loaded.Server != c.Accounts[1].Server {
		t.Fatal(loaded.ActiveAccount, err)
	}
	b, _ := os.ReadFile(path)
	var saved map[string]any
	json.Unmarshal(b, &saved)
	if saved["password"] != "" {
		t.Fatal("flat credentials retained")
	}
}
func TestPushOutageDoesNotTriggerFallbackHistory(t *testing.T) {
	c := Config{Theme: "auto", Poll: 3, Timeout: 3, UpdateMode: "push", Reconcile: 3600, Server: "http://127.0.0.1:1", Username: "u", Password: "p"}
	a, err := newApp(c, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	if a.pushRetryInterval() != 300*time.Second {
		t.Fatal("incorrect default push retry")
	}
	a.LastSync = time.Now().Add(-10 * time.Minute)
	a.PushAttempt = time.Now()
	a.tick()
	if a.Jobs["sync"] {
		t.Fatal("push outage triggered polling")
	}
	a.PushAttempt = time.Now().Add(-6 * time.Minute)
	a.tick()
	if !a.Jobs["push"] {
		t.Fatal("push connection not retried")
	}
}
