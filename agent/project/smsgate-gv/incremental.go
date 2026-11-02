package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const coldSyncPath = "/voice/v1/voiceclient/inbox/coldsync"
const warmSyncPath = "/voice/v1/voiceclient/inbox/warmsync"

var errSyncReset = errors.New("Google Voice sync cursor requires reset")

// Bump when decoding expands so records skipped by older builds are backfilled.
const syncCursorVersion = 1

type syncCursor struct {
	Version int    `json:"version"`
	Account string `json:"account"`
	Token   string `json:"token"`
}

func (b *Browser) watchUpdates() {
	if b.updates != nil {
		return
	}
	b.updates = make(chan struct{}, 1)
	var mu sync.Mutex
	channels := map[network.RequestID]bool{}
	appReads := map[network.RequestID]bool{}
	notify := func() {
		if b.watchReady.Load() {
			select {
			case b.updates <- struct{}{}:
			default:
			}
		}
	}
	chromedp.ListenTarget(b.ctx, func(event any) {
		mu.Lock()
		defer mu.Unlock()
		switch e := event.(type) {
		case *network.EventResponseReceived:
			u, err := url.Parse(e.Response.URL)
			if err != nil {
				return
			}
			if strings.Contains(u.Path, "/punctual/multi-watch/channel") && e.Type == network.ResourceTypeXHR {
				channels[e.RequestID] = true
			}
		case *network.EventRequestWillBeSent:
			if b.apiInFlight.Load() == 0 && e.Type == network.ResourceTypeXHR {
				u, err := url.Parse(e.Request.URL)
				if err == nil && (u.Path == warmSyncPath || strings.Contains(u.Path, "/api2thread/get")) {
					appReads[e.RequestID] = true
				}
			}
		case *network.EventDataReceived:
			// WebChannel keepalives observed at 54 bytes or less are ignored. Larger
			// channel chunks are an invalidation hint, never parsed as message content.
			if channels[e.RequestID] && e.DataLength > 64 {
				notify()
			}
		case *network.EventLoadingFinished:
			if appReads[e.RequestID] {
				notify()
			}
			delete(appReads, e.RequestID)
			delete(channels, e.RequestID)
		case *network.EventLoadingFailed:
			delete(appReads, e.RequestID)
			delete(channels, e.RequestID)
		}
	})
}

func (b *Browser) readIncremental() ([]Message, error) {
	if b.cursor == "" {
		request := []any{[]any{[]int{1, 0, 0}, []int{2, 100, 50}, []int{3, 0, 0}, []int{4, 0, 0}, []int{5, 0, 0}, []int{6, 0, 0}, []int{7, 0, 0}, []int{8, 0, 0}, []int{9, 0, 0}, []int{10, 0, 0}}, []int{1, 1, 1, 1, 0}}
		body, e := b.voiceEndpoint(coldSyncPath, request)
		if e != nil {
			return nil, e
		}
		rows, token, e := decodeColdSync(body, b.config)
		if e != nil {
			return nil, e
		}
		b.pendingCursor = token
		return rows, nil
	}
	body, e := b.voiceEndpoint(warmSyncPath, []any{100, b.cursor, []int{1, 1, 1, 1, 0}})
	if e != nil {
		return nil, e
	}
	rows, token, more, e := decodeWarmSync(body, b.config)
	if errors.Is(e, errSyncReset) {
		b.cursor = ""
		return b.readIncremental()
	}
	if e != nil {
		return nil, e
	}
	b.pendingCursor = token
	// Pagination continuation is signaled to the worker only after this batch's
	// records and cursor have been committed. Never jump past uncommitted data.
	b.moreUpdates = more
	return rows, nil
}

func decodeColdSync(body []byte, c Config) ([]Message, string, error) {
	var envelope []json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || len(envelope) < 2 {
		return nil, "", errors.New("invalid cold sync response")
	}
	var views [][]json.RawMessage
	var token string
	if json.Unmarshal(envelope[0], &views) != nil || json.Unmarshal(envelope[1], &token) != nil || token == "" {
		return nil, "", errors.New("invalid cold sync cursor")
	}
	threads := []json.RawMessage{}
	for _, view := range views {
		var kind int
		if len(view) > 0 {
			_ = json.Unmarshal(view[0], &kind)
		}
		if kind == 2 && len(view) > 2 {
			var part []json.RawMessage
			if json.Unmarshal(view[2], &part) != nil {
				return nil, "", errors.New("invalid cold sync threads")
			}
			threads = append(threads, part...)
		}
	}
	wrapped, _ := json.Marshal([]any{threads})
	rows, e := decodeHistory(wrapped, c)
	return rows, token, e
}

