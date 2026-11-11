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

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Opt-in: creates/sends one MMS to two authorized numbers and records only
// request paths and body shapes.
func TestLiveGroupSendDiscovery(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_SEND") != "1" {
		t.Skip("explicit opt-in; sends one test MMS to authorized numbers")
	}
	numbers := []string{"+13852679447", "+18508888724"}
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
	optionJS := func(number string) string {
		return `(()=>{const allowed=` + literal(recipientDigits(number)) + `;const options=[...document.querySelectorAll('[role="option"],.send-to-button')].filter(e=>allowed.includes((e.querySelector('.send-to-label')?.innerText||e.getAttribute('aria-label')||e.innerText||'').replace(/\D/g,'')));if(options.length!==1)return false;options[0].click();return true})()`
	}
	var selected bool
	if e = b.run(chromedp.Click(`[aria-label="Send new message"]`, chromedp.ByQuery)); e != nil {
		t.Fatalf("cannot open new message: %v", e)
	}
	time.Sleep(time.Second)
	var warm []byte
	_ = b.run(chromedp.CaptureScreenshot(&warm))
	time.Sleep(3 * time.Second)
	var settle map[string]any
	_ = b.run(chromedp.Evaluate(`(() => { const i=document.querySelector('input[placeholder="Type a name or phone number"]'); return {found:!!i, focus:document.hasFocus(), active:document.activeElement?document.activeElement.tagName:''}; })()`, &settle))
	t.Logf("settle=%v", settle)
	for _, number := range numbers {
		var rect struct {
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			W     float64 `json:"w"`
			H     float64 `json:"h"`
			Found bool    `json:"found"`
		}
		_ = b.run(chromedp.Evaluate(`(() => { const r=document.querySelector('input.mat-mdc-chip-input')?.getBoundingClientRect(); return r?{x:r.x,y:r.y,w:r.width,h:r.height,found:true}:{found:false}; })()`, &rect))
		if !rect.Found {
			t.Fatalf("recipient input not found before %s", number)
		}
		if e = b.run(chromedp.MouseClickXY(rect.X+rect.W/2, rect.Y+rect.H/2)); e != nil {
			t.Fatalf("cannot click recipient input: %v", e)
		}
		if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error { return input.InsertText(number).Do(ctx) })); e != nil {
			t.Fatalf("cannot insert recipient: %v", e)
		}
		if e = b.run(chromedp.Poll(optionJS(number), &selected, chromedp.WithPollingTimeout(15*time.Second))); e != nil {
			t.Fatalf("cannot select recipient: %v", e)
		}
		time.Sleep(500 * time.Millisecond)
	}
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	imagePath := filepath.Join(".obj", "group_test.png")
	_ = os.MkdirAll(filepath.Dir(imagePath), 0700)
	if e = os.WriteFile(imagePath, image, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(imagePath)
	var gstate map[string]any
	_ = b.run(chromedp.Evaluate(`(() => ({
		files: [...document.querySelectorAll('input[type=file]')].map(i=>({multiple:i.multiple,accept:i.accept})),
		textareas: [...document.querySelectorAll('gv-message-entry textarea')].map(t=>({ph:t.placeholder||'',dis:t.disabled})),
		buttons: [...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})),
		media: document.querySelectorAll('gv-message-entry img, gv-message-entry [class*=attachment]').length
	}))()`, &gstate))
	t.Logf("gstate=%v", gstate)
	_ = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery))
	time.Sleep(time.Second)
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

