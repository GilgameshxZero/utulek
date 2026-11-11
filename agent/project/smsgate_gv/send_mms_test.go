package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestSubmitMMSQueuesAttachments(t *testing.T) {
	home := t.TempDir()
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	bridge := &Bridge{config: Config{Username: "user", Password: "private", Number: "+15555550000"}, store: store, home: home, jobs: make(chan string, 4), ready: true}
	body, _ := json.Marshal(map[string]any{
		"id":           "mms-test",
		"phoneNumbers": []string{"+13852679447"},
		"mmsMessage": map[string]any{
			"text":    "photo",
			"subject": "test",
			"attachments": []map[string]any{{
				"contentType": "image/png",
				"name":        "p.png",
				"data":        base64.StdEncoding.EncodeToString([]byte{1, 2, 3}),
			}},
		},
	})
	request := httptest.NewRequest("POST", "/message", bytes.NewReader(body))
	request.SetBasicAuth("user", "private")
	recorder := httptest.NewRecorder()
	bridge.ServeHTTP(recorder, request)
	if recorder.Code != 202 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	message, ok := store.get("mms-test")
	if !ok || message.Type != "MMS" || message.Body == nil || message.Body.Text != "photo" || len(message.Attachments) != 1 {
		t.Fatalf("message=%+v", message)
	}
	attachment := message.Attachments[0]
	if attachment.ContentType != "image/png" || attachment.Name != "p.png" || len(attachment.Inline) != 3 {
		t.Fatalf("attachment=%+v", attachment)
	}
	if len(bridge.jobs) != 1 {
		t.Fatal("message was not queued")
	}
}

func TestSubmitMMSRejectsUnsupportedMedia(t *testing.T) {
	home := t.TempDir()
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	bridge := &Bridge{config: Config{Username: "user", Password: "private", Number: "+15555550000"}, store: store, home: home, jobs: make(chan string, 4), ready: true}
	body, _ := json.Marshal(map[string]any{
		"phoneNumbers": []string{"+13852679447"},
		"mmsMessage": map[string]any{
			"text": "x",
			"attachments": []map[string]any{{
				"contentType": "application/pdf",
				"name":        "x.pdf",
				"data":        base64.StdEncoding.EncodeToString([]byte{1}),
			}},
		},
	})
	request := httptest.NewRequest("POST", "/message", bytes.NewReader(body))
	request.SetBasicAuth("user", "private")
	recorder := httptest.NewRecorder()
	bridge.ServeHTTP(recorder, request)
	if recorder.Code != 400 {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestPrepareGroupSendPayload(t *testing.T) {
	fields := make([]any, 11)
	fields[4] = "group text"
	fields[6] = []string{"+1 (385) 267-9447", "+1 (850) 888-8724"}
	fields[8] = []uint64{1}
	body, _ := json.Marshal(fields)
	m := Message{Body: &Text{Text: "group text"}, SendToken: "42", Recipients: []Recipient{{Number: "+13852679447"}, {Number: "+18508888724"}}}
	prepared, e := prepareSendPayload(body, m)
	if e != nil {
		t.Fatal(e)
	}
	var out []json.RawMessage
	if json.Unmarshal(prepared, &out) != nil || len(out) != 11 {
		t.Fatal("prepared payload invalid")
	}
	fields[6] = []string{"+1 (385) 267-9447"}
	body, _ = json.Marshal(fields)
	if _, e = prepareSendPayload(body, m); e == nil {
		t.Fatal("recipient mismatch accepted")
	}
}

func TestSubmitGroupMMSQueues(t *testing.T) {
	home := t.TempDir()
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	bridge := &Bridge{config: Config{Username: "user", Password: "private", Number: "+15555550000"}, store: store, home: home, jobs: make(chan string, 4), ready: true}
	body, _ := json.Marshal(map[string]any{
		"phoneNumbers": []string{"+13852679447", "+18508888724"},
		"mmsMessage": map[string]any{
			"text": "group photo",
			"attachments": []map[string]any{{
				"contentType": "image/png",
				"name":        "p.png",
				"data":        base64.StdEncoding.EncodeToString([]byte{1, 2, 3}),
			}},
		},
	})
	request := httptest.NewRequest("POST", "/message", bytes.NewReader(body))
	request.SetBasicAuth("user", "private")
	recorder := httptest.NewRecorder()
	bridge.ServeHTTP(recorder, request)
	if recorder.Code != 202 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var queued Message
	if e := json.Unmarshal(recorder.Body.Bytes(), &queued); e != nil || len(queued.Recipients) != 2 || len(queued.Attachments) != 1 {
		t.Fatalf("queued=%+v err=%v", queued, e)
	}
}
