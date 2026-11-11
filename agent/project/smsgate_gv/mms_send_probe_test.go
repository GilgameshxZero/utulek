package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

// Opt-in live discovery: sends one real MMS to the authorized Android number and
// records only request paths and body shapes, never message content or media.
func TestLiveMMSSendDiscovery(t *testing.T) {
	if os.Getenv("GV_LIVE_MMS_SEND") != "1" {
		t.Skip("explicit opt-in; sends one test MMS")
	}
	number := "+13852679447"
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.openRecipient(number); e != nil {
		t.Fatal(e)
	}
	image := make([]byte, 0, 100)
	image, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	imagePath := filepath.Join(".obj", "mms_test.png")
	if e = os.MkdirAll(filepath.Dir(imagePath), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(imagePath, image, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(imagePath)
	var dom struct {
		Inputs  []string `json:"inputs"`
		Buttons []string `json:"buttons"`
	}
	_ = b.run(chromedp.Evaluate(`(() => ({
		inputs: [...document.querySelectorAll('input[type=file]')].map(i => ({accept: i.accept, multiple: i.multiple})),
		buttons: [...document.querySelectorAll('gv-message-entry button')].map(b => b.getAttribute('aria-label') || b.title || '').filter(Boolean)
	}))()`, &dom))
	type request struct {
		Host       string   `json:"host"`
		Path       string   `json:"path"`
		Method     string   `json:"method"`
		PostLength int      `json:"post_length"`
		Fields     []string `json:"fields"`
	}
	var mu sync.Mutex
	requests := []request{}
	ids := map[network.RequestID]string{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		e, ok := event.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		u, err := url.Parse(e.Request.URL)
		if err != nil || (e.Request.Method != "POST" && e.Request.Method != "PUT") {
			return
		}
		mu.Lock()
		ids[e.RequestID] = u.Host + "\x00" + u.Path + "\x00" + e.Request.Method
		mu.Unlock()
	})
	if e = b.run(chromedp.SetUploadFiles(`input[type=file]`, []string{imagePath}, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	text := "smsgate_gv MMS test " + time.Now().UTC().Format("150405")
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e)return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(text) + `})()`
	if e = b.run(chromedp.Evaluate(js, &ready)); e != nil || !ready {
		t.Fatal("composer not ready", e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const b=document.querySelector('gv-message-entry button[aria-label="Send message"]');return !!b&&!b.disabled})()`, &ready, chromedp.WithPollingTimeout(30*time.Second))); e != nil {
		t.Fatal("send button not enabled", e)
	}
	if e = b.run(chromedp.Click(`gv-message-entry button[aria-label="Send message"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(15 * time.Second)
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	idCopy := map[network.RequestID]string{}
	for id, path := range ids {
		idCopy[id] = path
	}
	mu.Unlock()
	for id, path := range idCopy {
		parts := strings.Split(path, "\x00")
		var body []byte
		_ = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			text, err := network.GetRequestPostData(id).Do(ctx)
			body = []byte(text)
			return err
		}))
		fields := []string{}
		var value []json.RawMessage
		if json.Unmarshal(body, &value) == nil {
			fields = fieldTypes(value)
		}
		requests = append(requests, request{Host: parts[0], Path: parts[1], Method: parts[2], PostLength: len(body), Fields: fields})
	}
	if e = writeJSON(filepath.Join(".obj", "mms_send_probe.json"), map[string]any{"inputs": dom.Inputs, "buttons": dom.Buttons, "requests": requests}); e != nil {
		t.Fatal(e)
	}
	t.Logf("inputs=%d buttons=%d requests=%d", len(dom.Inputs), len(dom.Buttons), len(requests))
}

func TestLiveMMSAttachDebug(t *testing.T) {
	if os.Getenv("GV_LIVE_MMS_DEBUG") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.openRecipient("+13852679447"); e != nil {
		t.Fatal(e)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	m := Message{ID: "debug", Attachments: []Attachment{{PartID: 1, Name: "test.png", ContentType: "image/png", Inline: png}}}
	files, cleanup, e := writeOutboxFiles(home, m)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	if e = b.attach(files); e != nil {
		t.Fatal(e)
	}
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e)return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'debug attach');e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`
	_ = b.run(chromedp.Evaluate(js, &ready))
	time.Sleep(20 * time.Second)
	var state struct {
		Inputs    []map[string]any `json:"inputs"`
		Buttons   []map[string]any `json:"buttons"`
		Textareas []map[string]any `json:"textareas"`
		Media     int              `json:"media"`
		Value     string           `json:"value"`
	}
	_ = b.run(chromedp.Evaluate(`(() => ({
		inputs: [...document.querySelectorAll('input[type=file]')].map(i => ({multiple: i.multiple, files: i.files ? i.files.length : -1})),
		buttons: [...document.querySelectorAll('gv-message-entry button')].map(b => ({label: b.getAttribute('aria-label')||'', disabled: b.disabled})),
		media: document.querySelectorAll('gv-message-entry img, gv-message-entry [class*=attachment], gv-message-entry [class*=media]').length,
		textareas: [...document.querySelectorAll('gv-message-entry textarea')].map(t => ({placeholder: t.placeholder||'', disabled: t.disabled, value: t.value||''})),
		value: (document.querySelector('`+composerSelector+`')||{}).value||''
	}))()`, &state))
	t.Logf("state=%+v", state)
}
