package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func runDoctor(home string, c Config) []Check {
	checks := []Check{}
	add := func(name, status, detail string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail})
	}
	if info, e := os.Stat(home); e != nil {
		add("state directory", "FAIL", e.Error())
	} else if info.Mode().Perm()&0077 != 0 {
		add("state directory", "WARN", fmt.Sprintf("permissions %04o; expected 0700", info.Mode().Perm()))
	} else {
		add("state directory", "OK", "")
	}
	if info, e := os.Stat(filepath.Join(home, "session.json")); e != nil {
		add("saved session", "WARN", "missing; run auth-check or login")
	} else {
		if info.Mode().Perm()&0077 != 0 {
			add("saved session", "WARN", fmt.Sprintf("permissions %04o; expected 0600", info.Mode().Perm()))
		} else if data, e := os.ReadFile(filepath.Join(home, "session.json")); e != nil {
			add("saved session", "FAIL", e.Error())
		} else {
			var cookies []any
			if json.Unmarshal(data, &cookies) != nil || len(cookies) == 0 {
				add("saved session", "FAIL", "invalid or empty cookie file")
			} else {
				add("saved session", "OK", fmt.Sprintf("%d cookies", len(cookies)))
			}
		}
	}
	if data, e := os.ReadFile(filepath.Join(home, "messages.json")); e != nil {
		if os.IsNotExist(e) {
			add("message journal", "WARN", "not created yet")
		} else {
			add("message journal", "FAIL", e.Error())
		}
	} else {
		var rows map[string]Message
		if json.Unmarshal(data, &rows) != nil {
			add("message journal", "FAIL", "invalid JSON")
		} else {
			add("message journal", "OK", fmt.Sprintf("%d messages", len(rows)))
		}
	}
	if data, e := os.ReadFile(filepath.Join(home, "sync_cursor.json")); e != nil {
		add("sync cursor", "WARN", "missing; next sync performs a cold snapshot")
	} else {
		var cursor syncCursor
		if json.Unmarshal(data, &cursor) != nil || cursor.Version != syncCursorVersion || cursor.Account != c.Number || cursor.Token == "" {
			add("sync cursor", "WARN", "stale or invalid; next sync performs a cold snapshot")
		} else {
			add("sync cursor", "OK", "")
		}
	}
	if _, e := os.Stat(c.Browser); e != nil {
		add("chromium", "FAIL", "configured browser is not executable")
	} else {
		add("chromium", "OK", "")
	}
	if _, e := exec.LookPath(c.Browser); e != nil {
		add("chromium in PATH", "WARN", "configured by absolute path; ensure the service user can execute it")
	}
	if _, e := openWebhooks(home); e != nil {
		add("webhooks", "FAIL", e.Error())
	} else {
		add("webhooks", "OK", "")
	}
	var cacheBytes int64
	if entries, e := os.ReadDir(filepath.Join(home, "media")); e == nil {
		for _, entry := range entries {
			if info, e := entry.Info(); e == nil && !entry.IsDir() {
				cacheBytes += info.Size()
			}
		}
	}
	limit := cacheLimitBytes(c)
	status := "OK"
	if cacheBytes > limit {
		status = "WARN"
	}
	add("media cache", status, fmt.Sprintf("%d MiB used of %d MiB", cacheBytes/1024/1024, limit/1024/1024))
	if info, e := os.Stat(filepath.Join(home, "serve.log")); e == nil {
		add("service log", "OK", fmt.Sprintf("%d KiB", info.Size()/1024))
	} else {
		add("service log", "WARN", "not found")
	}
	var stat syscall.Statfs_t
	if e := syscall.Statfs(home, &stat); e == nil {
		free := int64(stat.Bavail) * int64(stat.Bsize)
		status := "OK"
		if free < 100*1024*1024 {
			status = "WARN"
		}
		add("disk space", status, fmt.Sprintf("%d MiB free", free/1024/1024))
	}
	client := http.Client{Timeout: 3 * time.Second}
	address := c.Listen
	if host, port, e := net.SplitHostPort(address); e == nil && (host == "0.0.0.0" || host == "::") {
		address = "127.0.0.1:" + port
	}
	request, e := http.NewRequest("GET", "http://"+address+"/health", nil)
	if e == nil {
		request.SetBasicAuth(c.Username, c.Password)
		if response, e := client.Do(request); e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				add("service", "OK", "ready")
			} else {
				add("service", "WARN", fmt.Sprintf("health HTTP %d", response.StatusCode))
			}
		} else {
			add("service", "WARN", "not running")
		}
	}
	return checks
}

func (c Check) String() string {
	if c.Detail == "" {
		return fmt.Sprintf("%-4s %s", c.Status, c.Name)
	}
	return fmt.Sprintf("%-4s %s: %s", c.Status, c.Name, c.Detail)
}
