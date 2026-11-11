package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookRegistrationValidation(t *testing.T) {
	home := t.TempDir()
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	hooks, e := openWebhooks(home)
	if e != nil {
		t.Fatal(e)
	}
	bridge := &Bridge{config: Config{Username: "user", Password: "private"}, store: store, home: home, webhooks: hooks}
	do := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, stringsReader(body))
		request.SetBasicAuth("user", "private")
		recorder := httptest.NewRecorder()
		bridge.ServeHTTP(recorder, request)
		return recorder
	}
	if r := do("POST", "/webhooks", `{"id":"a","url":"http://127.0.0.1:1/hook","event":"sms:received"}`); r.Code != 201 {
		t.Fatalf("register status=%d body=%s", r.Code, r.Body.String())
	}
	if r := do("POST", "/webhooks", `{"id":"a","url":"http://127.0.0.1:1/hook","event":"sms:received"}`); r.Code != 200 {
		t.Fatalf("idempotent register status=%d", r.Code)
	}
	if r := do("POST", "/webhooks", `{"id":"a","url":"http://127.0.0.1:2/hook","event":"sms:received"}`); r.Code != 409 {
		t.Fatalf("conflict status=%d", r.Code)
	}
	if r := do("POST", "/webhooks", `{"id":"bad","url":"ftp://x/y","event":"sms:received"}`); r.Code != 400 {
		t.Fatalf("invalid URL status=%d", r.Code)
	}
	if r := do("POST", "/webhooks", `{"id":"bad","url":"http://x/y","event":"nope"}`); r.Code != 400 {
		t.Fatalf("invalid event status=%d", r.Code)
	}
	if r := do("GET", "/webhooks", ""); r.Code != 200 {
		t.Fatalf("list status=%d", r.Code)
	}
	reopened, e := openWebhooks(home)
	if e != nil || len(reopened.list()) != 1 {
		t.Fatalf("persistence failed: %v %d", e, len(reopened.list()))
	}
	if r := do("DELETE", "/webhooks/a", ""); r.Code != 204 {
		t.Fatalf("delete status=%d", r.Code)
	}
	if r := do("DELETE", "/webhooks/a", ""); r.Code != 404 {
		t.Fatalf("repeat delete status=%d", r.Code)
	}
}

func TestWebhookDelivery(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- body
		w.WriteHeader(200)
	}))
	defer server.Close()
	bridge := &Bridge{webhookClient: &http.Client{Timeout: 5 * time.Second}}
	hook := Webhook{ID: "a", URL: server.URL, Event: "sms:received"}
	bridge.postWebhook(hook, "sms:received", map[string]any{"messageId": "gv-1", "message": "hello"})
	select {
	case body := <-received:
		if body["event"] != "sms:received" || body["id"] == "" {
			t.Fatalf("body=%v", body)
		}
		payload, _ := body["payload"].(map[string]any)
		if payload["messageId"] != "gv-1" {
			t.Fatalf("payload=%v", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook not delivered")
	}
}

func TestMessageWebhookEvents(t *testing.T) {
	if event, _, ok := messageWebhook(Message{ID: "x", State: "Received", Sender: "+15551110000", Type: "SMS", Preview: "hi"}); !ok || event != "sms:received" {
		t.Fatalf("sms event=%s ok=%v", event, ok)
	}
	if event, _, ok := messageWebhook(Message{ID: "x", State: "Received", Sender: "+15551110000", Type: "MMS"}); !ok || event != "mms:received" {
		t.Fatalf("mms event=%s ok=%v", event, ok)
	}
	if event, _, ok := messageWebhook(Message{ID: "x", State: "Sent", Submitted: true}); !ok || event != "sms:sent" {
		t.Fatalf("sent event=%s ok=%v", event, ok)
	}
	if event, _, ok := messageWebhook(Message{ID: "x", State: "Failed", Submitted: true}); !ok || event != "sms:failed" {
		t.Fatalf("failed event=%s ok=%v", event, ok)
	}
	if _, _, ok := messageWebhook(Message{ID: "x", State: "Pending"}); ok {
		t.Fatal("pending message emitted event")
	}
}

func stringsReader(value string) io.Reader { return strings.NewReader(value) }
