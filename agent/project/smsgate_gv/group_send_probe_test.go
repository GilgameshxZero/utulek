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

// Opt-in: creates/sends one MMS to two authorized numbers and records only
// request paths and body shapes.
func TestLiveGroupSendDiscovery(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_SEND") != "1" {
		t.Skip("explicit opt-in; sends one test MMS to authorized numbers")
	}
	numbers := []string{"+13852679447", "+18015582948"}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
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
	if e = b.run(chromedp.Click(`[aria-label="Send new message"]`, chromedp.ByQuery), chromedp.WaitVisible(`input[placeholder="Type a name or phone number"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	var inputs []map[string]any
	_ = b.run(chromedp.Evaluate(`(()=>[...document.querySelectorAll('input')].map(i=>({ph:i.placeholder||'',type:i.type,dis:i.disabled})))()`, &inputs))
	t.Logf("inputs=%v", inputs)
	for _, number := range numbers {
		var typed bool
		typeJS := `(()=>{const i=document.querySelector('input[placeholder="Type a name or phone number"]');if(!i)return false;i.focus();Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,` + literal(number) + `);i.dispatchEvent(new Event('input',{bubbles:true}));return true})()`
		if e = b.run(chromedp.Evaluate(typeJS, &typed)); e != nil || !typed {
			t.Fatalf("cannot enter recipient: %v", e)
		}
		var selected bool
		js := `(()=>{const allowed=` + literal(recipientDigits(number)) + `;const options=[...document.querySelectorAll('[role="option"],.send-to-button')].filter(e=>allowed.includes((e.querySelector('.send-to-label')?.innerText||e.getAttribute('aria-label')||e.innerText||'').replace(/\D/g,'')));if(options.length!==1)return false;options[0].click();return true})()`
		if e = b.run(chromedp.Poll(js, &selected, chromedp.WithPollingTimeout(15*time.Second))); e != nil {
			t.Fatal(e)
		}
	}
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	imagePath := filepath.Join(".obj", "group_test.png")
	_ = os.MkdirAll(filepath.Dir(imagePath), 0700)
	if e = os.WriteFile(imagePath, image, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(imagePath)
	if e = b.attach([]string{imagePath}); e != nil {
		t.Fatal(e)
	}
	text := "smsgate_gv group test " + time.Now().UTC().Format("150405")
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e)return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(text) + `})()`
	if e = b.run(chromedp.Evaluate(js, &ready)); e != nil || !ready {
		t.Fatal("caption not set", e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const b=document.querySelector('gv-message-entry button[aria-label^="Send message"]');return !!b&&!b.disabled})()`, &ready, chromedp.WithPollingTimeout(45*time.Second))); e != nil {
		t.Fatal("send button not ready", e)
	}
	type request struct {
		Host       string   `json:"host"`
		Path       string   `json:"path"`
		PostLength int      `json:"post_length"`
		Fields     []string `json:"fields"`
	}
	var mu sync.Mutex
	ids := map[network.RequestID]string{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		e, ok := event.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		u, err := url.Parse(e.Request.URL)
		if err != nil || e.Request.Method != "POST" || !strings.Contains(u.Path, "/voice/v1/voiceclient/") {
			return
		}
		mu.Lock()
		ids[e.RequestID] = u.Host + "\x00" + u.Path
		mu.Unlock()
	})
	if e = b.run(chromedp.Click(`gv-message-entry button[aria-label^="Send message"]`, chromedp.ByQuery)); e != nil {
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
	requests := []request{}
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
		requests = append(requests, request{Host: parts[0], Path: parts[1], PostLength: len(body), Fields: fields})
	}
	if e = writeJSON(filepath.Join(".obj", "group_send_probe.json"), map[string]any{"requests": requests}); e != nil {
		t.Fatal(e)
	}
	t.Logf("requests=%d", len(requests))
}
