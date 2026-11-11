package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	_ "modernc.org/sqlite"
)

var errAuthPaused = errors.New("authentication paused; retrying saved session and browser cookie import periodically; refresh Google Voice in the source browser if needed")

func googleDomain(domain string) bool {
	d := strings.TrimPrefix(domain, ".")
	return d == "google.com" || strings.HasSuffix(d, ".google.com")
}

func defaultCookieSource() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "chromium", "Default")
}

// SQLite creates a consistent snapshot including committed WAL content. Never
// copy a live database and its WAL independently, or launch the source profile.
func snapshotCookies(ctx context.Context, source, destination string) error {
	u := &url.URL{Scheme: "file", Path: source}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "busy_timeout(3000)")
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "VACUUM INTO ?", destination)
	return e
}

func copySourceProfile(ctx context.Context, source, target string) error {
	if source == "" {
		source = defaultCookieSource()
	}
	source, e := filepath.Abs(source)
	if e != nil {
		return e
	}
	state, e := os.ReadFile(filepath.Join(filepath.Dir(source), "Local State"))
	if e != nil {
		return errors.New("source browser Local State unavailable")
	}
	if e = os.WriteFile(filepath.Join(target, "Local State"), state, 0600); e != nil {
		return e
	}
	dest := filepath.Join(target, "Default")
	if e = privateDir(dest); e != nil {
		return e
	}
	cookieSource := filepath.Join(source, "Cookies")
	if _, e = os.Stat(cookieSource); os.IsNotExist(e) {
		cookieSource = filepath.Join(source, "Network", "Cookies")
	}
	path := filepath.Join(dest, "Cookies")
	if e = snapshotCookies(ctx, cookieSource, path); e != nil {
		return errors.New("cannot snapshot source browser cookies")
	}
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return e
	}
	defer db.Close()
	// Remove unrelated sessions before starting the private browser.
	_, e = db.ExecContext(ctx, `DELETE FROM cookies WHERE host_key NOT IN ('google.com','.google.com') AND host_key NOT LIKE '%.google.com'`)
	return e
}

func cookieParams(cookies []*network.Cookie) []*network.CookieParam {
	out := []*network.CookieParam{}
	for _, c := range cookies {
		if !googleDomain(c.Domain) || c.PartitionKeyOpaque {
			continue
		}
		if !c.Session && c.Expires <= float64(time.Now().Unix()) {
			continue
		}
		p := &network.CookieParam{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly, SameSite: c.SameSite, Priority: c.Priority, SourceScheme: c.SourceScheme, SourcePort: c.SourcePort, PartitionKey: c.PartitionKey}
		if !c.Session {
			t := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
			p.Expires = &t
		}
		out = append(out, p)
	}
	return out
}

func (b *Browser) saveSession(home string) error {
	var cookies []*network.Cookie
	if e := b.run(chromedp.ActionFunc(func(ctx context.Context) error { var e error; cookies, e = storage.GetCookies().Do(ctx); return e })); e != nil {
		return e
	}
	filtered := []*network.Cookie{}
	for _, c := range cookies {
		if googleDomain(c.Domain) {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return errors.New("no Google session cookies available")
	}
	return writeJSON(filepath.Join(home, "session.json"), filtered)
}

func startBrowser(parent context.Context, home string, c Config) (*Browser, error) {
	return startBrowserWithProgress(parent, home, c, func(string) {})
}

func startBrowserWithProgress(parent context.Context, home string, c Config, progress func(string)) (*Browser, error) {
	progress("Checking saved Google session...")
	// Each service browser gets a fresh disposable profile. Saved cookies are
	// injected through CDP, independent of a desktop wallet at service startup.
	if data, e := os.ReadFile(filepath.Join(home, "session.json")); e == nil {
		var cookies []*network.Cookie
		if json.Unmarshal(data, &cookies) == nil && len(cookieParams(cookies)) > 0 {
			b, e := launchBrowser(parent, home, c, func(string) error { return nil })
			if e == nil {
				e = b.run(storage.SetCookies(cookieParams(cookies)))
				if e == nil {
					e = b.verify()
				}
				if e == nil {
					e = b.saveSession(home)
				}
				if e == nil {
					return b, nil
				}
				b.close()
			}
		}
	}
	if parent.Err() != nil {
		return nil, parent.Err()
	}
	progress("Saved session unavailable; importing source browser cookies...")
	b, e := launchBrowser(parent, home, c, func(profile string) error { return copySourceProfile(parent, c.CookieSource, profile) })
	if e != nil {
		return nil, errAuthPaused
	}
	progress("Checking imported Google session...")
	if e = b.verify(); e == nil {
		e = b.saveSession(home)
	}
	if e != nil {
		b.close()
		return nil, errAuthPaused
	}
	return b, nil
}

// Retrying a stale browser session must not become a tight restart loop. An
// explicit history refresh also respects this gate.
type authRetry struct {
	next     time.Time
	interval time.Duration
}

func (r *authRetry) connect(at time.Time, start func() (*Browser, error)) (*Browser, error) {
	if at.Before(r.next) {
		return nil, errAuthPaused
	}
	b, e := start()
	if e != nil {
		r.next = at.Add(r.interval)
		return nil, e
	}
	r.next = time.Time{}
	return b, nil
}

// The one-shot check must finish, even if Chromium or the source wallet stalls.
// The service has its own periodic retry loop; this command never waits for it.
func checkAuthentication(parent context.Context, home string, c Config, progress func(string)) error {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	b, e := startBrowserWithProgress(ctx, home, c, progress)
	if e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return e
	}
	progress("Authentication verified; closing temporary browser...")
	b.close()
	return nil
}
