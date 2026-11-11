package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Bridge struct {
	config        Config
	store         *Store
	home          string
	webhooks      *WebhookStore
	webhookClient *http.Client
	attachments   chan attachmentRequest
	jobs          chan string
	refresh       chan bool
	mu            sync.Mutex
	ready         bool
	lastError     string
	lastSync      string
	authState     string
	authNextRetry string
}

func (s *Bridge) health(ready bool, err error, synced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = ready
	s.lastError = ""
	if err != nil {
		s.lastError = err.Error()
	}
	if synced {
		s.lastSync = now()
	}
}
func (s *Bridge) worker(ctx context.Context, home string) {
	var b *Browser
	var updates <-chan struct{}
	var debounce <-chan time.Time
	var notificationTimer *time.Timer
	defer func() {
		if notificationTimer != nil {
			notificationTimer.Stop()
		}
	}()
	retry := authRetry{interval: time.Duration(s.config.AuthRetrySeconds) * time.Second}
	connectBrowser := func() error {
		var e error
		b, e = retry.connect(time.Now(), func() (*Browser, error) { return startBrowser(context.WithoutCancel(ctx), home, s.config) })
		s.mu.Lock()
		s.authState = "authenticated"
		s.authNextRetry = ""
		if e != nil {
			s.authState = "paused"
			s.authNextRetry = retry.next.UTC().Format(time.RFC3339)
		}
		s.mu.Unlock()
		return e
	}
	defer func() {
		if b != nil {
			b.close()
		}
	}()
	syncBrowser := func() {
		if b == nil {
			if e := connectBrowser(); e != nil {
				s.health(false, e, false)
				return
			}
		}
		e := b.sync(s.store)
		if e == nil {
			e = b.saveSession(home)
		}
		s.health(e == nil, e, e == nil)
		if b != nil {
			updates = b.updates
		}
		// Keep the authenticated page through transient endpoint/storage failures;
		// only authentication loss or a dead browser needs a new page bootstrap.
		if e != nil && (errors.Is(e, errAuthPaused) || b.ctx.Err() != nil) {
			b.close()
			b = nil
			if errors.Is(e, errAuthPaused) {
				// Import fresh source cookies immediately after authentication loss.
				if recoveryErr := connectBrowser(); recoveryErr != nil {
					s.health(false, recoveryErr, false)
				}
			}
		}
	}
	syncBrowser()
	ticker := time.NewTicker(time.Duration(s.config.PollSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-updates:
			if debounce == nil {
				notificationTimer = time.NewTimer(notificationDebounce)
				debounce = notificationTimer.C
			}
		case <-debounce:
			debounce = nil
			syncBrowser()
		case <-ticker.C:
			syncBrowser()
		case <-s.refresh:
			syncBrowser()
		case id := <-s.jobs:
			m, ok := s.store.get(id)
			if !ok || m.State != "Pending" {
				continue
			}
			if b == nil {
				setState(&m, "Failed", "browser unavailable; no send attempted")
			} else {
				m = b.send(s.store, m)
				if m.Error == errAuthPaused.Error() {
					s.health(false, errAuthPaused, false)
					b.close()
					b = nil
					if recoveryErr := connectBrowser(); recoveryErr != nil {
						s.health(false, recoveryErr, false)
					}
				}
			}
			if e := s.store.put(m); e != nil {
				s.health(false, errors.New("cannot persist send outcome; service stopped accepting sends"), false)
				if b != nil {
					b.close()
					b = nil
				}
			}
			if b != nil && b.ctx.Err() != nil {
				b.close()
				b = nil
				s.health(false, errors.New("browser stopped after uncertain send; reconciling without resend"), false)
				select {
				case s.refresh <- true:
				default:
				}
			}
		case request := <-s.attachments:
			if b == nil {
				if e := connectBrowser(); e != nil {
					request.result <- attachmentResult{err: e}
					continue
				}
			}
			data, contentType, e := b.attachment(request.sourceID)
			request.result <- attachmentResult{data: data, contentType: contentType, err: e}
		}
	}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func secureEqual(a, b string) bool {
	aa := sha256.Sum256([]byte(a))
	bb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aa[:], bb[:]) == 1
}

