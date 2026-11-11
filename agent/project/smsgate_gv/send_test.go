package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestSendCorrelationAndRecovery(t *testing.T) {
	account, number := "+15555550000", "+15555550123"
	pending := Message{ID: "client-id", Account: account, Submitted: true, SendToken: sendToken(account, "client-id"), Body: &Text{Text: "same\n中文"}, Recipients: []Recipient{{number, "Processed"}}, State: "Processed", Created: "2026-11-02T00:00:00Z"}
	request := `[null,null,null,null,"same\n中文","t.+15555550123",null,null,[123],null,["opaque-app-proof"]]`
	prepared, e := prepareSendPayload([]byte(request), pending)
	if e != nil {
		t.Fatal(e)
	}
	var fields []json.RawMessage
	if e = json.Unmarshal(prepared, &fields); e != nil {
		t.Fatal(e)
	}
	if string(fields[10]) != `["opaque-app-proof"]` || !strings.Contains(string(fields[8]), pending.SendToken) {
		t.Fatal("changed proof or lost journal token")
	}
	wrong := pending
	wrong.Recipients = []Recipient{{"+15555550999", "Processed"}}
	if _, e = prepareSendPayload([]byte(request), wrong); e == nil {
		t.Fatal("wrong recipient accepted")
	}
	wrong = pending
	wrong.Body = &Text{Text: "different"}
	if _, e = prepareSendPayload([]byte(request), wrong); e == nil {
		t.Fatal("wrong text accepted")
	}
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = store.put(pending); e != nil {
		t.Fatal(e)
	}
	store, e = openStore(filepath.Dir(store.path))
	if e != nil {
		t.Fatal(e)
	}
	got, _ := store.get(pending.ID)
	if got.State != "Uncertain" {
		t.Fatal("interrupted send not marked uncertain")
	}
	imported := Message{ID: nativeBridgeID(account, "native-id"), Source: "native-id", Account: account, SendToken: pending.SendToken, Body: pending.Body, Recipients: []Recipient{{number, "Sent"}}, State: "Sent", Created: "2026-11-02T00:00:01.123Z"}
	if e = store.mergeImports([]Message{imported}); e != nil {
		t.Fatal(e)
	}
	got, ok := store.get(pending.ID)
	if !ok || got.State != "Sent" || got.Source != "native-id" || len(store.rows) != 1 || !got.Submitted {
		t.Fatal("token reconciliation lost identity or duplicated send")
	}
	if _, fresh, e := store.reserve(pending); e != nil || fresh {
		t.Fatal("duplicate submission would be sent again")
	}
	// Identical text without the exact token must never confirm an uncertain send.
	pending.ID = "another-client-id"
	pending.SendToken = sendToken(account, pending.ID)
	if e = store.put(pending); e != nil {
		t.Fatal(e)
	}
	if e = store.mergeImports([]Message{imported}); e != nil {
		t.Fatal(e)
	}
	got, _ = store.get(pending.ID)
	if got.State != "Processed" {
		t.Fatal("same text incorrectly reconciled")
	}
	queued := pending
	queued.ID = "never-dispatched"
	queued.State = "Pending"
	if e = store.put(queued); e != nil {
		t.Fatal(e)
	}
	store, e = openStore(filepath.Dir(store.path))
	if e != nil {
		t.Fatal(e)
	}
	got, _ = store.get(queued.ID)
	if got.State != "Failed" {
		t.Fatal("queued message should not be retried on restart")
	}
}

func TestLiveNewRecipientPreparation(t *testing.T) {
	if os.Getenv("GV_LIVE_PREPARE") != "1" {
		t.Skip("explicit opt-in; no sends")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	number := os.Getenv("GV_REMOTE_NUMBER")
	if !phonePattern.MatchString(number) {
		t.Fatal("explicit number required")
	}
	c.SyncNumbers = []string{number}
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	if e = b.openNewRecipient(number); e != nil {
		var shape json.RawMessage
		_ = b.run(chromedp.Evaluate(`(()=>({inputs:[...document.querySelectorAll('input,textarea')].map(e=>({tag:e.tagName,placeholder:e.getAttribute('placeholder'),aria:e.getAttribute('aria-label')})),options:document.querySelector('.cdk-overlay-container')?.outerHTML,details:document.querySelector('gv-thread-details')?.outerHTML}))()`, &shape))
		_ = writeJSON(filepath.Join(".obj", "new_recipient_probe.json"), shape)
		t.Fatal(e)
	}
	t.Log("new-message recipient path verified independently; no text entered or sent")
}

func TestStoreSnapshotIsolation(t *testing.T) {
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	message := Message{ID: "snapshot", State: "Processed", Body: &Text{Text: "original"}, Recipients: []Recipient{{"+15555550123", "Processed"}}}
	if e = store.put(message); e != nil {
		t.Fatal(e)
	}
	message.Body.Text = "caller changed"
	message.Recipients[0].State = "Uncertain"
	first, _ := store.get(message.ID)
	if first.Body.Text != "original" || first.Recipients[0].State != "Processed" {
		t.Fatal("caller mutated committed journal")
	}
	first.Body.Text = "reader changed"
	first.Recipients[0].State = "Sent"
	second, _ := store.get(message.ID)
	if second.Body.Text != "original" || second.Recipients[0].State != "Processed" {
		t.Fatal("reader mutated journal")
	}
	rows := store.list(false)
	rows[0].Recipients[0].State = "Failed"
	second, _ = store.get(message.ID)
	if second.Recipients[0].State != "Processed" {
		t.Fatal("listing mutated journal")
	}
}
