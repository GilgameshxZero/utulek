package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestLiveSendProtocol(t *testing.T) {
	if os.Getenv("GV_LIVE_SEND_PROTOCOL") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	number := os.Getenv("GV_REMOTE_NUMBER")
	if !phonePattern.MatchString(number) {
		t.Fatal("explicit remote required")
	}
	c.SyncNumbers = []string{number}
	journalPath := filepath.Join(".obj", "send_protocol_journal.json")
	var journal struct {
		ID     string `json:"id"`
		Text   string `json:"text"`
		State  string `json:"state"`
		Source string `json:"source,omitempty"`
	}
	data, e := os.ReadFile(journalPath)
	if e != nil || json.Unmarshal(data, &journal) != nil || journal.State != "prepared" || journal.Text == "" {
		t.Fatal("prepared one-shot send journal required; never repeat an attempt")
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	before, e := b.readNetworkHistory()
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range before {
		if m.Body != nil && m.Body.Text == journal.Text {
			t.Fatal("test content already exists; reconcile instead of resending")
		}
	}
	var ready bool
	js := `(()=>{const m=[...document.querySelectorAll('gv-thread-list-item')].filter(e=>(e.querySelector('.participants')?.innerText||'').replace(/\D/g,'')===` + literal(strings.TrimPrefix(number, "+")) + `||(e.querySelector('.participants')?.innerText||'').replace(/\D/g,'')===` + literal(strings.TrimPrefix(number, "+1")) + `);if(m.length!==1)return false;m[0].querySelector('[role="button"]').click();return true})()`
	if e = b.run(chromedp.Poll(js, &ready, chromedp.WithPollingTimeout(15*time.Second))); e != nil {
		t.Fatal(e)
	}
	js = `(()=>{const n=document.querySelector('gv-thread-details-header .header-title')?.innerText.replace(/\D/g,'')||'';return new URL(location.href).searchParams.get('itemId')===` + literal("t."+number) + `&&(n===` + literal(strings.TrimPrefix(number, "+")) + `||n===` + literal(strings.TrimPrefix(number, "+1")) + `)})()`
	if e = b.run(chromedp.Poll(js, &ready, chromedp.WithPollingTimeout(15*time.Second)), chromedp.WaitVisible(`gv-message-entry textarea[placeholder="Type a message"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	type item struct {
		ID     network.RequestID
		Path   string
		Method string
	}
	var mu sync.Mutex
	var requests []item
	chromedp.ListenTarget(b.ctx, func(ev any) {
		if r, ok := ev.(*network.EventRequestWillBeSent); ok {
			u, e := url.Parse(r.Request.URL)
			if e == nil && u.Hostname() == "clients6.google.com" && strings.Contains(strings.ToLower(u.Path), "send") {
				mu.Lock()
				requests = append(requests, item{r.RequestID, u.Path, r.Request.Method})
				mu.Unlock()
			}
		}
	})
	var filled bool
	js = `(()=>{const e=document.querySelector('gv-message-entry textarea[placeholder="Type a message"]');if(!e||e.value!=='')return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(journal.Text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(journal.Text) + `})()`
	if e = b.run(chromedp.Evaluate(js, &filled)); e != nil || !filled {
		t.Fatal("could not fill empty composer", e)
	}
	if e = b.run(chromedp.Poll(`!!document.querySelector('gv-message-entry button[aria-label="Send message"]:not([disabled])')`, &ready, chromedp.WithPollingTimeout(10*time.Second))); e != nil {
		t.Fatal(e)
	}
	journal.State = "attempted"
	if e = writeJSON(journalPath, journal); e != nil {
		t.Fatal(e)
	}
	var clicked bool
	js = `(()=>{const e=document.querySelector('gv-message-entry textarea[placeholder="Type a message"]');const b=document.querySelector('gv-message-entry button[aria-label="Send message"]');if(!e||e.value!==` + literal(journal.Text) + `||!b||b.disabled)return false;b.click();return true})()`
	if e = b.run(chromedp.Evaluate(js, &clicked)); e != nil || !clicked {
		t.Fatal("send outcome uncertain; inspect journal, do not retry", e)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		rows, e := b.readNetworkHistory()
		if e == nil {
			for _, m := range rows {
				if m.Body != nil && m.Body.Text == journal.Text && len(m.Recipients) == 1 && m.Recipients[0].Number == number {
					journal.Source = m.Source
					journal.State = "confirmed"
				}
			}
		}
		if journal.Source != "" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	mu.Lock()
	captured := append([]item(nil), requests...)
	mu.Unlock()
	result := []map[string]any{}
	for _, r := range captured {
		var post string
		var body []byte
		if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var e error
			post, e = network.GetRequestPostData(r.ID).Do(ctx)
			return e
		})); e != nil {
			post = "unavailable"
		}
		if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var e error
			body, e = network.GetResponseBody(r.ID).Do(ctx)
			return e
		})); e != nil {
			body = []byte("unavailable")
		}
		var fields []json.RawMessage
		_ = json.Unmarshal([]byte(post), &fields)
		result = append(result, map[string]any{"path": r.Path, "method": r.Method, "request_field_count": len(fields), "response": string(body), "sensitive_request_fields": "not retained"})
	}
	if e = writeJSON(filepath.Join(".obj", "send_protocol_payload.json"), result); e != nil {
		t.Fatal(e)
	}
	if e = writeJSON(journalPath, journal); e != nil {
		t.Fatal(e)
	}
	if journal.Source == "" {
		t.Fatal(errors.New("send unconfirmed; journal retained; never repeat automatically"))
	}
	// Recreate the browser and verify authoritative history again after reload.
	if e = b.run(chromedp.Reload()); e != nil {
		t.Fatal(e)
	}
	rows, e := b.readNetworkHistory()
	if e != nil {
		t.Fatal(e)
	}
	matches := 0
	for _, m := range rows {
		if m.Source == journal.Source && m.Body != nil && m.Body.Text == journal.Text {
			matches++
		}
	}
	if matches != 1 {
		t.Fatal("persisted send not uniquely confirmed")
	}
	t.Logf("single send confirmed by native ID and persisted after reload; %d send requests captured", len(result))
}
