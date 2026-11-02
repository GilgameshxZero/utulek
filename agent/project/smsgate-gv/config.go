package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
)

type Config struct {
	Listen           string   `json:"listen"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	Number           string   `json:"number"`
	Browser          string   `json:"browser"`
	Account          int      `json:"account"`
	PollSeconds      int      `json:"poll_seconds"`
	MaxThreads       int      `json:"max_threads"`
	CookieSource     string   `json:"cookie_source,omitempty"`
	AuthRetrySeconds int      `json:"auth_retry_seconds,omitempty"`
	SyncNumbers      []string `json:"sync_numbers,omitempty"`
}

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func homeDir() string {
	if p := os.Getenv("SMSGATE_GV_HOME"); p != "" {
		return p
	}
	p := os.Getenv("XDG_STATE_HOME")
	if p == "" {
		h, _ := os.UserHomeDir()
		p = filepath.Join(h, ".local", "state")
	}
	return filepath.Join(p, "smsgate-gv")
}

func randomID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

func privateDir(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return os.Chmod(path, 0700)
}

// Sync the file and directory before reporting a durable update.
func writeJSON(path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".write-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(append(b, '\n'))
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

func readConfig(home string) (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(home, "config.json"))
	if e != nil {
		return c, fmt.Errorf("run init first: %w", e)
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	for _, number := range c.SyncNumbers {
		if !phonePattern.MatchString(number) {
			return c, errors.New("sync_numbers must contain international phone numbers")
		}
	}
	if c.AuthRetrySeconds == 0 {
		c.AuthRetrySeconds = 300
	}
	if c.AuthRetrySeconds < 5 {
		return c, errors.New("auth_retry_seconds must be at least 5")
	}
	if !phonePattern.MatchString(c.Number) || c.Username == "" || c.Password == "" || c.Account < 0 || c.PollSeconds < 5 || c.MaxThreads < 1 || c.MaxThreads > 1000 {
		return c, errors.New("invalid configuration: check number, credentials, account, poll_seconds (>=5), max_threads (1..1000)")
	}
	return c, nil
}

func findBrowser() string {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if p, e := exec.LookPath(name); e == nil {
			return p
		}
	}
	return ""
}

// A crashed process releases this advisory lock automatically.
func lockHome(home string) (*os.File, error) {
	if e := privateDir(home); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(home, "bridge.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another bridge or login command is using this profile; stop it first")
	}
	return f, nil
}
