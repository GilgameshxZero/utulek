package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const version = "0.5.0"

type Config struct {
	PushRetry     int       `json:"push_retry_seconds,omitempty"`
	Accounts      []Account `json:"accounts,omitempty"`
	ActiveAccount string    `json:"active_account,omitempty"`
	UpdateMode    string    `json:"update_mode,omitempty"`
	Reconcile     int       `json:"reconcile_seconds,omitempty"`
	Server        string    `json:"server"`
	Username      string    `json:"username"`
	Password      string    `json:"password"`
	Poll          int       `json:"poll_seconds"`
	Timeout       int       `json:"timeout_seconds"`
	Cache         bool      `json:"cache_enabled"`
	CacheDir      string    `json:"cache_dir"`
	Theme         string    `json:"theme"`
	VCFDir        string    `json:"vcf_dir"`
}

func configPath() string {
	if path := os.Getenv("SMSGATEC_CONFIG"); path != "" {
		return expand(path)
	}
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		root = filepath.Join(home(), ".config")
	}
	return filepath.Join(root, "smsgatec", "config.json")
}
func home() string { value, _ := os.UserHomeDir(); return value }
func expand(path string) string {
	if path == "~" {
		return home()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home(), path[2:])
	}
	return path
}
func (c Config) ready() bool { return c.Server != "" && c.Username != "" && c.Password != "" }
func (c Config) validate() error {
	if c.PushRetry != 0 && (c.PushRetry < 5 || c.PushRetry > 86400) {
		return errors.New("push_retry_seconds must be 5 to 86400")
	}
	seen := map[string]bool{}
	for _, account := range c.Accounts {
		if account.ID == "" || account.Name == "" || seen[account.ID] {
			return errors.New("accounts require unique IDs and names")
		}
		seen[account.ID] = true
		item := c
		item.Accounts = nil
		item.Server = account.Server
		item.Username = account.Username
		item.Password = account.Password
		if !item.ready() {
			return errors.New("account credentials and server required")
		}
		if err := item.validate(); err != nil {
			return err
		}
	}

	if c.UpdateMode != "" && c.UpdateMode != "push" && c.UpdateMode != "poll" {
		return errors.New("update_mode must be push or poll")
	}
	if c.Reconcile != 0 && (c.Reconcile < 60 || c.Reconcile > 86400) {
		return errors.New("reconcile_seconds must be 60 to 86400")
	}
	if c.Theme != "auto" && c.Theme != "light" && c.Theme != "dark" {
		return errors.New("theme must be auto, light, or dark")
	}
	if c.Poll < 3 || c.Poll > 3600 || c.Timeout < 3 || c.Timeout > 120 {
		return errors.New("invalid refresh interval or timeout")
	}
	if c.Server != "" {
		u, err := url.Parse(c.Server)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("use an HTTP(S) server URL with credentials in their own fields")
		}
	}
	return nil
}
func loadConfig(path string) (Config, error) {
	c := Config{UpdateMode: "push", Reconcile: 3600, Poll: 60, Timeout: 20, Cache: true, Theme: "auto", VCFDir: "~/main.syncthing/monochrome/module/contact/khard"}
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &c) != nil {
			return c, errors.New("invalid configuration JSON")
		}
	} else if !os.IsNotExist(err) {
		return c, errors.New("cannot read configuration")
	}
	if len(c.Accounts) > 0 {
		if c.ActiveAccount == "" {
			c.ActiveAccount = c.Accounts[0].ID
		}
		var err error
		c, err = c.account(c.ActiveAccount)
		if err != nil {
			return c, err
		}
	}
	for _, item := range []struct {
		key   string
		field *string
	}{{"SMSGATEC_SERVER", &c.Server}, {"SMSGATEC_USERNAME", &c.Username}, {"SMSGATEC_PASSWORD", &c.Password}} {
		if value, exists := os.LookupEnv(item.key); exists {
			*item.field = value
		}
	}
	c.Server = strings.TrimRight(c.Server, "/")
	return c, c.validate()
}
func privateWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".smsgatec-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = protectFile(file.Name()); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
func (c Config) save(path string) error {
	if err := c.validate(); err != nil {
		return err
	}
	if len(c.Accounts) > 0 {
		for i := range c.Accounts {
			if c.Accounts[i].ID == c.ActiveAccount {
				c.Accounts[i].Server = c.Server
				c.Accounts[i].Username = c.Username
				c.Accounts[i].Password = c.Password
			}
		}
		c.Server = ""
		c.Username = ""
		c.Password = ""
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return privateWrite(path, append(data, '\n'))
}
func cachePath(c Config) string {
	if !c.Cache || !c.ready() {
		return ""
	}
	root := expand(c.CacheDir)
	if root == "" {
		root = os.Getenv("XDG_CACHE_HOME")
		if root == "" {
			root = filepath.Join(home(), ".cache")
		}
		root = filepath.Join(root, "smsgatec")
	}
	hash := sha256.Sum256([]byte(c.Server + "\x00" + c.Username))
	return filepath.Join(root, hex.EncodeToString(hash[:6])+".sqlite3")
}

type Message struct {
	ID        string       `json:"id"`
	Number    string       `json:"number"`
	Direction string       `json:"direction"`
	Text      string       `json:"text"`
	Timestamp string       `json:"timestamp"`
	State     string       `json:"state"`
	Type      string       `json:"type,omitempty"`
	Media     []Attachment `json:"attachments,omitempty"`
	Subject   string       `json:"subject,omitempty"`
}

func (m Message) key() string { return m.Direction + ":" + m.ID + ":" + m.Number }
func (m Message) stamp() time.Time {
	if strings.HasPrefix(m.Timestamp, "1970-01-01") {
		return time.Time{}
	}
	value, _ := time.Parse(time.RFC3339Nano, m.Timestamp)
	return value
}
func (m Message) dateLabel() string {
	if m.stamp().IsZero() {
		return "date unknown"
	}
	return m.stamp().UTC().Format("01-02 15:04 UTC")
}
func sortedMessages(records map[string]Message, number string) []Message {
	result := []Message{}
	stamps := map[string]time.Time{}
	for _, m := range records {
		if number == "" || number == m.Number {
			result = append(result, m)
			stamps[m.key()] = m.stamp()
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if stamps[result[i].key()].Equal(stamps[result[j].key()]) {
			return result[i].key() < result[j].key()
		}
		return stamps[result[i].key()].Before(stamps[result[j].key()])
	})
	return result
}
func digits(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func recipient(value string) (string, error) {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("().-", r) {
			return -1
		}
		return r
	}, value)
	if strings.HasPrefix(value, "00") {
		value = "+" + value[2:]
	}
	if !phonePattern.MatchString(value) {
		return "", errors.New("include country code, e.g. +12025550123")
	}
	return value, nil
}
func phoneKey(value string) string {
	d := digits(value)
	if strings.HasPrefix(value, "00") {
		d = strings.TrimPrefix(d, "00")
	}
	if len(d) >= 7 {
		return "+" + d
	}
	if d != "" {
		return d
	}
	return value
}
func clean(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, value)
}

