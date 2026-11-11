package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var webhookEvents = map[string]bool{
	"sms:received":  true,
	"mms:received":  true,
	"sms:sent":      true,
	"sms:delivered": true,
	"sms:failed":    true,
	"sms:cancelled": true,
	"app:started":   true,
}

type Webhook struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Event string `json:"event"`
}

type WebhookStore struct {
	mu    sync.Mutex
	path  string
	items map[string]Webhook
}

func openWebhooks(home string) (*WebhookStore, error) {
	store := &WebhookStore{path: home + "/webhooks.json", items: map[string]Webhook{}}
	data, e := os.ReadFile(store.path)
	if e == nil {
		if e = json.Unmarshal(data, &store.items); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if store.items == nil {
		store.items = map[string]Webhook{}
	}
	return store, nil
}

func validWebhook(hook Webhook) error {
	if hook.ID == "" || len(hook.ID) > 128 || strings.ContainsAny(hook.ID, "/\\\x00") {
		return fmt.Errorf("invalid webhook id")
	}
	if !webhookEvents[hook.Event] {
		return fmt.Errorf("unsupported webhook event")
	}
	parsed, e := url.Parse(hook.URL)
	if e != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || len(hook.URL) > 2048 {
		return fmt.Errorf("invalid webhook URL")
	}
	return nil
}

func (w *WebhookStore) list() []Webhook {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Webhook, 0, len(w.items))
	for _, hook := range w.items {
		out = append(out, hook)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *WebhookStore) put(hook Webhook) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if old, ok := w.items[hook.ID]; ok {
		if old.URL != hook.URL || old.Event != hook.Event {
			return false, fmt.Errorf("webhook id already exists")
		}
		return false, nil
	}
	w.items[hook.ID] = hook
	if e := writeJSON(w.path, w.items); e != nil {
		delete(w.items, hook.ID)
		return false, e
	}
	return true, nil
}

func (w *WebhookStore) remove(id string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.items[id]; !ok {
		return false, nil
	}
	old := w.items[id]
	delete(w.items, id)
	if e := writeJSON(w.path, w.items); e != nil {
		w.items[id] = old
		return false, e
	}
	return true, nil
}

func (w *WebhookStore) forEvent(event string) []Webhook {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []Webhook{}
	for _, hook := range w.items {
		if hook.Event == event {
			out = append(out, hook)
		}
	}
	return out
}

func (s *Bridge) dispatch(event string, payload any) {
	if s.webhooks == nil {
		return
	}
	for _, hook := range s.webhooks.forEvent(event) {
		go s.postWebhook(hook, event, payload)
	}
}

func (s *Bridge) postWebhook(hook Webhook, event string, payload any) {
	body, e := json.Marshal(map[string]any{"id": randomID(), "event": event, "payload": payload})
	if e != nil {
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		request, e := http.NewRequestWithContext(ctx, "POST", hook.URL, strings.NewReader(string(body)))
		if e == nil {
			request.Header.Set("Content-Type", "application/json")
			response, requestErr := s.webhookClient.Do(request)
			if requestErr == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
				response.Body.Close()
				if response.StatusCode >= 200 && response.StatusCode < 300 {
					cancel()
					return
				}
			}
			e = requestErr
		}
		cancel()
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
}

func messageWebhook(m Message) (string, map[string]any, bool) {
	text := m.Preview
	if m.Body != nil {
		text = m.Body.Text
	}
	payload := map[string]any{
		"messageId":  m.Source,
		"receivedAt": m.Created,
		"sender":     m.Sender,
		"message":    text,
		"body":       text,
	}
	if m.MMS != nil {
		payload["subject"] = m.MMS.Subject
	}
	switch {
	case m.State == "Received" && m.Sender != "":
		event := "sms:received"
		if m.Type == "MMS" {
			event = "mms:received"
		}
		payload["transactionId"] = m.Source
		return event, payload, true
	case m.State == "Sent" && m.Submitted:
		return "sms:sent", payload, true
	case m.State == "Failed" && m.Submitted:
		return "sms:failed", payload, true
	}
	return "", nil, false
}

func (s *Bridge) deliverWebhooks(ctx context.Context) {
	updates, unsubscribe := s.store.subscribe()
	defer unsubscribe()
	seen := map[string]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case rows, ok := <-updates:
			if !ok {
				return
			}
			for _, m := range rows {
				event, payload, ok := messageWebhook(m)
				if !ok {
					continue
				}
				signature := event + "\x00" + m.State
				if seen[m.ID] == signature {
					continue
				}
				seen[m.ID] = signature
				s.dispatch(event, payload)
			}
		}
	}
}
