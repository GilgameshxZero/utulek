package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeAttachmentMetadata(t *testing.T) {
	record := `["native-m",1793545877539,"+15555550000",[],10,1,null,null,null,"caption",null,null,5,1,["caption","",[["image/jpeg","source-1",1,[[1,900,675],[2,128,96]],0,null,null,"token"]]]]`
	thread := fixtureThread(record)
	rows, e := decodeVoiceThread(thread, "+15555550000")
	if e != nil || len(rows) != 1 || len(rows[0].Attachments) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, e)
	}
	attachment := rows[0].Attachments[0]
	if attachment.ID != "source-1" || attachment.ContentType != "image/jpeg" || attachment.Width != 900 || attachment.Height != 675 {
		t.Fatalf("attachment=%+v", attachment)
	}
	body, _ := json.Marshal([]any{[]any{thread}})
	messages, e := decodeHistory(body, Config{Number: "+15555550000", MaxThreads: 10})
	if e != nil || len(messages) != 1 || len(messages[0].Attachments) != 1 {
		t.Fatalf("messages=%+v err=%v", messages, e)
	}
	bridgeAttachment := messages[0].Attachments[0]
	if bridgeAttachment.PartID != 1 || bridgeAttachment.SourceID != "source-1" || bridgeAttachment.ContentType != "image/jpeg" || bridgeAttachment.Name != "attachment-1.jpg" {
		t.Fatalf("bridge attachment=%+v", bridgeAttachment)
	}
}

func TestPruneMediaCacheRemovesOldestFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for i, name := range []string{"old", "middle", "new"} {
		path := filepath.Join(dir, name)
		if e := os.WriteFile(path, make([]byte, 10), 0600); e != nil {
			t.Fatal(e)
		}
		stamp := now.Add(time.Duration(i-3) * time.Hour)
		if e := os.Chtimes(path, stamp, stamp); e != nil {
			t.Fatal(e)
		}
	}
	if e := pruneMediaCache(dir, 20); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 2 {
		t.Fatalf("entries=%d err=%v", len(entries), e)
	}
	if _, e := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(e) {
		t.Fatal("oldest file was not removed")
	}
	for _, name := range []string{"middle", "new"} {
		if _, e := os.Stat(filepath.Join(dir, name)); e != nil {
			t.Fatalf("%s removed", name)
		}
	}
	if e := pruneMediaCache(filepath.Join(dir, "missing"), 1); e != nil {
		t.Fatal(e)
	}
}

func TestAttachmentEndpointServesCachedBytes(t *testing.T) {
	home := t.TempDir()
	store, e := openStore(home)
	if e != nil {
		t.Fatal(e)
	}
	attachment := Attachment{PartID: 1, Name: "attachment-1.jpg", ContentType: "image/jpeg", SourceID: "source-1"}
	message := Message{ID: "gv-1", Account: "+15555550000", Sender: "+15555550123", Created: "2026-11-01T00:00:00Z", Preview: "photo", Type: "MMS", Attachments: []Attachment{attachment}}
	if e = store.put(message); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(home, "media", cacheName(attachment.SourceID))
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte{1, 2, 3}, 0600); e != nil {
		t.Fatal(e)
	}
	bridge := &Bridge{config: Config{Username: "user", Password: "private"}, store: store, home: home}
	request := httptest.NewRequest("GET", "/inbox/gv-1/attachments/1", nil)
	request.SetBasicAuth("user", "private")
	recorder := httptest.NewRecorder()
	bridge.ServeHTTP(recorder, request)
	if recorder.Code != 200 || recorder.Header().Get("Content-Type") != "image/jpeg" || recorder.Body.Len() != 3 {
		t.Fatalf("status=%d type=%q body=%d", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Len())
	}
	request = httptest.NewRequest("GET", "/inbox/gv-1/attachments/9", nil)
	request.SetBasicAuth("user", "private")
	recorder = httptest.NewRecorder()
	bridge.ServeHTTP(recorder, request)
	if recorder.Code != 404 {
		t.Fatalf("missing part status=%d", recorder.Code)
	}
}