func decodeWarmSync(body []byte, c Config) ([]Message, string, bool, error) {
	var envelope []json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || len(envelope) == 0 {
		return nil, "", false, errors.New("invalid warm sync response")
	}
	if len(envelope) > 1 && string(envelope[1]) != "null" {
		return nil, "", false, errSyncReset
	}
	var content []json.RawMessage
	if json.Unmarshal(envelope[0], &content) != nil || len(content) < 4 {
		return nil, "", false, errors.New("invalid warm sync content")
	}
	var token string
	if json.Unmarshal(content[3], &token) != nil || token == "" {
		return nil, "", false, errors.New("invalid warm sync cursor")
	}
	var threads [][]json.RawMessage
	var items []json.RawMessage
	if string(content[0]) != "null" && json.Unmarshal(content[0], &threads) != nil {
		return nil, "", false, errors.New("invalid changed threads")
	}
	if string(content[1]) != "null" && json.Unmarshal(content[1], &items) != nil {
		return nil, "", false, errors.New("invalid changed items")
	}
	byThread := map[string]map[string]json.RawMessage{}
	add := func(threadID string, raw json.RawMessage) error {
		var fields []json.RawMessage
		var id string
		if json.Unmarshal(raw, &fields) != nil || len(fields) < 15 || json.Unmarshal(fields[0], &id) != nil || id == "" {
			return errors.New("invalid changed message identity")
		}
		if byThread[threadID] == nil {
			byThread[threadID] = map[string]json.RawMessage{}
		}
		byThread[threadID][id] = raw
		return nil
	}
	for _, thread := range threads {
		var id string
		if len(thread) < 1 || json.Unmarshal(thread[0], &id) != nil {
			return nil, "", false, errors.New("invalid changed thread identity")
		}
		if len(thread) > 2 && string(thread[2]) != "null" {
			var records []json.RawMessage
			if json.Unmarshal(thread[2], &records) != nil {
				return nil, "", false, errors.New("invalid changed thread items")
			}
			for _, r := range records {
				if e := add(id, r); e != nil {
					return nil, "", false, e
				}
			}
		}
		if len(thread) > 14 && string(thread[14]) != "null" {
			if e := add(id, thread[14]); e != nil {
				return nil, "", false, e
			}
		}
	}
	for _, item := range items {
		var fields []json.RawMessage
		var id string
		if json.Unmarshal(item, &fields) != nil || len(fields) < 30 || json.Unmarshal(fields[29], &id) != nil {
			return nil, "", false, errors.New("invalid changed message thread")
		}
		if e := add(id, item); e != nil {
			return nil, "", false, e
		}
	}
	combined := []any{}
	for id, records := range byThread {
		values := []json.RawMessage{}
		for _, record := range records {
			values = append(values, record)
		}
		combined = append(combined, []any{id, 1, values})
	}
	wrapped, _ := json.Marshal([]any{combined})
	warmConfig := c
	warmConfig.MaxThreads = len(combined) + 1
	rows, e := decodeHistory(wrapped, warmConfig)
	if e != nil {
		return nil, "", false, e
	}
	more := false
	if len(content) > 4 {
		var value int
		_ = json.Unmarshal(content[4], &value)
		more = value != 0
	}
	return rows, token, more, nil
}

func (b *Browser) commitCursor(store *Store) error {
	if b.pendingCursor == "" {
		return nil
	}
	if b.pendingCursor != b.cursor {
		if e := writeJSON(filepath.Join(filepath.Dir(store.path), "sync_cursor.json"), syncCursor{Version: syncCursorVersion, Account: b.config.Number, Token: b.pendingCursor}); e != nil {
			return e
		}
	}
	b.cursor = b.pendingCursor
	b.pendingCursor = ""
	b.watchReady.Store(true)
	if b.moreUpdates {
		select {
		case b.updates <- struct{}{}:
		default:
		}
		b.moreUpdates = false
	}
	return nil
}

// Keep notification hints coalesced; a single message can generate several
// channel chunks and app-side requests.
const notificationDebounce = 250 * time.Millisecond
