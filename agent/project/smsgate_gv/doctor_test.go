package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDoctorFixture(t *testing.T, home string) Config {
	t.Helper()
	c := Config{Number: "+15555550000", Username: "u", Password: "p", Listen: "127.0.0.1:1", Browser: "/bin/sh", CacheLimitMB: 1}
	files := map[string]string{
		"session.json":     `[{"name":"SID"}]`,
		"messages.json":    `{"gv-1":{"id":"gv-1","state":"Received"}}`,
		"sync_cursor.json": `{"version":` + itoa(syncCursorVersion) + `,"account":"+15555550000","token":"cursor"}`,
	}
	for name, content := range files {
		path := filepath.Join(home, name)
		if e := os.WriteFile(path, []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return c
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	out := ""
	for value > 0 {
		out = string(rune('0'+value%10)) + out
		value /= 10
	}
	return out
}

func doctorFailed(checks []Check) string {
	for _, check := range checks {
		if check.Status == "FAIL" {
			return check.Name + ": " + check.Detail
		}
	}
	return ""
}

func TestDoctorHealthyState(t *testing.T) {
	home := t.TempDir()
	c := writeDoctorFixture(t, home)
	checks := runDoctor(home, c)
	if failure := doctorFailed(checks); failure != "" {
		t.Fatalf("unexpected failure: %s", failure)
	}
	foundServiceWarning := false
	for _, check := range checks {
		if check.Name == "service" && check.Status == "WARN" {
			foundServiceWarning = true
		}
	}
	if !foundServiceWarning {
		t.Fatal("stopped service was not reported")
	}
}

func TestDoctorDetectsCorruptJournal(t *testing.T) {
	home := t.TempDir()
	c := writeDoctorFixture(t, home)
	if e := os.WriteFile(filepath.Join(home, "messages.json"), []byte("not json"), 0600); e != nil {
		t.Fatal(e)
	}
	if failure := doctorFailed(runDoctor(home, c)); !strings.Contains(failure, "message journal") {
		t.Fatalf("corrupt journal not detected: %q", failure)
	}
}

func TestDoctorWarnsLoosePermissionsAndMissingSession(t *testing.T) {
	home := t.TempDir()
	c := writeDoctorFixture(t, home)
	if e := os.Chmod(home, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(home, "session.json")); e != nil {
		t.Fatal(e)
	}
	warnings := map[string]bool{}
	for _, check := range runDoctor(home, c) {
		if check.Status == "WARN" {
			warnings[check.Name] = true
		}
	}
	if !warnings["state directory"] || !warnings["saved session"] {
		t.Fatalf("warnings=%v", warnings)
	}
}