var bridgeCapabilities = []string{"sms", "full-history", "recent-history", "mms-download", "group-receive", "webhooks", "event-stream", "incremental-sync"}

func (s *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// No CORS support. Reject cross-origin browser requests even with credentials.
	if r.Header.Get("Origin") != "" {
		apiError(w, 403, "browser origins are not supported")
		return
	}
	u, p, ok := r.BasicAuth()
	if !ok || !secureEqual(u, s.config.Username) || !secureEqual(p, s.config.Password) {
		w.Header().Set("WWW-Authenticate", `Basic realm="smsgate_gv"`)
		apiError(w, 401, "authentication required")
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/events":
		s.eventStream(w, r)
	case r.Method == "GET" && r.URL.Path == "/health":
		s.mu.Lock()
		v := map[string]any{"ready": s.ready, "error": s.lastError, "lastSync": s.lastSync, "number": s.config.Number, "backend": "google_voice", "capabilities": bridgeCapabilities, "authentication": s.authState, "authNextRetry": s.authNextRetry}
		ready := s.ready
		s.mu.Unlock()
		status := 200
		if !ready {
			status = 503
		}
		respond(w, status, v)
	case r.Method == "GET" && (r.URL.Path == "/inbox" || r.URL.Path == "/messages"):
		limit, offset := 500, 0
		var e error
		if q := r.URL.Query().Get("limit"); q != "" {
			limit, e = strconv.Atoi(q)
			if e != nil || limit < 1 || limit > 500 {
				apiError(w, 400, "limit must be 1..500")
				return
			}
		}
		if q := r.URL.Query().Get("offset"); q != "" {
			offset, e = strconv.Atoi(q)
			if e != nil || offset < 0 {
				apiError(w, 400, "invalid offset")
				return
			}
		}
		s.mu.Lock()
		synced := s.lastSync != ""
		s.mu.Unlock()
		if !synced && len(s.store.list(true))+len(s.store.list(false)) == 0 {
			apiError(w, 503, "initial Google Voice synchronization is not complete")
			return
		}
		rows := s.store.list(r.URL.Path == "/inbox")
		if value := r.URL.Query().Get("from"); value != "" {
			from, e := time.Parse(time.RFC3339Nano, value)
			if e != nil {
				apiError(w, 400, "from must be an RFC3339 timestamp")
				return
			}
			filtered := make([]Message, 0, len(rows))
			for _, m := range rows {
				stamp, e := time.Parse(time.RFC3339Nano, m.Created)
				if e == nil && !stamp.Before(from) {
					filtered = append(filtered, m)
				}
			}
			rows = filtered
		}
		w.Header().Set("X-Total-Count", strconv.Itoa(len(rows)))
		if offset > len(rows) {
			offset = len(rows)
		}
		end := offset + limit
		if end > len(rows) {
			end = len(rows)
		}
		respond(w, 200, rows[offset:end])
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/message/"):
		m, ok := s.store.get(strings.TrimPrefix(r.URL.Path, "/message/"))
		if !ok {
			apiError(w, 404, "message not found")
			return
		}
		respond(w, 200, m)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/inbox/") && strings.Contains(r.URL.Path, "/attachments/"):
		rest := strings.TrimPrefix(r.URL.Path, "/inbox/")
		parts := strings.SplitN(rest, "/attachments/", 2)
		if len(parts) != 2 || parts[1] == "" {
			apiError(w, 400, "invalid attachment path")
			return
		}
		messageID, e := url.PathUnescape(parts[0])
		if e != nil {
			apiError(w, 400, "invalid message ID")
			return
		}
		partID, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil || partID < 1 {
			apiError(w, 400, "invalid attachment part")
			return
		}
		m, ok := s.store.get(messageID)
		if !ok {
			apiError(w, 404, "message not found")
			return
		}
		var attachment *Attachment
		for i := range m.Attachments {
			if m.Attachments[i].PartID == partID {
				attachment = &m.Attachments[i]
				break
			}
		}
		if attachment == nil {
			apiError(w, 404, "attachment not found")
			return
		}
		data, contentType, e := s.attachmentBytes(*attachment)
		if e != nil {
			apiError(w, 503, "attachment unavailable")
			return
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Disposition", `inline; filename="`+attachment.Name+`"`)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	case r.Method == "GET" && r.URL.Path == "/webhooks":
		respond(w, 200, s.webhooks.list())
	case r.Method == "POST" && r.URL.Path == "/webhooks":
		var hook Webhook
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		if e := json.NewDecoder(r.Body).Decode(&hook); e != nil {
			apiError(w, 400, "invalid webhook JSON")
			return
		}
		if e := validWebhook(hook); e != nil {
			apiError(w, 400, e.Error())
			return
		}
		created, e := s.webhooks.put(hook)
		if e != nil {
			apiError(w, 409, e.Error())
			return
		}
		status := 200
		if created {
			status = 201
		}
		respond(w, status, hook)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/webhooks/"):
		id, e := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/webhooks/"))
		if e != nil || id == "" {
			apiError(w, 400, "invalid webhook id")
			return
		}
		removed, e := s.webhooks.remove(id)
		if e != nil {
			apiError(w, 500, "cannot persist webhook removal")
			return
		}
		if !removed {
			apiError(w, 404, "webhook not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == "POST" && r.URL.Path == "/inbox/refresh":
		select {
		case s.refresh <- true:
		default:
		}
		respond(w, 202, map[string]string{"state": "Pending", "scope": "full history available through Google Voice cold sync"})
	case r.Method == "POST" && r.URL.Path == "/message":
		s.submit(w, r)
	default:
		apiError(w, 404, "unsupported endpoint")
	}
}
func (s *Bridge) submit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       string          `json:"id"`
		Body     *Text           `json:"textMessage"`
		MMS      json.RawMessage `json:"mmsMessage"`
		Numbers  []string        `json:"phoneNumbers"`
		Delivery bool            `json:"withDeliveryReport"`
	}
	var mms struct {
		Text        string `json:"text"`
		Subject     string `json:"subject"`
		Attachments []struct {
			ContentType string `json:"contentType"`
			Name        string `json:"name"`
			Data        []byte `json:"data"`
		} `json:"attachments"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024*1024)
	d := json.NewDecoder(r.Body)
	if e := d.Decode(&body); e != nil {
		apiError(w, 400, "invalid JSON or request too large")
		return
	}
	if d.Decode(new(any)) != io.EOF {
		apiError(w, 400, "expected one JSON object")
		return
	}
	mmsPresent := len(body.MMS) > 0
	if mmsPresent {
		if json.Unmarshal(body.MMS, &mms) != nil {
			apiError(w, 400, "invalid mmsMessage")
			return
		}
	}
	text := ""
	if body.Body != nil {
		text = body.Body.Text
	}
	if mmsPresent {
		if mms.Text != "" {
			text = mms.Text
		} else if mms.Subject != "" {
			text = mms.Subject
		}
	}
	if len(text) > 8000 || len(body.Numbers) != 1 || !phonePattern.MatchString(body.Numbers[0]) || len(body.ID) > 128 || strings.ContainsAny(body.ID, "/\\\x00") {
		apiError(w, 400, "requires textMessage.text (0..8000 bytes) and one international phoneNumbers entry; optional id up to 128 characters")
		return
	}
	if strings.TrimSpace(text) == "" && !(mmsPresent && len(mms.Attachments) > 0) {
		apiError(w, 400, "text is required without MMS attachments")
		return
	}
	if body.ID == "" {
		body.ID = randomID()
	}
	m := Message{Submitted: true, Account: s.config.Number, ID: body.ID, Created: now(), State: "Pending", Body: &Text{Text: text}, Type: "SMS", Recipients: []Recipient{{body.Numbers[0], "Pending"}}}
	if mmsPresent {
		if len(mms.Attachments) > 10 {
			apiError(w, 400, "at most 10 attachments are supported")
			return
		}
		total := 0
		for i, attachment := range mms.Attachments {
			if len(attachment.Data) == 0 || len(attachment.Data) > 20*1024*1024 {
				apiError(w, 400, "each attachment must be 1..20 MiB")
				return
			}
			total += len(attachment.Data)
			if total > 25*1024*1024 {
				apiError(w, 400, "attachments exceed the 25 MiB request limit")
				return
			}
			contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
			if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") && !strings.HasPrefix(contentType, "audio/") {
				apiError(w, 400, "attachments must be image, video or audio")
				return
			}
			name := filepath.Base(strings.ReplaceAll(attachment.Name, "\\", "_"))
			if name == "" || name == "." || name == "/" {
				name = fmt.Sprintf("attachment-%d", i+1)
			}
			if len(name) > 255 {
				apiError(w, 400, "attachment name is too long")
				return
			}
			m.Attachments = append(m.Attachments, Attachment{PartID: int64(i + 1), Name: name, ContentType: contentType, Size: int64(len(attachment.Data)), Inline: append([]byte(nil), attachment.Data...)})
		}
		m.Type = "MMS"
		m.MMS = &MediaText{Text: text, Subject: mms.Subject}
	}
	// Duplicate IDs are reconciled even when signed out, and never enqueued twice.
	if old, ok := s.store.get(m.ID); ok {
		if old.Account != m.Account || old.Body == nil || old.Body.Text != m.Body.Text || len(old.Recipients) != 1 || old.Recipients[0].Number != m.Recipients[0].Number || len(old.Attachments) != len(m.Attachments) {
			apiError(w, 409, "message ID already exists with different content")
			return
		}
		respond(w, 200, old)
		return
	}
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	if !ready {
		apiError(w, 503, "Google Voice is not ready; check /health; authentication recovery retries automatically")
		return
	}
	m, fresh, e := s.store.reserve(m)
	if e != nil {
		apiError(w, 409, "cannot reserve message ID; conflicting request or journal write failure")
		return
	}
	if fresh {
		select {
		case s.jobs <- m.ID:
		default:
			setState(&m, "Failed", "queue full; no send attempted")
			if e := s.store.put(m); e != nil {
				apiError(w, 500, "cannot persist queue outcome; reconcile message by ID")
				return
			}
			respond(w, 503, m)
			return
		}
	}
	respond(w, 202, m)
}

func serve(ctx context.Context, home string, c Config) error {
	if e := rotateLogFile(os.Stdout, int64(c.LogMaxMB)*1024*1024); e != nil {
		fmt.Fprintln(os.Stderr, "Warning: cannot rotate service log:", e)
	}
	host, _, e := net.SplitHostPort(c.Listen)
	if e != nil {
		return e
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return errors.New("listen must contain a valid IP address and port")
	}
	store, e := openStore(home)
	if e != nil {
		return e
	}
	webhooks, e := openWebhooks(home)
	if e != nil {
		return e
	}
	listener, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return e
	}
	s := &Bridge{authState: "checking", config: c, store: store, home: home, webhooks: webhooks, webhookClient: &http.Client{Timeout: 10 * time.Second}, attachments: make(chan attachmentRequest, 4), jobs: make(chan string, 32), refresh: make(chan bool, 1)}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.deliverWebhooks(workerCtx)
	go s.dispatch("app:started", map[string]any{"backend": "google_voice"})
	done := make(chan struct{})
	go func() { defer close(done); s.worker(workerCtx, home) }()
	server := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() {
		<-workerCtx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Printf("Listening on http://%s (HTTP Basic auth required). No browser message content is logged.\n", c.Listen)
	e = server.Serve(listener)
	cancel()
	<-done
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