type Contact struct{ Name, Kind string }

func parseContacts(text string) map[string]Contact {
	result := map[string]Contact{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) == "" {
			continue
		}
		number := strings.TrimSpace(fields[0])
		if strings.HasPrefix(number, "+") || strings.HasPrefix(number, "00") {
			number = phoneKey(number)
		} else {
			number = strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) || strings.ContainsRune("().-", r) {
					return -1
				}
				return r
			}, number)
		}
		name := strings.TrimSpace(clean(fields[1]))
		if name == "" {
			name = "Unnamed contact"
		}
		kind := ""
		if len(fields) > 2 {
			kind = clean(fields[2])
		}
		result[number] = Contact{name, kind}
	}
	return result
}

type APIError struct {
	Text      string
	Status    int
	Uncertain bool
}

func (e *APIError) Error() string { return e.Text }

type Gateway struct {
	config Config
	client *http.Client
}

func newGateway(c Config) *Gateway {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 4
	transport.IdleConnTimeout = 90 * time.Second
	return &Gateway{c, &http.Client{Timeout: time.Duration(c.Timeout) * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (g *Gateway) request(ctx context.Context, method, path string, body any, target any) (http.Header, error) {
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.config.Server+path, input)
	if err != nil {
		return nil, &APIError{"invalid gateway address", 0, false}
	}
	req.SetBasicAuth(g.config.Username, g.config.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, &APIError{"gateway unreachable; check phone, network, and address", 0, method == "POST"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Header, &APIError{fmt.Sprintf("gateway HTTP %d", resp.StatusCode), resp.StatusCode, method == "POST" && (resp.StatusCode >= 500 || resp.StatusCode == 409)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024+1))
	if err != nil || len(data) > 32*1024*1024 {
		return resp.Header, &APIError{"incomplete gateway response", 0, method == "POST"}
	}
	if target != nil && len(data) > 0 {
		if json.Unmarshal(data, target) != nil {
			return resp.Header, &APIError{"invalid gateway response", 0, method == "POST"}
		}
	}
	return resp.Header, nil
}

type APIRecipient struct {
	Number string `json:"phoneNumber"`
	State  string `json:"state"`
}
type APIMessage struct {
	ID          string         `json:"id"`
	Created     string         `json:"createdAt"`
	Sender      string         `json:"sender"`
	Preview     string         `json:"contentPreview"`
	State       string         `json:"state"`
	Encrypted   bool           `json:"isEncrypted"`
	Recipients  []APIRecipient `json:"recipients"`
	Type        string         `json:"type"`
	Attachments []Attachment   `json:"attachments"`
	MMS         *MMSPayload    `json:"mmsMessage"`
	Body        *struct {
		Text string `json:"text"`
	} `json:"textMessage"`
}

func (row APIMessage) messages() []Message {
	if row.Sender != "" {
		return []Message{{ID: row.ID, Number: phoneKey(row.Sender), Direction: "in", Text: row.Preview, Timestamp: row.Created, State: "Received", Type: row.Type, Media: row.Attachments}}
	}
	text := "[Text unavailable]"
	if row.Body != nil {
		text = row.Body.Text
	}
	kind := "SMS"
	media := []Attachment{}
	if row.MMS != nil {
		kind = "MMS"
		text = row.MMS.Text
		if row.MMS.Subject != "" {
			// Subject is separate from the message text.
		}
		for _, part := range row.MMS.Attachments {
			media = append(media, Attachment{Name: part.Name, ContentType: part.ContentType, Size: int64(len(part.Data)), Inline: part.Data})
		}
	}
	if row.Encrypted {
		text = "[Encrypted message]"
	}
	result := []Message{}
	for _, r := range row.Recipients {
		state := r.State
		if state == "" {
			state = row.State
		}
		subject := ""
		if row.MMS != nil {
			subject = row.MMS.Subject
		}
		result = append(result, Message{ID: row.ID, Number: phoneKey(r.Number), Direction: "out", Text: text, Timestamp: row.Created, State: state, Type: kind, Media: media, Subject: subject})
	}
	return result
}
func (g *Gateway) listing(ctx context.Context, path string) ([]APIMessage, error) {
	result := []APIMessage{}
	for offset := 0; offset < 100000; {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		rows := []APIMessage{}
		header, err := g.request(ctx, "GET", fmt.Sprintf("%s%slimit=500&offset=%d", path, separator, offset), nil, &rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
		offset += len(rows)
		total, totalErr := strconv.Atoi(header.Get("X-Total-Count"))
		if len(rows) < 500 || (totalErr == nil && offset >= total) {
			return result, nil
		}
	}
	return nil, errors.New("history exceeds 100,000 records")
}
func (g *Gateway) message(ctx context.Context, id string) (APIMessage, error) {
	var row APIMessage
	_, err := g.request(ctx, "GET", "/message/"+url.PathEscape(id), nil, &row)
	if err == nil && row.ID == "" {
		err = &APIError{"missing message ID", 0, false}
	}
	return row, err
}
func (g *Gateway) sync(ctx context.Context, known map[string]Message) ([]Message, error) {
	return g.syncSince(ctx, known, time.Time{})
}
func (g *Gateway) syncSince(ctx context.Context, known map[string]Message, since time.Time) ([]Message, error) {
	inboxPath, outboxPath := "/inbox?includeAttachments=true", "/messages"
	if !since.IsZero() {
		filter := "from=" + url.QueryEscape(since.Add(-5*time.Minute).UTC().Format(time.RFC3339))
		inboxPath += "&" + filter
		outboxPath += "?" + filter
	}
	// Request message bodies in a single list only when filling an empty cache.
	if len(known) == 0 {
		if strings.Contains(outboxPath, "?") {
			outboxPath += "&includeContent=true"
		} else {
			outboxPath += "?includeContent=true"
		}
	}
	type listResult struct {
		rows []APIMessage
		err  error
	}
	inboxResult := make(chan listResult, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { rows, err := g.listing(ctx, inboxPath); inboxResult <- listResult{rows, err} }()
	outbox, err := g.listing(ctx, outboxPath)
	if err != nil {
		cancel()
		<-inboxResult
		return nil, err
	}
	incoming := <-inboxResult
	if incoming.err != nil {
		return nil, incoming.err
	}
	inbox := incoming.rows
	result := []Message{}
	seen := map[string]bool{}
	for _, row := range inbox {
		result = append(result, row.messages()...)
	}
	for _, row := range outbox {
		if row.ID == "" {
			continue
		}
		seen[row.ID] = true
		if row.Body == nil && row.MMS == nil {
			if m, ok := known[row.ID]; ok && m.Text != "[Text unavailable]" {
				row.Body = &struct {
					Text string `json:"text"`
				}{m.Text}
			} else {
				row, err = g.message(ctx, row.ID)
				if err != nil {
					return nil, err
				}
			}
		}
		messages := row.messages()
		if old, ok := known[row.ID]; ok {
			for i := range messages {
				messages[i] = mergeMessage(old, messages[i])
			}
		}

		result = append(result, messages...)
	}
	for id, m := range known {
		if !seen[id] && (m.State == "Uncertain" || (!since.IsZero() && (m.State == "Pending" || m.State == "Processed" || (m.State == "Sent" && m.Type != "MMS" && time.Since(m.stamp()) < 48*time.Hour)))) {
			row, err := g.message(ctx, id)
			if err != nil {
				var api *APIError
				if errors.As(err, &api) && api.Status == 404 {
					continue
				}
				return nil, err
			}
			if row.Body == nil {
				row.Body = &struct {
					Text string `json:"text"`
				}{m.Text}
			}
			messages := row.messages()
			for i := range messages {
				messages[i] = mergeMessage(m, messages[i])
			}

			result = append(result, messages...)
		}
	}
	return result, nil
}
func (g *Gateway) send(ctx context.Context, id, number, text string) ([]Message, error) {
	body := map[string]any{"id": id, "textMessage": map[string]string{"text": text}, "phoneNumbers": []string{number}, "withDeliveryReport": true}
	var row APIMessage
	_, err := g.request(ctx, "POST", "/message", body, &row)
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, &APIError{"send outcome unknown", 0, true}
	}
	if row.Body == nil {
		row.Body = &struct {
			Text string `json:"text"`
		}{text}
	}
	return row.messages(), nil
}
func (g *Gateway) importHistory(ctx context.Context) error {
	_, err := g.request(ctx, "POST", "/inbox/refresh", map[string]any{"since": "1970-01-01T00:00:00Z", "until": time.Now().UTC().Format(time.RFC3339), "messageTypes": []string{"SMS", "MMS"}, "webhookDelivery": "Disabled", "triggerWebhooks": false}, nil)
	return err
}
func messageID() string {
	data := make([]byte, 15)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
