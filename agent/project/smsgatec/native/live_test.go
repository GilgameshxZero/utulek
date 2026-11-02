package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Explicitly opt in; never sends during ordinary regression runs. Each run has
// a durable journal written before any POST and refuses to reuse that journal.
func TestLiveWindowsGateway(t *testing.T) {
	path := os.Getenv("SMS_LIVE_CONFIG")
	if path == "" {
		t.Skip("live sends require SMS_LIVE_CONFIG")
	}
	number, err := recipient(os.Getenv("SMS_LIVE_NUMBER"))
	if err != nil {
		t.Fatal("live sends require SMS_LIVE_NUMBER with an explicit international recipient")
	}
	if err := protectFile(path); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	c.CacheDir = os.Getenv("SMS_LIVE_CACHE")
	if c.CacheDir == "" {
		c.CacheDir = filepath.Join(home(), "sms-live-cache")
	}
	c.Cache = true
	a, err := newApp(c, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	rows, err := readContacts(context.Background(), c.VCFDir, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("direct vCard lookup: %d phone entries", len(rows))
	journal := os.Getenv("SMS_LIVE_JOURNAL")
	if journal == "" {
		journal = filepath.Join(home(), "sms-live-journal.json")
	}

	type entry struct{ ID, Number, Text, State string }
	entries := []entry{}
	if _, e := os.Stat(journal); e == nil {
		t.Fatal("live journal already exists; inspect prior results before any further sends")
	}
	persist := func() {
		data, _ := json.MarshalIndent(entries, "", "  ")
		if e := privateWrite(journal, data); e != nil {
			t.Fatal(e)
		}
	}
	if len(entries) == 0 {
		for _, number := range []string{number} {
			id := messageID()
			text := "Windows sms port verification " + time.Now().UTC().Format(time.RFC3339) + " " + id
			entries = append(entries, entry{id, number, text, "Uncertain"})
			persist()
			a.Number = number
			a.Body = []rune(text)
			a.Phase = "review"
			a.send()
			until := time.Now().Add(60 * time.Second)
			for a.Jobs["send"] && time.Now().Before(until) {
				time.Sleep(100 * time.Millisecond)
				a.drain()
			}
			if a.Jobs["send"] {
				t.Fatal("send timeout; journal retained; do not repeat")
			}
			sent := false
			for _, m := range a.Cache.records {
				if m.Number == number && m.Text == text && m.State != "Failed" && m.State != "Uncertain" {
					entries[len(entries)-1].ID = m.ID
					sent = true
					break
				}
			}
			if !sent {
				t.Fatalf("send not accepted: %s", a.Status)
			}
			persist()
			for time.Now().Before(until) {
				row, e := a.Gateway.message(context.Background(), entries[len(entries)-1].ID)
				if e != nil {
					t.Fatal(e)
				}
				state := row.State
				for _, r := range row.Recipients {
					if r.State != "" {
						state = r.State
					}
				}
				entries[len(entries)-1].State = state
				persist()
				if state == "Delivered" || state == "Sent" {
					break
				}
				if state == "Failed" {
					t.Fatal("live send failed")
				}
				time.Sleep(time.Second)
			}
			t.Logf("SMS %s state %s", number, entries[len(entries)-1].State)
		}
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aH1sAAAAASUVORK5CYII=")
	id := messageID()
	text := "Windows sms MMS verification " + id
	entries = append(entries, entry{id, number, text, "Uncertain"})
	persist()
	media := filepath.Join(c.CacheDir, "windows-test.png")
	if err := privateWrite(media, png); err != nil {
		t.Fatal(err)
	}
	a.Number = number
	a.Body = []rune(text)
	a.DraftFiles = []string{media}
	a.Subject = "Windows port test"
	a.Phase = "review"
	a.send()
	until := time.Now().Add(60 * time.Second)
	for a.Jobs["send"] && time.Now().Before(until) {
		time.Sleep(100 * time.Millisecond)
		a.drain()
	}
	accepted := false
	for _, m := range a.Cache.records {
		if m.Text == text && m.State != "Uncertain" && m.State != "Failed" {
			entries[len(entries)-1].ID = m.ID
			accepted = true
		}
	}
	if !accepted {
		t.Fatalf("MMS not accepted: %s", a.Status)
	}
	for time.Now().Before(until) {
		r, e := a.Gateway.message(context.Background(), entries[len(entries)-1].ID)
		if e != nil {
			t.Fatal(e)
		}
		state := r.State
		for _, p := range r.Recipients {
			if p.State != "" {
				state = p.State
			}
		}
		entries[len(entries)-1].State = state
		persist()
		if state == "Sent" || state == "Delivered" {
			t.Log("MMS handed to carrier:", state)
			return
		}
		if state == "Failed" {
			t.Fatal("MMS failed")
		}
		time.Sleep(time.Second)
	}
	t.Fatal("MMS final state unresolved; journal retained")
}

func TestLiveWindowsIncoming(t *testing.T) {
	path := os.Getenv("SMS_LIVE_CONFIG")
	if path == "" {
		t.Skip("requires explicit live config")
	}
	number, err := recipient(os.Getenv("SMS_LIVE_NUMBER"))
	if err != nil {
		t.Fatal("live inbox checks require SMS_LIVE_NUMBER with an explicit international sender")
	}
	c, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	g := newGateway(c)
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		rows, e := g.listing(context.Background(), "/inbox?includeAttachments=true")
		if e != nil {
			t.Fatal(e)
		}
		received := false
		downloaded := false
		for _, r := range rows {
			if phoneKey(r.Sender) != number {
				continue
			}
			if strings.Contains(r.Preview, "Google Voice reply to verify Windows sms incoming") {
				received = true
			}
			for _, part := range r.Attachments {
				if downloaded {
					break
				}
				data, e := g.download(context.Background(), r.ID, part)
				if e != nil {
					t.Fatal(e)
				}
				if len(data) == 0 {
					t.Fatal("empty MMS attachment")
				}
				path := filepath.Join(t.TempDir(), safeName(part.Name, part.ContentType))
				if e = saveExclusive(path, data); e != nil {
					t.Fatal(e)
				}
				if !isPrivateFile(path) {
					t.Fatal("downloaded MMS not private")
				}
				downloaded = true
			}
		}
		if received && downloaded {
			t.Log("Windows received Google Voice reply and downloaded/saved incoming MMS with private ACL")
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("incoming reply or MMS download not verified")
}
