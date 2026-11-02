package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// Filter before BEGIN: unchanged refreshes need no SQLite transaction or fsync.
func changedMessages(records map[string]Message, messages []Message) []Message {
	changed := make([]Message, 0, len(messages))
	for _, m := range messages {
		if old, ok := records[m.key()]; !ok || !reflect.DeepEqual(old, m) {
			changed = append(changed, m)
		}
	}
	return changed
}

// The cursor advances only after both endpoints and the cache commit succeed.
// It is scoped to the gateway/account so changing servers never skips history.
type syncCursor struct{ Since time.Time }

func (a *App) cursorPath() string {
	if a.Cache.path == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(a.Config.Server + "\x00" + a.Config.Username))
	return filepath.Join(filepath.Dir(a.Cache.path), "sync-"+hex.EncodeToString(sum[:8])+".json")
}
func (a *App) loadSyncCursor() {
	path := a.cursorPath()
	if path == "" || len(a.Cache.records) == 0 {
		return
	}
	data, err := os.ReadFile(path)
	var cursor syncCursor
	if err == nil && json.Unmarshal(data, &cursor) == nil && !cursor.Since.After(time.Now()) {
		a.SyncSince = cursor.Since
	}
}
func (a *App) saveSyncCursor() {
	if path := a.cursorPath(); path != "" {
		data, _ := json.Marshal(syncCursor{a.SyncSince})
		if err := privateWrite(path, data); err != nil {
			a.note("cannot save sync cursor; next start will reconcile history")
		}
	}
}

func stateRank(state string) int {
	switch state {
	case "Pending":
		return 1
	case "Processed":
		return 2
	case "Sent":
		return 3
	case "Delivered":
		return 4
	}
	return 0
}

// Shared by history/status fetches and cache writes. Message content is immutable;
// a metadata refresh must not discard cached text, subjects, or downloaded media.
func mergeMessage(old, next Message) Message {
	if next.Direction == "out" {
		if stateRank(old.State) > stateRank(next.State) && stateRank(next.State) > 0 {
			next.State = old.State
		}
		if next.Text == "[Text unavailable]" {
			next.Text = old.Text
		}
		if strings.HasPrefix(old.Type, "MMS") && next.Type == "SMS" {
			next.Type = old.Type
			next.Media = old.Media
		}
	}
	if next.Subject == "" {
		next.Subject = old.Subject
	}
	if len(next.Media) == 0 {
		next.Media = nil
		return next
	}
	next.Media = append([]Attachment(nil), next.Media...)
	for i := range next.Media {
		for _, part := range old.Media {
			if part.PartID == next.Media[i].PartID && part.Name == next.Media[i].Name && part.ContentType == next.Media[i].ContentType && len(next.Media[i].Inline) == 0 {
				next.Media[i].Local = part.Local
				next.Media[i].Inline = part.Inline
			}
		}
	}
	return next
}
