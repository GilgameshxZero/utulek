package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPushIncomingNeedsNoFetch(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Error(w, "unexpected fetch", 500) }))
	defer server.Close()
	gateway := newGateway(Config{Server: server.URL, Timeout: 3})
	for _, tc := range []struct{ event, id, transaction, want string }{
		{"sms:received", "ffffffff", "", "text:-1"},
		{"mms:received", "carrier-id", "abc", "mms-header:96354"},
		{"mms:downloaded", "123", "", "mms:48690"},
	} {
		var e pushEvent
		e.Event = tc.event
		e.Payload.ID = tc.id
		e.Payload.TransactionID = tc.transaction
		e.Payload.Sender = "+12025550123"
		e.Payload.Received = "2026-11-01T00:00:00Z"
		e.Payload.Message = "full text"
		e.Payload.Body = "caption"
		e.Payload.Attachments = []pushPart{{Attachment: Attachment{PartID: 7, Name: "image.png"}, Data: []byte("image bytes")}}
		got, err := gateway.eventMessages(context.Background(), []pushEvent{e})
		if err != nil || len(got) != 1 || got[0].ID != tc.want {
			t.Fatalf("%s identity mismatch: %v %v", tc.event, got, err)
		}
		if tc.event == "mms:downloaded" && string(got[0].Media[0].Inline) != "image bytes" {
			t.Fatal("webhook media was discarded")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("incoming events fetched the gateway")
	}
}
func TestIncrementalSyncBoundsHistoryAndResolvesPending(t *testing.T) {
	var incoming, outgoing, detail atomic.Int32
	since := time.Date(2026, 11, 1, 1, 0, 0, 123456789, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/inbox", "/messages":
			if r.URL.Query().Get("from") != since.Add(-5*time.Minute).Format(time.RFC3339) {
				t.Error("missing incremental time bound")
			}
			if r.URL.Path == "/inbox" {
				incoming.Add(1)
			} else {
				outgoing.Add(1)
			}
			fmt.Fprint(w, "[]")
		case "/message/old-pending":
			detail.Add(1)
			fmt.Fprint(w, `{"id":"old-pending","state":"Delivered","recipients":[{"phoneNumber":"+12025550123","state":"Delivered"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	g := newGateway(Config{Server: server.URL, Timeout: 3})
	known := map[string]Message{"old-pending": {ID: "old-pending", Number: "+12025550123", Direction: "out", Text: "retained", State: "Pending"}}
	messages, err := g.syncSince(context.Background(), known, since)
	if err != nil || incoming.Load() != 1 || outgoing.Load() != 1 || detail.Load() != 1 || len(messages) != 1 || messages[0].State != "Delivered" || messages[0].Text != "retained" {
		t.Fatalf("incremental reconciliation failed: %v %v", messages, err)
	}
}
func TestCacheSkipsUnchangedAndRetainsMedia(t *testing.T) {
	a := testApp(t)
	m := Message{ID: "in", Direction: "in", Number: "+12025550123", Type: "MMS_DOWNLOADED", Media: []Attachment{{PartID: 7, Name: "a.png", ContentType: "image/png", Local: "cached.png"}}}
	if err := a.Cache.put([]Message{m}); err != nil {
		t.Fatal(err)
	}
	revision := a.Cache.revision
	m.Media = []Attachment{{PartID: 7, Name: "a.png", ContentType: "image/png"}}
	if err := a.cacheMessages([]Message{m}); err != nil {
		t.Fatal(err)
	}
	if a.Cache.revision != revision || a.Cache.records[m.key()].Media[0].Local != "cached.png" {
		t.Fatal("unchanged metadata rewrote cache or discarded downloaded media")
	}
	m.State = "Changed"
	if err := a.cacheMessages([]Message{m}); err != nil || a.Cache.revision != revision+1 {
		t.Fatal("changed records were not saved")
	}
}
func TestContactIndexSearchAndInvalidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contact.vcf")
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("BEGIN:VCARD\nFN:"+name+"\nNOTE:searchable nickname\nTEL:+12025550123\nEND:VCARD\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("Original")
	reader := &contactReader{}
	for _, query := range []string{"", "nickname", "searchable nickname"} {
		got, err := reader.read(context.Background(), dir, query)
		if err != nil || got["+12025550123"].Name != "Original" {
			t.Fatal("indexed search lost vCard fields")
		}
	}
	write("Updated name")
	reader.checked = time.Time{}
	got, err := reader.read(context.Background(), dir, "nickname")
	if err != nil || got["+12025550123"].Name != "Updated name" {
		t.Fatal("changed vCard was not reindexed")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reader.checked = time.Time{}
	got, err = reader.read(context.Background(), dir, "")
	if err != nil || len(got) != 0 {
		t.Fatal("deleted vCard remained indexed")
	}
}
func TestPushHandlerBackpressure(t *testing.T) {
	p := &pushListener{events: make(chan pushEvent, 1)}
	h := p.handler("/secret")
	for i, want := range []int{204, 503} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/secret", strings.NewReader(`{"id":"event","event":"sms:received"}`))
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("request %d returned %d, want %d", i, w.Code, want)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/wrong", strings.NewReader(`{}`)))
	if w.Code != 404 {
		t.Fatal("listener accepted a wrong secret path")
	}
}

// Opt-in callback check waits for a real incoming event. It never sends texts,
// reimports history, changes phone settings, or prints message content.
func TestLivePushRoundTrip(t *testing.T) {
	path := os.Getenv("SMS_PUSH_CONFIG")
	if path == "" {
		t.Skip("explicit SMS_PUSH_CONFIG required")
	}
	c, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	g := newGateway(c)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	p, err := startPush(ctx, g, path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	t.Log("HTTPS listener ready; waiting for a real incoming event")
	select {
	case event := <-p.events:
		m, ok := event.incoming()
		if !ok {
			t.Fatal("expected incoming SMS/MMS event")
		}
		stamp, _ := time.Parse(time.RFC3339Nano, m.Timestamp)
		rows, err := g.listing(ctx, "/inbox?includeAttachments=true&from="+url.QueryEscape(stamp.Add(-time.Second).Format(time.RFC3339))+"&to="+url.QueryEscape(stamp.Add(time.Second).Format(time.RFC3339)))
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ID == m.ID && phoneKey(row.Sender) == m.Number {
				t.Log("phone-to-client HTTPS callback and canonical inbox identity verified")
				return
			}
		}
		t.Fatal("webhook identity differs from canonical inbox")
	case <-time.After(5 * time.Minute):
		t.Skip("no real incoming event arrived during the check")
	}
}

func TestMMSArrivalFetchesMetadataWithoutMedia(t *testing.T) {
	a := testApp(t)
	var requests atomic.Int32
	stamp := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/inbox" || r.URL.Query().Get("type") != "MMS_DOWNLOADED" || r.URL.Query().Get("from") == "" {
			t.Error("unexpected MMS fetch")
			http.Error(w, "unexpected", 500)
			return
		}
		fmt.Fprintf(w, `[{"id":"mms:1","sender":"+12025550123","createdAt":%q,"type":"MMS_DOWNLOADED","contentPreview":"caption","attachments":[{"partId":7,"name":"a.png","contentType":"image/png","size":1000000}]}]`, stamp.Format(time.RFC3339))
	}))
	defer server.Close()
	a.Gateway = newGateway(Config{Server: server.URL, Timeout: 3})
	a.MMSWait = map[string]pendingMMS{"header": {Number: "+12025550123", Since: stamp.Add(-2 * time.Minute), Delay: time.Second}}
	a.pollMMS()
	settle(t, a)
	a.pollMMS()
	if requests.Load() != 1 || len(a.MMSWait) != 0 {
		t.Fatal("downloaded MMS kept polling")
	}
	for _, m := range a.Cache.records {
		if m.ID == "mms:1" && (len(m.Media) != 1 || len(m.Media[0].Inline) != 0 || m.Media[0].Local != "") {
			t.Fatal("media fetched without user request")
		}
	}
	for _, event := range pushEvents {
		if event == "mms:downloaded" {
			t.Fatal("subscribed to automatic base64 media downloads")
		}
	}
}

func TestLiveIncomingMMSMetadata(t *testing.T) {
	path, caption := os.Getenv("SMS_PUSH_CONFIG"), os.Getenv("SMS_MMS_CAPTION")
	if path == "" || caption == "" {
		t.Skip("explicit SMS_PUSH_CONFIG and SMS_MMS_CAPTION required")
	}
	c, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	c.CacheDir = t.TempDir()
	c.Cache = true
	a, err := newApp(c, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	p, err := startPush(a.Context, a.Gateway, path)
	if err != nil {
		t.Fatal(err)
	}
	a.Push = p
	a.LastSync = time.Now()
	t.Log("waiting for incoming MMS arrival notification and metadata")
	until := time.Now().Add(3 * time.Minute)
	for time.Now().Before(until) {
		a.tick()
		for _, m := range a.Cache.records {
			if m.Type != "MMS_DOWNLOADED" || !strings.Contains(m.Text, caption) {
				continue
			}
			if len(m.Media) == 0 {
				t.Fatal("MMS attachment metadata missing")
			}
			part := m.Media[0]
			if part.Local != "" || len(part.Inline) != 0 {
				t.Fatal("incoming image downloaded before user action")
			}
			data, err := a.Gateway.download(a.Context, m.ID, part)
			if err != nil || len(data) == 0 {
				t.Fatal("explicit attachment download failed", err)
			}
			destination := filepath.Join(t.TempDir(), safeName(part.Name, part.ContentType))
			if err = saveExclusive(destination, data); err != nil || !isPrivateFile(destination) {
				t.Fatal("private attachment save failed", err)
			}
			t.Log("incoming MMS displayed through metadata only; explicit image download and private save verified")
			return
		}
		time.Sleep(80 * time.Millisecond)
	}
	t.Fatal("incoming MMS metadata not received")
}
