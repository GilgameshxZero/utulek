package main

import (
	"net/url"
	"time"
)

// SMSGate's downloaded-MMS webhook includes base64 media. Subscribe only to its
// small arrival notification and poll metadata briefly; fetch media on demand.
type pendingMMS struct {
	Number      string
	Since, Next time.Time
	Delay       time.Duration
	Attempts    int
}

func (a *App) trackMMS(events []pushEvent) {
	if a.MMSWait == nil {
		a.MMSWait = map[string]pendingMMS{}
	}
	for _, e := range events {
		if e.Event != "mms:received" {
			continue
		}
		m, ok := e.incoming()
		if !ok {
			continue
		}
		a.MMSWait[m.ID] = pendingMMS{Number: m.Number, Since: m.stamp().Add(-2 * time.Minute), Next: time.Now().Add(2 * time.Second), Delay: 2 * time.Second}
	}
}
func (a *App) resolvedMMS(messages []Message) {
	for _, m := range messages {
		if m.Type != "MMS_DOWNLOADED" {
			continue
		}
		for id, p := range a.MMSWait {
			if m.Number == p.Number && !m.stamp().Before(p.Since) && m.stamp().Before(p.Since.Add(12*time.Minute)) {
				delete(a.MMSWait, id)
			}
		}
	}
}
func (a *App) pollMMS() {
	if len(a.MMSWait) == 0 || a.Jobs["mms"] || a.Jobs["sync"] || a.Jobs["events"] || a.Jobs["send"] {
		return
	}
	var since time.Time
	now := time.Now()
	for id, p := range a.MMSWait {
		if now.Before(p.Next) {
			continue
		}
		if p.Attempts >= 7 {
			delete(a.MMSWait, id)
			continue
		}
		if since.IsZero() || p.Since.Before(since) {
			since = p.Since
		}
		p.Attempts++
		p.Delay = min(p.Delay*2, time.Minute)
		p.Next = now.Add(p.Delay)
		a.MMSWait[id] = p
	}
	if since.IsZero() {
		return
	}
	gateway := a.Gateway
	a.job("mms", func() result {
		rows, err := gateway.listing(a.Context, "/inbox?includeAttachments=true&type=MMS_DOWNLOADED&from="+url.QueryEscape(since.UTC().Format(time.RFC3339)))
		messages := []Message{}
		for _, r := range rows {
			messages = append(messages, r.messages()...)
		}
		return result{Messages: messages, Err: err}
	})
}
