package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestGatewaySMSMMSAndBinaryAttachment(t *testing.T) {
	var lock sync.Mutex
	posts := []map[string]any{}
	inboxQuery := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "u" || password != "p" {
			t.Error("missing auth")
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/message":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			lock.Lock()
			posts = append(posts, body)
			lock.Unlock()
			w.WriteHeader(202)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": body["id"], "state": "Pending", "recipients": []map[string]string{{"phoneNumber": "+12025550123", "state": "Pending"}}})
		case r.URL.Path == "/inbox":
			inboxQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "mms:1", "sender": "+12025550123", "type": "MMS_DOWNLOADED", "contentPreview": "image text", "attachments": []map[string]any{{"partId": 7, "name": "p.png", "contentType": "image/png", "size": 3}}}})
		case r.URL.Path == "/messages":
			w.Write([]byte("[]"))
		case r.URL.Path == "/inbox/mms:1/attachments/7":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{1, 2, 3})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	gateway := newGateway(Config{Server: server.URL, Username: "u", Password: "p", Timeout: 5})
	ctx := context.Background()
	if _, err := gateway.send(ctx, "sms-id", "+12025550123", "no preface"); err != nil {
		t.Fatal(err)
	}
	parts := []WireAttachment{{"image/png", "p.png", []byte{1, 2, 3}}}
	messages, err := gateway.sendMMS(ctx, "mms-id", "+12025550123", "MMS text", "Optional title", parts)
	if err != nil {
		t.Fatal(err)
	}
	if messages[0].Type != "MMS" || len(messages[0].Media) != 1 {
		t.Fatal("MMS metadata missing")
	}
	for i, body := range posts {
		_, hasSMS := body["textMessage"]
		_, hasMMS := body["mmsMessage"]
		if hasSMS == hasMMS {
			t.Fatal("payload variants combined")
		}
		if i == 0 && body["textMessage"].(map[string]any)["text"] != "no preface" {
			t.Fatal("message changed")
		}
	}
	wire := posts[1]["mmsMessage"].(map[string]any)["attachments"].([]any)[0].(map[string]any)
	if posts[1]["mmsMessage"].(map[string]any)["subject"] != "Optional title" || messages[0].Subject != "Optional title" {
		t.Fatal("MMS subject lost")
	}
	if wire["data"] != "AQID" || wire["contentType"] != "image/png" {
		t.Fatal("incorrect MMS encoding")
	}
	incoming, err := gateway.sync(ctx, map[string]Message{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inboxQuery, "includeAttachments=true") || strings.Contains(inboxQuery, "type=SMS") {
		t.Fatal("MMS filtered out")
	}
	if len(incoming) != 1 || len(incoming[0].Media) != 1 {
		t.Fatal("incoming MMS absent")
	}
	data, err := gateway.download(ctx, "mms:1", incoming[0].Media[0])
	if err != nil || !reflect.DeepEqual(data, []byte{1, 2, 3}) {
		t.Fatal("download failed", err)
	}
}

func TestLostSendResponseRecoversWithoutSecondPOST(t *testing.T) {
	var message APIMessage
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			posts++
			message = APIMessage{ID: body["id"].(string), State: "Delivered", Recipients: []APIRecipient{{"+12025550123", "Delivered"}}}
			message.Body = &struct {
				Text string `json:"text"`
			}{body["textMessage"].(map[string]any)["text"].(string)}
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
		case r.URL.Path == "/inbox":
			w.Write([]byte("[]"))
		case r.URL.Path == "/messages":
			json.NewEncoder(w).Encode([]APIMessage{message})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	a := testApp(t)
	a.Demo = false
	a.Config = Config{Server: server.URL, Username: "u", Password: "p", Poll: 10, Timeout: 5}
	a.Gateway = newGateway(a.Config)
	a.openThread("+12025550123")
	a.key(13, 12)
	typeKeys(a, "exact test")
	a.key(13, 12)
	if a.Phase != "review" || posts != 0 {
		t.Fatal("Enter did not finish into review")
	}
	a.key(13, 12)
	settle(t, a)
	if posts != 1 {
		t.Fatal("send was retried", posts)
	}
	id := a.Sending.ID
	if a.Cache.records["out:"+id+":"+a.Number].State != "Uncertain" {
		t.Fatal("ambiguous send falsely confirmed")
	}
	a.sync()
	settle(t, a)
	if posts != 1 || a.Cache.records["out:"+id+":"+a.Number].State != "Delivered" {
		t.Fatal("reconciliation failed or resent")
	}
}

func TestImportSuppressesWebhooksAndIncludesMMS(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(202)
	}))
	defer server.Close()
	err := newGateway(Config{Server: server.URL, Timeout: 5}).importHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if body["webhookDelivery"] != "Disabled" || body["triggerWebhooks"] != false {
		t.Fatal("webhooks enabled")
	}
	if !reflect.DeepEqual(body["messageTypes"], []any{"SMS", "MMS"}) {
		t.Fatal("MMS missing from history import")
	}
}
func TestOutgoingMMSCacheCanOpenWithoutGateway(t *testing.T) {
	a := testApp(t)
	part := Attachment{Name: "photo.png", ContentType: "image/png", Inline: []byte{4, 5, 6}, Size: 3}
	message := Message{ID: "mms", Number: "+12025550123", Direction: "out", Type: "MMS", Media: []Attachment{part}}
	a.Cache.path = filepath.Join(t.TempDir(), "profile.sqlite3")
	if err := a.cacheMessages([]Message{message}); err != nil {
		t.Fatal(err)
	}
	stored := a.Cache.records[message.key()]
	if stored.Media[0].Local == "" || len(stored.Media[0].Inline) > 0 {
		t.Fatal("outgoing media not cached separately")
	}
	data, err := attachmentBytes(context.Background(), nil, attachmentChoice{stored, stored.Media[0]})
	if err != nil || !reflect.DeepEqual(data, []byte{4, 5, 6}) {
		t.Fatal("cached attachment unavailable")
	}
	if !isPrivateFile(stored.Media[0].Local) {
		t.Fatal("attachment permissions")
	}
}
