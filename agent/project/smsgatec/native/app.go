package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type snapshot struct {
	View, Query, Anchor   string
	Selected, Top, Scroll int
	Aliases               map[string]bool
	FilterBase            *snapshot
}
type result struct {
	SyncTime            time.Time
	MediaKey, MediaPath string
	MediaPart           int64
	Push                *pushListener
	Events              []pushEvent
	Kind, Query         string
	Token               int
	Messages            []Message
	Contacts            map[string]Contact
	Err                 error
	Notice, OpenPath    string
}
type sendContext struct {
	ID, Number, Body string
	Files            []string
	Subject          string
}
type App struct {
	AccountBase                 snapshot
	MMSWait                     map[string]pendingMMS
	SyncSince                   time.Time
	Dirty                       bool
	ContactReader               *contactReader
	HistoryCache                []drawLine
	HistoryRevision             uint64
	HistoryWidth                int
	HistoryNumber, HistoryQuery string
	HistorySending              bool
	Push                        *pushListener
	PushAttempt                 time.Time
	PendingEvents               []pushEvent
	EventRetry                  time.Time
	SyncFailures                int
	Config                      Config
	ConfigPath                  string
	Cache                       *Cache
	Gateway                     *Gateway
	Demo                        bool
	View, Query, Number, Phase  string
	Selected, Top, Scroll       int
	Body                        []rune
	Cursor                      int
	Contacts                    map[string]Contact
	Aliases                     map[string]bool
	Stack                       []snapshot
	Search                      bool
	SearchBase, FilterBase      *snapshot
	SearchToken                 int
	LastSearch                  string
	SearchChanged               time.Time
	Paste                       bool
	Status                      string
	StatusUntil, LastSync       time.Time
	Jobs                        map[string]bool
	Results                     chan result
	Context                     context.Context
	Cancel                      context.CancelFunc
	Sending                     *sendContext
	Running                     bool
	BeforeHelp                  string
	Prompt                      func(string, string, bool) (string, bool)
	DraftFiles                  []string
	LastFileDir, MediaTemp      string
	Picker                      *picker
	Subject                     string
	Workers                     sync.WaitGroup
}

