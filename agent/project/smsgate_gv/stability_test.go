package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCleanupScratchRemovesOnlyStaleBrowserProfiles(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"browser-old", "browser-newer"} {
		dir := filepath.Join(home, name)
		if e := os.MkdirAll(filepath.Join(dir, "Default"), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "Default", "Cookies"), []byte("private"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"profile", "other"} {
		if e := os.MkdirAll(filepath.Join(home, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(home, "browser-not-a-directory"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	removed, e := cleanupScratch(home)
	if e != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, e)
	}
	for _, name := range []string{"browser-old", "browser-newer"} {
		if _, e := os.Stat(filepath.Join(home, name)); !os.IsNotExist(e) {
			t.Fatalf("%s was not removed", name)
		}
	}
	for _, name := range []string{"profile", "other", "browser-not-a-directory"} {
		if _, e := os.Stat(filepath.Join(home, name)); e != nil {
			t.Fatalf("%s was removed: %v", name, e)
		}
	}
}

func TestCleanupScratchMissingHome(t *testing.T) {
	removed, e := cleanupScratch(filepath.Join(t.TempDir(), "missing"))
	if e != nil || removed != 0 {
		t.Fatalf("removed=%d err=%v", removed, e)
	}
	if e := removeScratch(filepath.Join(t.TempDir(), "already-gone")); e != nil {
		t.Fatalf("removeScratch: %v", e)
	}
}

func TestHistoryConfigDefaultsAndValidation(t *testing.T) {
	home := t.TempDir()
	base := map[string]any{
		"number": "+15555550000", "username": "user", "password": "private",
		"poll_seconds": 300, "max_threads": 1000,
	}
	if e := writeJSON(filepath.Join(home, "config.json"), base); e != nil {
		t.Fatal(e)
	}
	c, e := readConfig(home)
	if e != nil || c.HistoryMessages != 1000 {
		t.Fatalf("history default=%d err=%v", c.HistoryMessages, e)
	}
	base["history_messages"] = 1001
	if e = writeJSON(filepath.Join(home, "config.json"), base); e != nil {
		t.Fatal(e)
	}
	if _, e = readConfig(home); e == nil {
		t.Fatal("history_messages above 1000 accepted")
	}
}

func TestColdSyncRequestUsesConfiguredLimits(t *testing.T) {
	request := coldSyncRequest(Config{MaxThreads: 154, HistoryMessages: 217})
	views := request[0].([]any)
	limit := views[1].([]int)
	if limit[0] != 2 || limit[1] != 154 || limit[2] != 217 {
		t.Fatalf("cold sync limits=%v", limit)
	}
	defaults := coldSyncRequest(Config{})
	limit = defaults[0].([]any)[1].([]int)
	if limit[1] != 1000 || limit[2] != 1000 {
		t.Fatalf("cold sync defaults=%v", limit)
	}
}

func TestHealthReportsImplementedCapabilities(t *testing.T) {
	s := &Bridge{config: Config{Username: "user", Password: "private"}, authState: "checking"}
	req := httptest.NewRequest("GET", "/health", nil)
	req.SetBasicAuth("user", "private")
	r := httptest.NewRecorder()
	s.ServeHTTP(r, req)
	if r.Code != 503 {
		t.Fatalf("unready health status=%d", r.Code)
	}
	var body struct {
		Backend        string   `json:"backend"`
		Capabilities   []string `json:"capabilities"`
		Authentication string   `json:"authentication"`
	}
	if e := json.Unmarshal(r.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	if body.Backend != "google_voice" || body.Authentication != "checking" || !reflect.DeepEqual(body.Capabilities, bridgeCapabilities) {
		t.Fatalf("unexpected health body: %+v", body)
	}
	for _, capability := range body.Capabilities {
		if capability == "group-send" || capability == "mms-send" {
			t.Fatalf("unimplemented capability advertised: %s", capability)
		}
	}
}