func TestLiveGroupCreateDebug(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_DEBUG") != "1" {
		t.Skip("explicit opt-in; no sends")
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
	if e = b.run(chromedp.Click(`[aria-label="Send new message"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(5 * time.Second)
	var shot1 []byte
	_ = b.run(chromedp.CaptureScreenshot(&shot1))
	_ = os.WriteFile(filepath.Join(".obj", "group1.png"), shot1, 0600)
	time.Sleep(3 * time.Second)
	var shot2 []byte
	_ = b.run(chromedp.CaptureScreenshot(&shot2))
	_ = os.WriteFile(filepath.Join(".obj", "group2.png"), shot2, 0600)
	var state map[string]any
	_ = b.run(chromedp.Evaluate(`(() => {
		const i = document.querySelector('input[placeholder="Type a name or phone number"]');
		let rect = null, at = null;
		if (i) { const r = i.getBoundingClientRect(); rect = {x:r.x,y:r.y,w:r.width,h:r.height,vis:getComputedStyle(i).visibility,disp:getComputedStyle(i).display}; const e=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2); if(e) at={tag:e.tagName,cls:String(e.className||'').slice(0,60)}; }
		return {focus:document.hasFocus(), active:document.activeElement?document.activeElement.tagName:'', input:rect, at};
	})()`, &state))
	t.Logf("state=%v", state)
	if e = b.run(chromedp.SendKeys(`input[placeholder="Type a name or phone number"]`, "+13852679447", chromedp.ByQuery)); e != nil {
		t.Fatalf("sendkeys: %v", e)
	}
	time.Sleep(2 * time.Second)
	var after map[string]any
	_ = b.run(chromedp.Evaluate(`(() => { const i=document.querySelector('input[placeholder="Type a name or phone number"]'); const opts=[...document.querySelectorAll('[role="option"],.send-to-button')]; return {value:(i&&i.value||'').length, options:opts.length}; })()`, &after))
	t.Logf("after=%v", after)
}

func TestLiveGroupTextSend(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_TEXT") != "1" {
		t.Skip("explicit opt-in; sends one text to two authorized numbers")
	}
	numbers := []string{"+13852679447", "+18508888724"}
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
	if e = b.run(chromedp.Click(`[aria-label="Send new message"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Second)
	optionJS := func(number string) string {
		return `(()=>{const allowed=` + literal(recipientDigits(number)) + `;const options=[...document.querySelectorAll('[role="option"],.send-to-button')].filter(e=>allowed.includes((e.querySelector('.send-to-label')?.innerText||e.getAttribute('aria-label')||e.innerText||'').replace(/\D/g,'')));if(options.length!==1)return false;options[0].click();return true})()`
	}
	var selected bool
	for _, number := range numbers {
		var rect struct {
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			W     float64 `json:"w"`
			H     float64 `json:"h"`
			Found bool    `json:"found"`
		}
		_ = b.run(chromedp.Evaluate(`(() => { const r=document.querySelector('input.mat-mdc-chip-input')?.getBoundingClientRect(); return r?{x:r.x,y:r.y,w:r.width,h:r.height,found:true}:{found:false}; })()`, &rect))
		if !rect.Found {
			t.Fatal("recipient input missing")
		}
		if e = b.run(chromedp.MouseClickXY(rect.X+rect.W/2, rect.Y+rect.H/2)); e != nil {
			t.Fatal(e)
		}
		if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error { return input.InsertText(number).Do(ctx) })); e != nil {
			t.Fatal(e)
		}
		if e = b.run(chromedp.Poll(optionJS(number), &selected, chromedp.WithPollingTimeout(15*time.Second))); e != nil {
			t.Fatal(e)
		}
		time.Sleep(500 * time.Millisecond)
	}
	text := "smsgate_gv group text " + time.Now().UTC().Format("150405")
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e)return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(text) + `})()`
	if e = b.run(chromedp.Evaluate(js, &ready)); e != nil || !ready {
		t.Fatal("text not set", e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const b=document.querySelector('gv-message-entry button[aria-label^="Send message"]');return !!b&&!b.disabled})()`, &ready, chromedp.WithPollingTimeout(20*time.Second))); e != nil {
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
	if e = writeJSON(filepath.Join(".obj", "group_text_probe.json"), map[string]any{"requests": requests}); e != nil {
		t.Fatal(e)
	}
	t.Logf("requests=%d", len(requests))
}

func TestLiveGroupReplySend(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_REPLY") != "1" {
		t.Skip("explicit opt-in; replies in an authorized group")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	if groupID == "" {
		t.Fatal("no two-participant group found")
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	var opened bool
	openJS := `(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(` + literal(groupID) + `)+']');if(!el)return false;const inner=el.querySelector('a,[role="button"],button')||el;inner.click();return true})()`
	if e = b.run(chromedp.Poll(openJS, &opened, chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal("cannot open group", e)
	}
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	imagePath := filepath.Join(".obj", "group_reply.png")
	_ = os.MkdirAll(filepath.Dir(imagePath), 0700)
	if e = os.WriteFile(imagePath, image, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(imagePath)
	if e = b.attach([]string{imagePath}); e != nil {
		t.Fatal("attach", e)
	}
	text := "smsgate_gv group reply " + time.Now().UTC().Format("150405")
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e)return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(text) + `})()`
	if e = b.run(chromedp.Evaluate(js, &ready)); e != nil || !ready {
		t.Fatal("caption not set", e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const b=document.querySelector('gv-message-entry button[aria-label^="Send message"]');return !!b&&!b.disabled})()`, &ready, chromedp.WithPollingTimeout(45*time.Second))); e != nil {
		t.Fatal("send not ready", e)
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
	if e = writeJSON(filepath.Join(".obj", "group_reply_probe.json"), map[string]any{"group": bool(groupID != ""), "requests": requests}); e != nil {
		t.Fatal(e)
	}
	t.Logf("requests=%d", len(requests))
}