func newApp(c Config, path string, demo bool) (*App, error) {
	cache, err := openCache(c, demo)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{Config: c, ConfigPath: path, Cache: cache, Gateway: newGateway(c), Demo: demo, View: "index", Phase: "read", Contacts: map[string]Contact{}, Jobs: map[string]bool{}, Results: make(chan result, 16), Context: ctx, Cancel: cancel, Running: true}
	a.MediaTemp, err = os.MkdirTemp("", "smsgatec-media-")
	if err != nil {
		cancel()
		cache.close()
		return nil, err
	}
	if demo {
		a.Contacts = map[string]Contact{"+12025550123": {"Alex Morgan", "mobile"}, "+12025550124": {"Jordan Lee", "work"}, "+12025550125": {"Sam Patel", "mobile"}}
		err = cache.put([]Message{
			{ID: "a", Number: "+12025550123", Direction: "in", Text: "Are we still meeting for dinner?", Timestamp: "2026-11-01T17:30:00Z", State: "Received"},
			{ID: "b", Number: "+12025550123", Direction: "out", Text: "Yes! See you at seven.", Timestamp: "2026-11-01T17:31:00Z", State: "Delivered"},
			{ID: "c", Number: "+12025550124", Direction: "in", Text: "The package is at the front desk.", Timestamp: "2026-11-01T16:15:00Z", State: "Received"},
		})
	}
	a.loadSyncCursor()
	return a, err
}
func (a *App) close() {
	a.Cancel()
	a.Workers.Wait()
	a.Push.close()
	for {
		select {
		case r := <-a.Results:
			if r.Push != nil {
				r.Push.close()
			}
		default:
			a.Gateway.client.CloseIdleConnections()
			a.Cache.close()
			os.RemoveAll(a.MediaTemp)
			return
		}
	}
}
func (a *App) note(message string) {
	a.Dirty = true
	a.Status = message
	a.StatusUntil = time.Now().Add(8 * time.Second)
}
func (a *App) job(kind string, fn func() result) {
	if a.Jobs[kind] {
		return
	}
	a.Jobs[kind] = true
	a.Workers.Add(1)
	go func() {
		defer a.Workers.Done()
		r := fn()
		r.Kind = kind
		select {
		case a.Results <- r:
		case <-a.Context.Done():
		}
	}()
}
func (a *App) knownOutgoing() map[string]Message {
	known := map[string]Message{}
	for _, m := range a.Cache.records {
		if m.Direction == "out" {
			known[m.ID] = m
		}
	}
	return known
}
func (a *App) sync() {
	if a.Demo || !a.Config.ready() || a.Jobs["sync"] || a.Jobs["send"] || a.Jobs["events"] || a.Jobs["mms"] {
		return
	}
	known := a.knownOutgoing()
	gateway := a.Gateway
	started := time.Now()
	since := a.SyncSince
	a.LastSync = started
	a.job("sync", func() result {
		messages, err := gateway.syncSince(a.Context, known, since)
		return result{Messages: messages, Err: err, SyncTime: started}
	})
}
func (a *App) contacts(query string) {
	if a.Demo {
		return
	}
	if a.ContactReader == nil {
		a.ContactReader = &contactReader{}
	}
	reader, directory := a.ContactReader, a.Config.VCFDir
	kind := "contacts"
	token := a.SearchToken
	if query != "" {
		kind = "search"
	}
	a.job(kind, func() result {
		contacts, err := reader.read(a.Context, directory, query)
		return result{Contacts: contacts, Query: query, Token: token, Err: err}
	})
}
func (a *App) drain() {
	for {
		select {
		case r := <-a.Results:
			a.Dirty = true
			delete(a.Jobs, r.Kind)
			if r.Err != nil {
				if r.Kind == "sync" {
					a.SyncFailures++
				}
				if r.Kind == "events" {
					a.PendingEvents = append(r.Events, a.PendingEvents...)
					a.EventRetry = time.Now().Add(30 * time.Second)
				}
				if r.Kind == "push" {
					a.note("push unavailable; retrying connection")
					continue
				}
				if r.Kind == "send" && a.Sending != nil {
					ctx := a.Sending
					m := a.Cache.records["out:"+ctx.ID+":"+ctx.Number]
					m.State = "Rejected"
					var api *APIError
					if !errors.As(r.Err, &api) || api.Uncertain {
						m.State = "Uncertain"
					}
					if err := a.Cache.put([]Message{m}); err != nil {
						a.note(err.Error())
					} else {
						a.note(r.Err.Error())
					}
				} else if r.Kind != "search" {
					a.note(r.Err.Error())
				}
				continue
			}
			switch r.Kind {
			case "contacts":
				a.HistoryCache = nil
				a.Contacts = r.Contacts
			case "search":
				a.HistoryCache = nil
				if r.Token == a.SearchToken && r.Query == a.Query {
					a.Aliases = map[string]bool{}
					for number, c := range r.Contacts {
						a.Contacts[number] = c
						a.Aliases[number] = true
					}
					if a.Selected == 0 && len(a.entries()) > 0 {
						a.Selected = 1
					}
				}
			case "push":
				a.Push = r.Push
				a.note("push updates connected")
			case "sync", "events", "mms":
				if r.Kind == "sync" {
					a.SyncFailures = 0
				}
				state := a.capture()
				if err := a.cacheMessages(r.Messages); err != nil {
					a.SyncFailures++
					a.note(err.Error())
				} else {
					if r.Kind == "sync" {
						a.SyncSince = r.SyncTime
						a.saveSyncCursor()
					}
					if r.Kind == "events" {
						a.trackMMS(r.Events)
						for _, e := range r.Events {
							if e.Event == "app:started" {
								a.LastSync = time.Time{}
							}
						}
					}
				}
				a.resolvedMMS(r.Messages)
				if a.View == "index" || a.View == "contacts" {
					a.restorePosition(state)
				}
			case "send":
				if err := a.cacheMessages(r.Messages); err != nil {
					a.note("send accepted, but cache save failed")
				} else {
					if a.Sending != nil && a.Number == a.Sending.Number && string(a.Body) == a.Sending.Body && a.Subject == a.Sending.Subject && strings.Join(a.DraftFiles, "\x00") == strings.Join(a.Sending.Files, "\x00") {
						a.Body = nil
						a.Cursor = 0
						a.Phase = "read"
						a.DraftFiles = nil
						a.Subject = ""
					}
					a.note("message accepted")
					a.StatusUntil = time.Now().Add(2 * time.Second)
				}
				// Sending already returns the accepted record; status webhooks follow.
			case "import":
				a.SyncSince = time.Time{}

				a.note("history import started")
				a.LastSync = time.Time{}
			case "attachment":
				if r.MediaPath != "" {
					if m, ok := a.Cache.records[r.MediaKey]; ok {
						m.Media = append([]Attachment(nil), m.Media...)
						for i := range m.Media {
							if m.Media[i].PartID == r.MediaPart {
								m.Media[i].Local = r.MediaPath
							}
						}
						if a.Cache.path != "" {
							if err := a.Cache.put([]Message{m}); err != nil {
								a.note(err.Error())
							}
						} else {
							a.Cache.records[m.key()] = m
							a.Cache.revision++
						}
					}
				}
				if r.OpenPath != "" {
					if err := launchOpener(r.OpenPath); err != nil {
						a.note(err.Error())
					}
				} else if r.Notice != "" {
					a.note(r.Notice)
				}
			}
		default:
			return
		}
	}
}
func (a *App) contact(number string) Contact {
	if c, ok := a.Contacts[number]; ok {
		return c
	}
	d := digits(number)
	if len(d) >= 10 {
		matches := []Contact{}
		for n, c := range a.Contacts {
			if strings.HasSuffix(digits(n), d[len(d)-10:]) {
				matches = append(matches, c)
			}
		}
		if len(matches) == 1 {
			return matches[0]
		}
	}
	return Contact{"Unlisted contact", ""}
}
func (a *App) entries() []string {
	latest := map[string]Message{}
	for _, m := range a.Cache.records {
		previous, ok := latest[m.Number]
		if !ok || m.stamp().After(previous.stamp()) || (m.stamp().Equal(previous.stamp()) && m.key() > previous.key()) {
			latest[m.Number] = m
		}
	}
	numbers := []string{}
	if a.View == "contacts" {
		keys := map[string]bool{}
		for number := range a.Contacts {
			canonical := number
			if !strings.HasPrefix(number, "+") && len(digits(number)) >= 10 {
				matches := []string{}
				d := digits(number)
				for n := range latest {
					if strings.HasSuffix(n, d[len(d)-10:]) {
						matches = append(matches, n)
					}
				}
				if len(matches) == 1 {
					canonical = matches[0]
				}
			}
			keys[canonical] = true
		}
		for n := range keys {
			numbers = append(numbers, n)
		}
		sort.Slice(numbers, func(i, j int) bool {
			left, right := strings.ToLower(a.contact(numbers[i]).Name), strings.ToLower(a.contact(numbers[j]).Name)
			if left == right {
				return numbers[i] < numbers[j]
			}
			return left < right
		})
	} else {
		for n := range latest {
			numbers = append(numbers, n)
		}
		sort.Slice(numbers, func(i, j int) bool {
			left, right := latest[numbers[i]], latest[numbers[j]]
			if left.stamp().Equal(right.stamp()) {
				return numbers[i] < numbers[j]
			}
			return left.stamp().After(right.stamp())
		})
	}
	query := strings.ToLower(a.Query)
	filtered := []string{}
	for _, n := range numbers {
		if query == "" || strings.Contains(strings.ToLower(a.contact(n).Name), query) || strings.Contains(strings.ToLower(n), query) || a.Aliases[n] {
			filtered = append(filtered, n)
		}
	}
	return filtered
}
func copyAliases(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	result := map[string]bool{}
	for k, v := range source {
		result[k] = v
	}
	return result
}
func (a *App) capture() snapshot {
	anchor := ""
	if a.View == "index" || a.View == "contacts" {
		rows := a.entries()
		if a.Selected > 0 && a.Selected <= len(rows) {
			anchor = rows[a.Selected-1]
		}
	}
	return snapshot{View: a.View, Query: a.Query, Anchor: anchor, Selected: a.Selected, Top: a.Top, Scroll: a.Scroll, Aliases: copyAliases(a.Aliases), FilterBase: a.FilterBase}
}
func (a *App) restorePosition(s snapshot) {
	a.Selected, a.Top = s.Selected, s.Top
	if s.Anchor != "" {
		for i, n := range a.entries() {
			if n == s.Anchor {
				relative := s.Selected - s.Top
				a.Selected = i + 1
				a.Top = max(0, a.Selected-relative)
				return
			}
		}
	}
	a.Selected = min(a.Selected, len(a.entries()))
	a.Top = min(a.Top, a.Selected)
}
func (a *App) restore(s snapshot) {
	a.View, a.Query, a.Scroll = s.View, s.Query, s.Scroll
	a.Aliases = copyAliases(s.Aliases)
	a.FilterBase = s.FilterBase
	a.Search = false
	a.SearchToken++
	a.LastSearch = ""
	a.restorePosition(s)
}
func (a *App) openContacts() {
	if a.View == "contacts" {
		a.newNumber("", "New conversation · Number")
		return
	}
	a.Stack = append(a.Stack, a.capture())
	a.View = "contacts"
	a.Query = ""
	a.Selected = 0
	a.Top = 0
	a.Aliases = nil
	a.FilterBase = nil
	a.Search = false
	a.SearchToken++
}
func (a *App) openThread(number string) {
	if a.View == "index" || a.View == "contacts" {
		a.Stack = append(a.Stack, a.capture())
	}
	a.View = "thread"
	a.Number = number
	a.Phase = "read"
	a.Query = ""
	a.Scroll = 0
	a.Body = nil
	a.DraftFiles = nil
	a.Subject = ""
	a.Cursor = 0
	a.Search = false
	a.FilterBase = nil
	a.Aliases = nil
	a.SearchToken++
}
func (a *App) back() {
	if a.FilterBase != nil {
		base := *a.FilterBase
		a.restore(base)
		return
	}
	a.Body = nil
	a.DraftFiles = nil
	a.Subject = ""
	a.Cursor = 0
	a.Phase = "read"
	a.Search = false
	if len(a.Stack) > 0 {
		s := a.Stack[len(a.Stack)-1]
		a.Stack = a.Stack[:len(a.Stack)-1]
		a.restore(s)
	} else {
		a.View = "index"
		a.Query = ""
		a.Selected = 0
		a.Top = 0
	}
}
func (a *App) startSearch() {
	s := a.capture()
	a.SearchBase = &s
	a.Search = true
	a.SearchToken++
	a.LastSearch = ""
	a.SearchChanged = time.Now()
}
func (a *App) cancelSearch() {
	if a.SearchBase != nil {
		s := *a.SearchBase
		a.SearchBase = nil
		a.restore(s)
	} else {
		a.Search = false
	}
}
func (a *App) confirmSearch() {
	if a.FilterBase == nil {
		a.FilterBase = a.SearchBase
	}
	a.Search = false
	a.SearchBase = nil
	if a.View != "thread" {
		a.selectEntry()
	}
}
func (a *App) searchChanged() {
	a.SearchToken++
	a.Aliases = nil
	a.LastSearch = ""
	a.SearchChanged = time.Now()
	if a.View == "thread" {
		a.Scroll = 0
	} else if len(a.entries()) > 0 {
		a.Selected = 1
		a.Top = 0
	} else {
		a.Selected = 0
		a.Top = 0
	}
}
func (a *App) selectEntry() {
	rows := a.entries()
	a.Selected = max(0, min(a.Selected, len(rows)))
	if a.Selected == 0 {
		if a.View == "index" {
			a.openContacts()
		} else {
			initial := ""
			if strings.HasPrefix(a.Query, "+") {
				initial = a.Query
			}
			a.newNumber(initial, "New conversation · Number")
		}
		return
	}
	number := rows[a.Selected-1]
	if a.View == "contacts" && !strings.HasPrefix(number, "+") {
		a.newNumber(number, a.contact(number).Name+" · Include country code")
	} else {
		a.openThread(number)
	}
}
func (a *App) newNumber(initial, title string) {
	if a.Prompt == nil {
		return
	}
	value, ok := a.Prompt(title, initial, false)
	if !ok {
		return
	}
	number, err := recipient(value)
	if err != nil {
		a.note(err.Error())
		return
	}
	a.openThread(number)
}
func (a *App) send() {
	if a.Demo {
		a.note("demo: sending disabled")
		return
	}
	if a.Jobs["send"] {
		a.note("waiting for the current send")
		return
	}
	if strings.TrimSpace(string(a.Body)) == "" && len(a.DraftFiles) == 0 {
		a.Phase = "compose"
		return
	}
	number, err := recipient(a.Number)
	if err != nil {
		a.note(err.Error())
		return
	}
	ctx := &sendContext{ID: messageID(), Number: number, Body: string(a.Body), Files: append([]string{}, a.DraftFiles...), Subject: a.Subject}
	parts := []WireAttachment{}
	media := []Attachment{}
	total := 0
	for _, path := range a.DraftFiles {
		part, err := readMedia(path)
		if err != nil {
			a.note(err.Error())
			return
		}
		total += len(part.Data)
		if total > 20*1024*1024 {
			a.note("combined attachments exceed 20 MiB")
			return
		}
		metadata := Attachment{Name: part.Name, ContentType: part.ContentType, Size: int64(len(part.Data)), Local: path}
		if a.Cache.path != "" {
			metadata.Local, err = mediaFile(filepath.Join(filepath.Dir(a.Cache.path), "media"), metadata, part.Data)
			if err != nil {
				a.note("cannot cache outgoing attachment; not sent")
				return
			}
		}
		parts = append(parts, part)
		media = append(media, metadata)
	}
	kind := "SMS"
	if len(parts) > 0 || ctx.Subject != "" {
		kind = "MMS"
	}
	message := Message{ID: ctx.ID, Number: ctx.Number, Direction: "out", Text: ctx.Body, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), State: "Uncertain", Type: kind, Media: media, Subject: ctx.Subject}
	if err = a.Cache.put([]Message{message}); err != nil {
		a.note("cannot save outgoing record; not sent")
		return
	}
	a.Sending = ctx
	gateway := a.Gateway
	a.job("send", func() result {
		var messages []Message
		var err error
		if len(parts) > 0 || ctx.Subject != "" {
			messages, err = gateway.sendMMS(a.Context, ctx.ID, ctx.Number, ctx.Body, ctx.Subject, parts)
		} else {
			messages, err = gateway.send(a.Context, ctx.ID, ctx.Number, ctx.Body)
		}
		return result{Messages: messages, Err: err}
	})
}
func (a *App) insert(text string) {
	runes := []rune(text)
	body := append([]rune{}, a.Body[:a.Cursor]...)
	body = append(body, runes...)
	body = append(body, a.Body[a.Cursor:]...)
	a.Body = body
	a.Cursor += len(runes)
}
func (a *App) editKey(key int) {
	switch key {
	case 10, 13, keyEnter, 27:
		a.Phase = "review"
	case keyShiftEnter, keyF4:
		a.insert("\n")
	case keyBackspace, 8, 127:
		if a.Cursor > 0 {
			a.Body = append(a.Body[:a.Cursor-1], a.Body[a.Cursor:]...)
			a.Cursor--
		}
	case keyDelete:
		if a.Cursor < len(a.Body) {
			a.Body = append(a.Body[:a.Cursor], a.Body[a.Cursor+1:]...)
		}
	case keyLeft:
		a.Cursor = max(0, a.Cursor-1)
	case keyRight:
		a.Cursor = min(len(a.Body), a.Cursor+1)
	case keyHome:
		for a.Cursor > 0 && a.Body[a.Cursor-1] != '\n' {
			a.Cursor--
		}
	case keyEnd:
		for a.Cursor < len(a.Body) && a.Body[a.Cursor] != '\n' {
			a.Cursor++
		}
	case keyUp, keyDown:
		start := a.Cursor
		for start > 0 && a.Body[start-1] != '\n' {
			start--
		}
		column := a.Cursor - start
		if key == keyUp && start > 0 {
			end := start - 1
			previous := end
			for previous > 0 && a.Body[previous-1] != '\n' {
				previous--
			}
			a.Cursor = min(end, previous+column)
		}
		if key == keyDown {
			end := a.Cursor
			for end < len(a.Body) && a.Body[end] != '\n' {
				end++
			}
			if end < len(a.Body) {
				next := end + 1
				end = next
				for end < len(a.Body) && a.Body[end] != '\n' {
					end++
				}
				a.Cursor = min(end, next+column)
			}
		}
	case 9:
		a.insert("    ")
	default:
		if key >= 32 && key < 0x110000 {
			a.insert(string(rune(key)))
		}
	}
}
func (a *App) key(key int, height int) {
	if key < 0 || key == keyResize {
		return
	}
	if a.View == "picker" {
		a.pickerKey(key, height)
		return
	}
	if key == keyPasteStart {
		a.Paste = true
		return
	}
	if key == keyPasteEnd {
		a.Paste = false
		return
	}
	if a.Paste {
		if a.View == "thread" && a.Phase == "compose" && key < 0x110000 {
			if key == 13 {
				key = 10
			}
			a.insert(string(rune(key)))
		}
		return
	}
	if a.View == "thread" && a.Phase == "compose" {
		a.editKey(key)
		return
	}
	if a.Search {
		switch key {
		case 27:
			a.cancelSearch()
		case 10, 13, keyEnter:
			a.confirmSearch()
		case 8, 127, keyBackspace:
			r := []rune(a.Query)
			if len(r) > 0 {
				a.Query = string(r[:len(r)-1])
				a.searchChanged()
			}
		case keyUp:
			a.Selected = max(0, a.Selected-1)
		case keyDown:
			a.Selected = min(len(a.entries()), a.Selected+1)
		default:
			if key >= 32 && key < 0x110000 {
				a.Query += string(rune(key))
				a.searchChanged()
			}
		}
		return
	}
	if a.View == "accounts" {
		a.accountsKey(key)
		return
	}
	if key == 'y' && (a.View == "index" || a.View == "contacts") {
		if len(a.Config.Accounts) == 0 {
			a.note("one account configured")
			return
		}
		a.AccountBase = a.capture()
		a.View = "accounts"
		a.Selected = a.accountIndex()
		a.Top = 0
		return
	}
	if a.View == "help" {
		if key == 'q' || key == 'b' || key == '?' || key == 27 {
			a.View = a.BeforeHelp
		}
		return
	}
	if a.View == "thread" && a.Phase == "review" && key == 's' {
		if a.Prompt != nil {
			subject, ok := a.Prompt("MMS subject (empty clears)", a.Subject, false)
			if ok {
				a.Subject = subject
			}
		}
		return
	}
	switch key {
	case 'q':
		if a.View == "thread" || a.View == "contacts" || a.FilterBase != nil {
			a.back()
		} else if a.Jobs["send"] {
			a.note("waiting for send result")
		} else {
			a.Running = false
		}
		return
	case '?':
		a.BeforeHelp = a.View
		a.View = "help"
		return
	case 'r', keyF5:
		a.SyncSince = time.Time{}

		a.sync()
		a.contacts("")
		return
	case 'I', keyF6:
		if !a.Demo && a.Config.ready() {
			gateway := a.Gateway
			a.job("import", func() result { return result{Err: gateway.importHistory(a.Context)} })
		}
		return
	case 'c', keyF2:
		a.settings()
		return
	case 't':
		modes := []string{"auto", "light", "dark"}
		for i, m := range modes {
			if a.Config.Theme == m {
				a.Config.Theme = modes[(i+1)%3]
				break
			}
		}
		terminalTheme(a.Config.Theme)
		return
	case '/':
		a.startSearch()
		return
	}
	if a.View == "thread" {
		switch key {
		case 'y':
			if a.Phase == "review" {
				a.send()
			}
		case 'b', 27:
			a.back()
		case 10, 13, keyEnter:
			if a.Phase == "review" {
				a.send()
			} else {
				a.Phase = "compose"
			}
		case 'e':
			a.Phase = "compose"
		case 'E':
			a.externalEditor()
		case 'o':
			a.openAttachmentPicker("open")
		case 's':
			a.openAttachmentPicker("save")
		case 'a':
			a.openFilePicker()
		case 'd':
			if a.Phase == "review" && len(a.DraftFiles) > 0 {
				a.DraftFiles = a.DraftFiles[:len(a.DraftFiles)-1]
			}
		case 'k', keyUp:
			a.Scroll++
		case 'j', keyDown:
			a.Scroll = max(0, a.Scroll-1)
		case keyPageUp:
			a.Scroll += max(1, height-2)
		case keyPageDown:
			a.Scroll = max(0, a.Scroll-max(1, height-2))
		case 'g':
			a.Scroll = 1 << 30
		case 'G':
			a.Scroll = 0
		}
	} else {
		switch key {
		case 'j', keyDown:
			a.Selected = min(len(a.entries()), a.Selected+1)
		case 'k', keyUp:
			a.Selected = max(0, a.Selected-1)
		case 10, 13, keyEnter:
			a.selectEntry()
		case 'n':
			a.openContacts()
		case 'b', 27:
			a.back()
		case 'g':
			a.Selected = 0
		case 'G':
			a.Selected = len(a.entries())
		case keyPageUp:
			a.Selected = max(0, a.Selected-max(1, height-1))
		case keyPageDown:
			a.Selected = min(len(a.entries()), a.Selected+max(1, height-1))
		}
	}
}
func (a *App) settings() {
	if a.Demo || a.Prompt == nil {
		return
	}
	if len(a.Jobs) > 0 {
		a.note("wait for the current request")
		return
	}
	next := a.Config
	for _, field := range []struct {
		Title  string
		Value  *string
		Secret bool
	}{{"Server URL", &next.Server, false}, {"Username", &next.Username, false}, {"Password · Enter keeps existing", &next.Password, true}} {
		initial := *field.Value
		if field.Secret {
			initial = ""
		}
		value, ok := a.Prompt(field.Title, initial, field.Secret)
		if !ok {
			return
		}
		if !field.Secret || value != "" {
			*field.Value = value
		}
	}
	next.Server = strings.TrimRight(strings.TrimSpace(next.Server), "/")
	if !next.ready() {
		a.note("server, username, and password required")
		return
	}
	if err := next.validate(); err != nil {
		a.note(err.Error())
		return
	}
	cache, err := openCache(next, false)
	if err != nil {
		a.note(err.Error())
		return
	}
	if err = next.save(a.ConfigPath); err != nil {
		cache.close()
		a.note("cannot save configuration")
		return
	}
	a.Push.close()
	a.Push = nil
	a.PushAttempt = time.Time{}
	a.PendingEvents = nil
	a.MMSWait = nil
	a.Gateway.client.CloseIdleConnections()
	a.Cache.close()
	a.Cache = cache
	a.HistoryCache = nil
	a.Config = next
	a.Gateway = newGateway(next)
	a.Stack = nil
	a.FilterBase = nil
	a.View = "index"
	a.Query = ""
	a.Selected = 0
	a.Top = 0
	a.Body = nil
	a.DraftFiles = nil
	a.Subject = ""
	a.LastSync = time.Time{}
	a.SyncSince = time.Time{}

	a.loadSyncCursor()
}
func (a *App) externalEditor() {
	command := os.Getenv("VISUAL")
	if command == "" {
		command = os.Getenv("EDITOR")
	}
	if command == "" {
		command = "vi"
		if runtime.GOOS == "windows" {
			command = "notepad.exe"
		}
	}
	args, err := splitCommand(command)
	if err != nil {
		a.note(err.Error())
		return
	}
	file, err := os.CreateTemp("", "smsgatec-*.txt")
	if err != nil {
		a.note("cannot create editor file")
		return
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.WriteString(string(a.Body)); err != nil {
		file.Close()
		a.note("cannot write editor file")
		return
	}
	file.Close()
	terminalSuspend()
	cmd := exec.Command(args[0], append(args[1:], name)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	terminalResume()
	if err != nil {
		a.note("editor did not finish successfully")
		return
	}
	data, err := os.ReadFile(name)
	if err != nil {
		a.note("cannot read editor text")
		return
	}
	a.Body = []rune(string(data))
	a.Cursor = len(a.Body)
	a.Phase = "review"
}
func (a *App) tick() {
	a.drain()
	if a.Search && a.View != "thread" && a.Query != "" && !a.Demo && a.LastSearch != a.Query && time.Since(a.SearchChanged) > 250*time.Millisecond && !a.Jobs["search"] {
		a.LastSearch = a.Query
		a.contacts(a.Query)
	}
	if !a.Demo && a.Config.ready() && a.Config.UpdateMode == "push" {
		if a.Push != nil {
			select {
			case <-a.Push.done:
				a.Push.close()
				a.Push = nil
				a.note("push disconnected; retrying connection")
			default:
			}
		}
		if a.Push == nil && !a.Jobs["push"] && time.Since(a.PushAttempt) >= a.pushRetryInterval() {
			a.PushAttempt = time.Now()
			gateway, path := a.Gateway, a.ConfigPath
			a.job("push", func() result { p, err := startPush(a.Context, gateway, path); return result{Push: p, Err: err} })
		}
		if a.Push != nil {
			for len(a.PendingEvents) < 128 {
				select {
				case e := <-a.Push.events:
					a.PendingEvents = append(a.PendingEvents, e)
				default:
					goto eventsDrained
				}
			}
		}
	}
eventsDrained:
	if len(a.PendingEvents) > 0 && !a.Jobs["events"] && !a.Jobs["sync"] && !a.Jobs["send"] && !a.Jobs["mms"] && !time.Now().Before(a.EventRetry) {
		events := a.PendingEvents
		a.PendingEvents = nil
		gateway := a.Gateway
		a.job("events", func() result {
			messages, err := gateway.eventMessages(a.Context, events)
			return result{Messages: messages, Events: events, Err: err}
		})
	}
	a.pollMMS()
	interval := time.Duration(a.Config.Poll) * time.Second
	if a.Config.UpdateMode == "push" {
		interval = max(interval, 60*time.Second)
	}
	if a.Config.UpdateMode == "push" {
		interval = time.Duration(a.Config.Reconcile) * time.Second
		if interval == 0 {
			interval = time.Hour
		}
	}
	// Back off network failures rather than repeatedly downloading history.
	if a.SyncFailures > 0 {
		interval = max(interval, min(time.Duration(1<<min(a.SyncFailures, 6))*15*time.Second, 15*time.Minute))
	}
	if time.Since(a.LastSync) >= interval && !a.Jobs["events"] {
		a.sync()
	}
}
func (a *App) summary() string {
	return fmt.Sprintf("%d messages, %d contacts", len(a.Cache.records), len(a.Contacts))
}
