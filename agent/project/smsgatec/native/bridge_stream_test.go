package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBridgeStreamUsesNativeIDsAndReconnects(t *testing.T) {
	var connections atomic.Int64
	var unexpected atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "test" || password != "private" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/health":
			fmt.Fprint(w, `{"backend":"google_voice","capabilities":["event-stream"]}`)
		case "/events":
			n := connections.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			if n == 1 {
				fmt.Fprint(w, "data: {\"event\":\"bridge:messages\",\"messages\":[{\"id\":\"gv-native-id\",\"sender\":\"+15555550123\",\"contentPreview\":\"stream text\",\"createdAt\":\"2026-11-02T00:00:00.123Z\",\"type\":\"SMS\"}]}\n\n")
				w.(http.Flusher).Flush()
				return
			}
			fmt.Fprint(w, "data: {\"event\":\"bridge:reset\"}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			unexpected.Add(1)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := newGateway(Config{Server: server.URL, Username: "test", Password: "private", Timeout: 5, PushRetry: 1})
	stream, e := startPush(ctx, g, "")
	if e != nil {
		t.Fatal(e)
	}
	defer stream.close()
	var first pushEvent
	select {
	case first = <-stream.events:
	case <-time.After(5 * time.Second):
		t.Fatal("no stream update")
	}
	messages, e := g.eventMessages(ctx, []pushEvent{first})
	if e != nil || len(messages) != 1 || messages[0].ID != "gv-native-id" || messages[0].Text != "stream text" || messages[0].Timestamp != "2026-11-02T00:00:00.123Z" {
		t.Fatal("stream changed native identity/content", e)
	}
	select {
	case event := <-stream.events:
		if event.Event != "bridge:reset" {
			t.Fatal("reconnect did not reset")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not reconnect")
	}
	if connections.Load() < 2 || unexpected.Load() != 0 {
		t.Fatal("event update unexpectedly polled history")
	}
	stream.close()
	select {
	case <-stream.done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream cancellation stuck")
	}
}