func TestLiveGroupAttachChooser(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_CHOOSER") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	if groupID == "" {
		t.Fatal("no group")
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	if e = b.run(page.SetInterceptFileChooserDialog(true)); e != nil {
		t.Fatal(e)
	}
	chooser := make(chan cdp.BackendNodeID, 1)
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*page.EventFileChooserOpened); ok {
			select {
			case chooser <- e.BackendNodeID:
			default:
			}
		}
	})
	if e = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Second)
	var uploadEls []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"],div,span')].filter(e => (e.innerText||'').trim() === 'Upload').map(e => { const r=e.getBoundingClientRect(); return {x:r.x,y:r.y,w:r.width,h:r.height}; }))()`, &uploadEls))
	if len(uploadEls) == 0 {
		t.Fatal("no Upload menu item")
	}
	x, _ := uploadEls[0]["x"].(float64)
	y, _ := uploadEls[0]["y"].(float64)
	w, _ := uploadEls[0]["w"].(float64)
	h, _ := uploadEls[0]["h"].(float64)
	if e = b.run(chromedp.MouseClickXY(x+w/2, y+h/2)); e != nil {
		t.Fatal(e)
	}
	var node cdp.BackendNodeID
	select {
	case node = <-chooser:
	case <-time.After(10 * time.Second):
		t.Fatal("no file chooser")
	}
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "chooser.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	var nodeInfo *cdp.Node
	var nodeFiles map[string]any
	var uploads []string
	var uploadMu sync.Mutex
	chromedp.ListenTarget(b.ctx, func(event any) {
		e, ok := event.(*network.EventRequestWillBeSent)
		if !ok || e.Request.Method != "POST" || strings.Contains(e.Request.URL, "/voice/v1/voiceclient/") {
			return
		}
		uploadMu.Lock()
		uploads = append(uploads, e.Request.URL)
		uploadMu.Unlock()
	})
	if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		nodeInfo, err = dom.DescribeNode().WithBackendNodeID(node).Do(ctx)
		if err != nil {
			return err
		}
		object, err := dom.ResolveNode().WithBackendNodeID(node).Do(ctx)
		if err != nil {
			return err
		}
		if err = dom.SetFileInputFiles([]string{path}).WithObjectID(object.ObjectID).Do(ctx); err != nil {
			return err
		}
		result, _, err := runtime.CallFunctionOn(`function(){return {files:this.files?this.files.length:-1, value:this.value||''}}`).WithObjectID(object.ObjectID).WithReturnByValue(true).Do(ctx)
		if err == nil && len(result.Value) > 0 {
			_ = json.Unmarshal(result.Value, &nodeFiles)
		}
		return nil
	})); e != nil {
		t.Fatal(e)
	}
	t.Logf("node=%s attrs=%v nodeFiles=%v uploads=%v", nodeInfo.NodeName, nodeInfo.Attributes, nodeFiles, uploads)
	time.Sleep(15 * time.Second)
	var state map[string]any
	_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length, inputs:[...document.querySelectorAll('input[type=file]')].map(i=>i.files?i.files.length:-1)}))()`, &state))
	t.Logf("chooser=%v state=%v", node, state)
}

func TestLiveGroupPickerTarget(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_PICKER") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	if groupID == "" {
		t.Fatal("no group")
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	if e = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(3 * time.Second)
	targets, e := chromedp.Targets(b.ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, tg := range targets {
		t.Logf("target type=%s url=%s", tg.Type, tg.URL)
	}
	for _, tg := range targets {
		if tg.Type != "iframe" {
			continue
		}
		tctx, tcancel := chromedp.NewContext(b.ctx, chromedp.WithTargetID(tg.TargetID))
		defer tcancel()
		var inputs int
		var labels []string
		if e := chromedp.Run(tctx, chromedp.Evaluate(`(() => { window.__labels=[...document.querySelectorAll('button,[role="button"]')].map(b=>(b.getAttribute('aria-label')||b.innerText||'').trim()).filter(Boolean).slice(0,20); return document.querySelectorAll('input[type=file]').length; })()`, &inputs)); e != nil {
			t.Logf("iframe eval: %v", e)
			continue
		}
		_ = chromedp.Run(tctx, chromedp.Evaluate(`window.__labels`, &labels))
		t.Logf("picker inputs=%d labels=%v", inputs, labels)
		image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
		path := filepath.Join(".obj", "picker.png")
		_ = os.WriteFile(path, image, 0600)
		defer os.Remove(path)
		if e := chromedp.Run(tctx, chromedp.SetUploadFiles(`input[type=file]`, []string{path}, chromedp.ByQuery)); e != nil {
			t.Logf("set files: %v", e)
		}
		time.Sleep(10 * time.Second)
		var state map[string]any
		_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length}))()`, &state))
		t.Logf("state=%v", state)
	}
}

