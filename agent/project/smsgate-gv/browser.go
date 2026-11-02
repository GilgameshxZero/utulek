package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
)

type Browser struct {
	ctx             context.Context
	cancel          context.CancelFunc
	allocCancel     context.CancelFunc
	config          Config
	base            string
	profile         string
	historyRequest  *historyRequest
	historyPostBody string
	cursor          string
	pendingCursor   string
	moreUpdates     bool
	updates         chan struct{}
	watchReady      atomic.Bool
	apiInFlight     atomic.Int32
}

func launchBrowser(parent context.Context, home string, c Config, prepare func(string) error) (*Browser, error) {
	if e := privateDir(home); e != nil {
		return nil, e
	}
	profile, e := os.MkdirTemp(home, "browser-*")
	if e != nil {
		return nil, e
	}
	if e = prepare(profile); e != nil {
		os.RemoveAll(profile)
		return nil, e
	}
	opts := []chromedp.ExecAllocatorOption{chromedp.ExecPath(c.Browser), chromedp.UserDataDir(profile), chromedp.Headless, chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.Flag("disable-dev-shm-usage", true)}
	ac, ca := chromedp.NewExecAllocator(parent, opts...)
	bc, cb := chromedp.NewContext(ac, chromedp.WithLogf(func(string, ...any) {}), chromedp.WithErrorf(func(string, ...any) {}))
	b := &Browser{ctx: bc, cancel: cb, allocCancel: ca, config: c, base: fmt.Sprintf("https://voice.google.com/u/%d/", c.Account), profile: profile}
	// Initialize on the long-lived context: cancelling a startup child context
	// would otherwise terminate Chromium after the first action.
	if e = chromedp.Run(b.ctx, chromedp.Navigate("about:blank")); e != nil {
		b.close()
		return nil, e
	}
	return b, nil
}
func (b *Browser) close() {
	// Ask Chrome to exit cleanly before deleting its profile; killing only the
	// parent can leave child processes writing files back during shutdown.
	shutdown, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	_ = chromedp.Cancel(shutdown)
	cancel()
	b.cancel()
	b.allocCancel()
	_ = os.RemoveAll(b.profile)
}
func literal(v any) string { data, _ := json.Marshal(v); return string(data) }
func (b *Browser) run(actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(b.ctx, 45*time.Second)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

const accountNumberJS = `(() => {
 const labels = [...document.querySelectorAll('h1,h2,h3,h4,div,span')].filter(e=>e.children.length===0 && e.textContent.trim()==='Google Voice number');
 for(const label of labels) { let p=label; for(let i=0;i<5&&p;i++,p=p.parentElement) {
   const t=p.innerText||''; if(t.length>1500) break;
   const m=t.match(/(?:\+?1[ .-]?)?\(?\d{3}\)?[ .-]\d{3}[ .-]\d{4}/);
   if(m) {let n=m[0].replace(/\D/g,'');return '+'+(n.length===10?'1':'')+n;}
 }} return '';
})()`

func (b *Browser) verify() error {
	var number string
	if e := b.run(chromedp.Navigate(b.base+"settings"), chromedp.Poll(accountNumberJS, &number, chromedp.WithPollingTimeout(25*time.Second), chromedp.WithPollingInterval(500*time.Millisecond))); e != nil {
		return errAuthPaused
	}
	if number != b.config.Number {
		return errors.New("Google Voice sender differs from configured number; check account and number in config.json")
	}
	return nil
}

func (b *Browser) sync(store *Store) error {
	if b.historyRequest == nil && b.cursor == "" {
		var saved syncCursor
		if data, e := os.ReadFile(filepath.Join(filepath.Dir(store.path), "sync_cursor.json")); e == nil && json.Unmarshal(data, &saved) == nil && saved.Account == b.config.Number && len(store.list(true))+len(store.list(false)) > 0 {
			b.cursor = saved.Token
		}
	}
	rows, e := b.readNetworkHistory()
	if e != nil {
		return e
	}
	if e = store.mergeImports(rows); e != nil {
		return e
	}
	return b.commitCursor(store)
}
