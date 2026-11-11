package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestCookieScope(t *testing.T) {
	for _, d := range []string{".google.com", "google.com", "voice.google.com"} {
		if !googleDomain(d) {
			t.Fatalf("rejected %s", d)
		}
	}
	for _, d := range []string{"evilgoogle.com", "google.com.evil", ".youtube.com", ""} {
		if googleDomain(d) {
			t.Fatalf("accepted %s", d)
		}
	}
	rows := cookieParams([]*network.Cookie{{Name: "valid", Domain: ".google.com", Session: true}, {Name: "expired", Domain: ".google.com", Expires: 1}, {Name: "other", Domain: "example.com", Session: true}, {Name: "opaque", Domain: ".google.com", Session: true, PartitionKeyOpaque: true}})
	if len(rows) != 1 || rows[0].Name != "valid" {
		t.Fatal("cookie scope or expiration filter failed")
	}
}

func TestProfileSnapshotWithWAL(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source", "Default")
	dst := filepath.Join(root, "target")
	if e := os.MkdirAll(src, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(dst, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(filepath.Dir(src), "Local State"), []byte(`{}`), 0600); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(src, "Cookies"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE cookies(host_key TEXT, value TEXT); INSERT INTO cookies VALUES('.google.com','first'),('voice.google.com','second'),('example.com','private'),('evilgoogle.com','private')`); e != nil {
		t.Fatal(e)
	}
	if e := copySourceProfile(context.Background(), src, dst); e != nil {
		t.Fatal(e)
	}
	copied, e := sql.Open("sqlite", filepath.Join(dst, "Default", "Cookies"))
	if e != nil {
		t.Fatal(e)
	}
	defer copied.Close()
	var n int
	if e = copied.QueryRow("SELECT count(*) FROM cookies").Scan(&n); e != nil || n != 2 {
		t.Fatalf("snapshot count=%d err=%v", n, e)
	}
	if e = db.QueryRow("SELECT count(*) FROM cookies").Scan(&n); e != nil || n != 4 {
		t.Fatal("source was changed")
	}
	st, e := os.Stat(filepath.Join(dst, "Default", "Cookies"))
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("snapshot permissions")
	}
}

func TestLiveAuth(t *testing.T) {
	if os.Getenv("GV_LIVE_AUTH") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	b, e := launchBrowser(ctx, home, c, func(profile string) error { return copySourceProfile(ctx, c.CookieSource, profile) })
	if e != nil {
		t.Fatal("source launch:", e)
	}
	defer b.close()
	var number string
	if e = b.run(chromedp.Navigate(b.base+"settings"), chromedp.Poll(accountNumberJS, &number, chromedp.WithPollingTimeout(25*time.Second))); e != nil {
		t.Fatal("settings probe:", e)
	}
	if number != c.Number {
		t.Fatal("configured sender not matched")
	}
	if e = b.saveSession(home); e != nil {
		t.Fatal("save:", e)
	}
	t.Log("verified configured sender; saved private session")
}

func TestLiveCachedAuthWithoutDesktop(t *testing.T) {
	if os.Getenv("GV_LIVE_CACHED_AUTH") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	c.CookieSource = filepath.Join(home, "deliberately_unavailable_source")
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if e = b.verify(); e != nil {
		t.Fatal(e)
	}
	t.Log("saved session verified without desktop, keyring access, or source profile")
}

func TestAuthPauseAndRecovery(t *testing.T) {
	now := time.Now()
	r := authRetry{interval: 5 * time.Minute}
	calls := 0
	fail := func() (*Browser, error) { calls++; return nil, errAuthPaused }
	if _, e := r.connect(now, fail); e == nil || calls != 1 {
		t.Fatal("failed authentication did not pause")
	}
	if _, e := r.connect(now.Add(time.Minute), fail); e == nil || calls != 1 {
		t.Fatal("retried before deadline")
	}
	expected := &Browser{}
	b, e := r.connect(now.Add(5*time.Minute), func() (*Browser, error) { calls++; return expected, nil })
	if e != nil || b != expected || calls != 2 || !r.next.IsZero() {
		t.Fatal("refreshed session did not recover")
	}
}

func TestLiveAuthRecovery(t *testing.T) {
	if os.Getenv("GV_LIVE_RECOVERY") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, e := readConfig(os.Getenv("SMSGATE_GV_HOME"))
	if e != nil {
		t.Fatal(e)
	}
	original := c.CookieSource
	home := t.TempDir()
	sourceRoot := filepath.Join(home, "source")
	if e = os.Mkdir(sourceRoot, 0700); e != nil {
		t.Fatal(e)
	}
	c.CookieSource = filepath.Join(sourceRoot, "Default")
	retry := authRetry{interval: 5 * time.Minute}
	at := time.Now()
	connect := func() (*Browser, error) { return startBrowser(ctx, home, c) }
	if b, e := retry.connect(at, connect); e == nil || b != nil {
		t.Fatal("missing source did not pause")
	}
	// Simulate the user making refreshed browser cookies available at the same
	// configured path. Neither the bridge nor its recovery state is restarted.
	if e = copySourceProfile(ctx, original, sourceRoot); e != nil {
		t.Fatal(e)
	}
	b, e := retry.connect(at.Add(5*time.Minute), connect)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if e = b.verify(); e != nil {
		t.Fatal(e)
	}
	t.Log("paused with unavailable cookies; recovered automatically when source became available")
}

func TestPausedServiceRejectsNewSend(t *testing.T) {
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s := &Bridge{config: Config{Username: "test", Password: "private"}, store: store, authState: "paused", jobs: make(chan string, 1)}
	req := httptest.NewRequest("POST", "/message", strings.NewReader(`{"id":"paused-test","textMessage":{"text":"must not send"},"phoneNumbers":["+15555550123"]}`))
	req.SetBasicAuth("test", "private")
	response := httptest.NewRecorder()
	s.ServeHTTP(response, req)
	if response.Code != 503 {
		t.Fatalf("expected paused rejection, got %d", response.Code)
	}
	if len(s.jobs) != 0 {
		t.Fatal("paused service queued a send")
	}
	if _, ok := store.get("paused-test"); ok {
		t.Fatal("paused service reserved a send")
	}
	req = httptest.NewRequest("GET", "/health", nil)
	req.SetBasicAuth("test", "private")
	response = httptest.NewRecorder()
	s.ServeHTTP(response, req)
	var health struct {
		Authentication string `json:"authentication"`
		Ready          bool   `json:"ready"`
	}
	if e = json.Unmarshal(response.Body.Bytes(), &health); e != nil {
		t.Fatal(e)
	}
	if response.Code != 503 || health.Ready || health.Authentication != "paused" {
		t.Fatal("health did not disclose paused authentication")
	}
}

func TestAuthCheckCancelsStuckBrowser(t *testing.T) {
	home := t.TempDir()
	fakeBrowser := filepath.Join(home, "stuck-browser")
	if e := os.WriteFile(fakeBrowser, []byte("#!/bin/sh\nexec sleep 30\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := writeJSON(filepath.Join(home, "session.json"), []*network.Cookie{{Name: "fixture", Value: "fixture", Domain: ".google.com", Path: "/", Session: true}}); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	e := checkAuthentication(ctx, home, Config{Browser: fakeBrowser}, func(string) {})
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", e)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("stuck browser cancellation took %s", elapsed)
	}
	profiles, e := filepath.Glob(filepath.Join(home, "browser-*"))
	if e != nil || len(profiles) != 0 {
		t.Fatal("temporary profile left after cancellation")
	}
}