func TestLiveGroupDirectUpload(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_DIRECT") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "direct.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	var setErr error
	if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
		doc, err := dom.GetDocument().Do(ctx)
		if err != nil {
			return err
		}
		nodeID, err := dom.QuerySelector(doc.NodeID, `input[type=file]`).Do(ctx)
		if err != nil {
			return err
		}
		setErr = dom.SetFileInputFiles([]string{path}).WithNodeID(nodeID).Do(ctx)
		return nil
	})); e != nil {
		t.Fatal(e)
	}
	t.Logf("direct set err=%v", setErr)
	var files int
	_ = b.run(chromedp.Evaluate(`(()=>{const i=document.querySelector('input[type=file]');return i&&i.files?i.files.length:-1})()`, &files))
	t.Logf("files after set=%d", files)
	if files > 0 {
		_ = b.run(chromedp.Evaluate(`(()=>{const i=document.querySelector('input[type=file]');i.dispatchEvent(new Event('input',{bubbles:true}));i.dispatchEvent(new Event('change',{bubbles:true}));return true})()`, new(bool)))
	}
	time.Sleep(10 * time.Second)
	var state map[string]any
	_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length, files:(()=>{const i=document.querySelector('input[type=file]');return i&&i.files?i.files.length:-1})()}))()`, &state))
	t.Logf("state=%v", state)
}

func TestLiveGroupFrames(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_FRAMES") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	var mu sync.Mutex
	contexts := map[int64]string{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*runtime.EventExecutionContextCreated); ok {
			mu.Lock()
			contexts[int64(e.Context.ID)] = e.Context.Origin
			mu.Unlock()
		}
	})
	if e = b.run(runtime.Enable()); e != nil {
		t.Fatal(e)
	}
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	_ = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery))
	time.Sleep(time.Second)
	var uploadEls []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"],div,span')].filter(e => (e.innerText||'').trim() === 'Upload').map(e => { const r=e.getBoundingClientRect(); return {x:r.x,y:r.y,w:r.width,h:r.height}; }))()`, &uploadEls))
	if len(uploadEls) > 0 {
		_ = b.run(chromedp.MouseClickXY(uploadEls[0]["x"].(float64)+uploadEls[0]["w"].(float64)/2, uploadEls[0]["y"].(float64)+uploadEls[0]["h"].(float64)/2))
	}
	time.Sleep(2 * time.Second)
	mu.Lock()
	copyContexts := map[int64]string{}
	for id, v := range contexts {
		copyContexts[id] = v
	}
	mu.Unlock()
	for id, origin := range copyContexts {
		var count int
		e := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			result, _, err := runtime.Evaluate(`document.querySelectorAll('input[type=file]').length`).WithContextID(runtime.ExecutionContextID(id)).WithReturnByValue(true).Do(ctx)
			if err == nil && len(result.Value) > 0 {
				_ = json.Unmarshal(result.Value, &count)
			}
			return err
		}))
		t.Logf("ctx=%d origin=%s files=%d err=%v", id, origin, count, e)
	}
}

func TestLiveGroupViaOneToOne(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_VIA_ONE") != "1" {
		t.Skip("explicit opt-in; no send")
	}
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
	if e = b.openRecipient("+13852679447"); e != nil {
		t.Fatal(e)
	}
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "one.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	if e = b.attach([]string{path}); e != nil {
		t.Fatal("attach", e)
	}
	var opts []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"]')].filter(b=>(b.getAttribute('aria-label')||'')==='More options').map(b=>{const r=b.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height};}))()`, &opts))
	t.Logf("moreOptions=%v", opts)
	if len(opts) > 0 {
		best := opts[0]
		for _, o := range opts {
			if o["y"].(float64) < best["y"].(float64) {
				best = o
			}
		}
		_ = b.run(chromedp.MouseClickXY(best["x"].(float64)+best["w"].(float64)/2, best["y"].(float64)+best["h"].(float64)/2))
		time.Sleep(time.Second)
		var menu []string
		_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="menuitem"],mat-menu-item,[role="button"]')].map(b=>(b.getAttribute('aria-label')||b.innerText||'').trim()).filter(Boolean).slice(0,40))()`, &menu))
		t.Logf("menu=%v", menu)
	}
}

func TestLiveGroupContextUpload(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_CONTEXT") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	var mu sync.Mutex
	contexts := map[int64]string{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*runtime.EventExecutionContextCreated); ok {
			mu.Lock()
			contexts[int64(e.Context.ID)] = e.Context.Origin
			mu.Unlock()
		}
	})
	if e = b.run(runtime.Enable()); e != nil {
		t.Fatal(e)
	}
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	if e = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Second)
	var uploadEls []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"],div,span')].filter(e => (e.innerText||'').trim() === 'Upload').map(e => { const r=e.getBoundingClientRect(); return {x:r.x,y:r.y,w:r.width,h:r.height}; }))()`, &uploadEls))
	if len(uploadEls) == 0 {
		t.Fatal("no Upload menu item")
	}
	_ = b.run(chromedp.MouseClickXY(uploadEls[0]["x"].(float64)+uploadEls[0]["w"].(float64)/2, uploadEls[0]["y"].(float64)+uploadEls[0]["h"].(float64)/2))
	time.Sleep(2 * time.Second)
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "context.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	mu.Lock()
	copyContexts := map[int64]string{}
	for id, v := range contexts {
		copyContexts[id] = v
	}
	mu.Unlock()
	for id, origin := range copyContexts {
		var count int
		e := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			result, _, err := runtime.Evaluate(`document.querySelectorAll('input[type=file]').length`).WithContextID(runtime.ExecutionContextID(id)).WithReturnByValue(true).Do(ctx)
			if err == nil && len(result.Value) > 0 {
				_ = json.Unmarshal(result.Value, &count)
			}
			return err
		}))
		if e != nil || count == 0 {
			continue
		}
		t.Logf("upload context=%d origin=%s", id, origin)
		var setErr error
		_ = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			result, _, err := runtime.Evaluate(`document.querySelector('input[type=file]')`).WithContextID(runtime.ExecutionContextID(id)).Do(ctx)
			if err != nil {
				return err
			}
			if err = dom.SetFileInputFiles([]string{path}).WithObjectID(result.ObjectID).Do(ctx); err != nil {
				return err
			}
			check, _, _ := runtime.CallFunctionOn(`function(){return this.files?this.files.length:-1}`).WithObjectID(result.ObjectID).WithReturnByValue(true).Do(ctx)
			t.Logf("context files=%s", check.Value)
			_, _, _ = runtime.CallFunctionOn(`function(){this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}));return true}`).WithObjectID(result.ObjectID).WithUserGesture(true).Do(ctx)
			return nil
		}))
		t.Logf("set err=%v", setErr)
		time.Sleep(10 * time.Second)
		var state map[string]any
		_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length}))()`, &state))
		t.Logf("state=%v", state)
	}
}

func TestLiveGroupChooserFixed(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_FIXED") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	var mu sync.Mutex
	contexts := map[string]int64{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*runtime.EventExecutionContextCreated); ok {
			var aux struct {
				FrameID   string `json:"frameId"`
				IsDefault bool   `json:"isDefault"`
			}
			if json.Unmarshal(e.Context.AuxData, &aux) == nil && aux.FrameID != "" {
				mu.Lock()
				if aux.IsDefault || contexts[aux.FrameID] == 0 {
					contexts[aux.FrameID] = int64(e.Context.ID)
				}
				mu.Unlock()
			}
		}
	})
	if e = b.run(runtime.Enable()); e != nil {
		t.Fatal(e)
	}
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	if e = b.run(page.SetInterceptFileChooserDialog(true)); e != nil {
		t.Fatal(e)
	}
	type chooserEvent struct {
		node  cdp.BackendNodeID
		frame cdp.FrameID
	}
	chooser := make(chan chooserEvent, 1)
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*page.EventFileChooserOpened); ok {
			select {
			case chooser <- chooserEvent{e.BackendNodeID, e.FrameID}:
			default:
			}
		}
	})
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "fixed.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	if e = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Second)
	var uploadEls []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"],div,span')].filter(e => (e.innerText||'').trim() === 'Upload').map(e => { const r=e.getBoundingClientRect(); return {x:r.x,y:r.y,w:r.width,h:r.height}; }))()`, &uploadEls))
	if len(uploadEls) == 0 {
		t.Fatal("no Upload item")
	}
	_ = b.run(chromedp.MouseClickXY(uploadEls[0]["x"].(float64)+uploadEls[0]["w"].(float64)/2, uploadEls[0]["y"].(float64)+uploadEls[0]["h"].(float64)/2))
	var event chooserEvent
	select {
	case event = <-chooser:
	case <-time.After(10 * time.Second):
		t.Fatal("no chooser")
	}
	mu.Lock()
	contextID := contexts[string(event.frame)]
	mu.Unlock()
	t.Logf("frame=%s context=%d", event.frame, contextID)
	var files int
	_tmp, _ := chromedp.Targets(b.ctx)
	for _, tg := range _tmp {
		if tg.Type != "iframe" {
			continue
		}
		tctx, tcancel := chromedp.NewContext(b.ctx, chromedp.WithTargetID(tg.TargetID))
		e := chromedp.Run(tctx, chromedp.ActionFunc(func(ctx context.Context) error {
			return dom.SetFileInputFiles([]string{path}).WithBackendNodeID(event.node).Do(ctx)
		}))
		tcancel()
		t.Logf("iframe target %s set err=%v", tg.URL, e)
	}
	_ = b.run(chromedp.Evaluate(`(()=>{const i=document.querySelector('input[type=file]');return i&&i.files?i.files.length:-1})()`, &files))
	time.Sleep(8 * time.Second)
	var state map[string]any
	_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length}))()`, &state))
	t.Logf("files=%d state=%v", files, state)
}

func TestLiveGroupChooserContexts(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUP_CONTEXTS") != "1" {
		t.Skip("explicit opt-in; no send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	groupID := ""
	for _, m := range store.rows {
		if m.GroupID != "" && len(m.Participants) == 2 {
			groupID = m.GroupID
			break
		}
	}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	var mu sync.Mutex
	ids := []int64{}
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*runtime.EventExecutionContextCreated); ok {
			mu.Lock()
			ids = append(ids, int64(e.Context.ID))
			mu.Unlock()
		}
	})
	if e = b.run(runtime.Enable()); e != nil {
		t.Fatal(e)
	}
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.run(chromedp.Poll(`(()=>{const el=document.querySelector('[data-store-key='+JSON.stringify(`+literal(groupID)+`)+']');if(!el)return false;(el.querySelector('a,[role="button"],button')||el).click();return true})()`, new(bool), chromedp.WithPollingTimeout(20*time.Second))); e != nil {
		t.Fatal(e)
	}
	time.Sleep(2 * time.Second)
	if e = b.run(page.SetInterceptFileChooserDialog(true)); e != nil {
		t.Fatal(e)
	}
	chooser := make(chan cdp.BackendNodeID, 1)
	chromedp.ListenTarget(b.ctx, func(event any) {
		if e, ok := event.(*page.EventFileChooserOpened); ok {
			select {
			case chooser <- e.BackendNodeID:
			default:
			}
		}
	})
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII=")
	path := filepath.Join(".obj", "contexts.png")
	_ = os.WriteFile(path, image, 0600)
	defer os.Remove(path)
	if e = b.run(chromedp.Click(`button[aria-label="Add image"]`, chromedp.ByQuery)); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Second)
	var uploadEls []map[string]any
	_ = b.run(chromedp.Evaluate(`(() => [...document.querySelectorAll('button,[role="button"],div,span')].filter(e => (e.innerText||'').trim() === 'Upload').map(e => { const r=e.getBoundingClientRect(); return {x:r.x,y:r.y,w:r.width,h:r.height}; }))()`, &uploadEls))
	if len(uploadEls) == 0 {
		t.Fatal("no Upload item")
	}
	_ = b.run(chromedp.MouseClickXY(uploadEls[0]["x"].(float64)+uploadEls[0]["w"].(float64)/2, uploadEls[0]["y"].(float64)+uploadEls[0]["h"].(float64)/2))
	var node cdp.BackendNodeID
	select {
	case node = <-chooser:
	case <-time.After(10 * time.Second):
		t.Fatal("no chooser")
	}
	mu.Lock()
	copyIDs := append([]int64(nil), ids...)
	mu.Unlock()
	for _, id := range copyIDs {
		var files int
		_ = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			object, err := dom.ResolveNode().WithBackendNodeID(node).WithExecutionContextID(runtime.ExecutionContextID(id)).Do(ctx)
			if err != nil {
				return err
			}
			if err = dom.SetFileInputFiles([]string{path}).WithObjectID(object.ObjectID).Do(ctx); err != nil {
				return err
			}
			result, _, err := runtime.CallFunctionOn(`function(){return this.files?this.files.length:-1}`).WithObjectID(object.ObjectID).WithReturnByValue(true).Do(ctx)
			if err == nil && len(result.Value) > 0 {
				_ = json.Unmarshal(result.Value, &files)
			}
			return nil
		}))
		t.Logf("ctx=%d files=%d", id, files)
		if files > 0 {
			_ = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
				object, err := dom.ResolveNode().WithBackendNodeID(node).WithExecutionContextID(runtime.ExecutionContextID(id)).Do(ctx)
				if err != nil {
					return err
				}
				_, _, _ = runtime.CallFunctionOn(`function(){this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}));return true}`).WithObjectID(object.ObjectID).WithUserGesture(true).Do(ctx)
				return nil
			}))
			break
		}
	}
	time.Sleep(8 * time.Second)
	var state map[string]any
	_ = b.run(chromedp.Evaluate(`(() => ({buttons:[...document.querySelectorAll('gv-message-entry button')].map(b=>({label:b.getAttribute('aria-label')||'',dis:b.disabled})), media: document.querySelectorAll('gv-message-entry img').length}))()`, &state))
	t.Logf("state=%v", state)
}
