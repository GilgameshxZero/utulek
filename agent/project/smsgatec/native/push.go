package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// Credentials and certificate material stay beside the private configuration.
type pushIdentity struct{ Token, Certificate, Key string }
type pushPart struct {
	Attachment
	Data []byte `json:"data"`
}
type pushEvent struct {
	Messages []APIMessage `json:"messages,omitempty"`
	ID       string       `json:"id"`
	Event    string       `json:"event"`
	Payload  struct {
		ID            string     `json:"messageId"`
		Received      string     `json:"receivedAt"`
		Sender        string     `json:"sender"`
		Message       string     `json:"message"`
		Body          string     `json:"body"`
		Subject       string     `json:"subject"`
		TransactionID string     `json:"transactionId"`
		Attachments   []pushPart `json:"attachments"`
	} `json:"payload"`
}
type pushListener struct {
	streamCancel context.CancelFunc
	endpoint     string
	server       *http.Server
	gateway      *Gateway
	ids          []string
	events       chan pushEvent
	done         chan error
}

var pushEvents = []string{"sms:received", "mms:received", "sms:sent", "sms:delivered", "sms:failed", "sms:cancelled", "app:started"}

func pushAddress(ctx context.Context, server string) (net.IP, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("missing gateway host")
	}
	// UDP connect selects the route without transmitting a packet.
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", net.JoinHostPort(host, "9"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	ip := conn.LocalAddr().(*net.UDPAddr).IP
	if !ip.IsPrivate() {
		return nil, errors.New("push requires a private network route to the phone")
	}
	return ip, nil
}

func certificateRequest(ctx context.Context, ip net.IP) (string, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: ip.String()}, IPAddresses: []net.IP{ip}}, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	body, _ := json.Marshal(map[string]string{"type": "webhook", "content": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))})
	client := &http.Client{Timeout: 20 * time.Second}
	defer client.CloseIdleConnections()
	var response struct {
		ID          string `json:"request_id"`
		Status      string `json:"status"`
		Certificate string `json:"certificate"`
	}
	request := func(method, path string, data []byte) error {
		req, err := http.NewRequestWithContext(ctx, method, "https://ca.sms-gate.app/api/v1"+path, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return errors.New("certificate authority unreachable")
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("certificate authority HTTP %d", res.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(res.Body, 128*1024)).Decode(&response)
	}
	if err = request("POST", "/csr", body); err != nil {
		return "", "", err
	}
	for response.Certificate == "" {
		if response.Status == "denied" || response.ID == "" {
			return "", "", errors.New("webhook certificate was not approved")
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(time.Second):
		}
		if err = request("GET", "/csr/"+url.PathEscape(response.ID), nil); err != nil {
			return "", "", err
		}
	}
	return response.Certificate, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})), nil
}

func loadPushIdentity(ctx context.Context, path string, ip net.IP) (pushIdentity, tls.Certificate, error) {
	var identity pushIdentity
	name := filepath.Join(filepath.Dir(path), "push", strings.ReplaceAll(ip.String(), ":", "_")+".json")
	if data, err := os.ReadFile(name); err == nil {
		if err = json.Unmarshal(data, &identity); err != nil {
			return identity, tls.Certificate{}, errors.New("invalid private push identity")
		}
	}
	if identity.Token == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return identity, tls.Certificate{}, err
		}
		identity.Token = hex.EncodeToString(token)
	}
	cert, err := tls.X509KeyPair([]byte(identity.Certificate), []byte(identity.Key))
	if err == nil {
		leaf, e := x509.ParseCertificate(cert.Certificate[0])
		if e == nil && leaf.VerifyHostname(ip.String()) == nil && time.Until(leaf.NotAfter) > 7*24*time.Hour {
			return identity, cert, nil
		}
	}
	identity.Certificate, identity.Key, err = certificateRequest(ctx, ip)
	if err != nil {
		return identity, tls.Certificate{}, err
	}
	cert, err = tls.X509KeyPair([]byte(identity.Certificate), []byte(identity.Key))
	if err != nil {
		return identity, cert, err
	}
	data, _ := json.Marshal(identity)
	err = privateWrite(name, data)
	return identity, cert, err
}

func (p *pushListener) handler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var event pushEvent
		// MMS events may contain base64 media, but it is neither retained nor cached here.
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024*1024)).Decode(&event) != nil || event.Event == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case p.events <- event:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusServiceUnavailable) // Phone retries; never silently discard.
		}
	})
}

func startPush(ctx context.Context, g *Gateway, path string) (*pushListener, error) {
	if u, e := url.Parse(g.config.Server); e == nil {
		if u.Hostname() != "" {
			var health struct {
				Backend      string   `json:"backend"`
				Capabilities []string `json:"capabilities"`
			}
			if _, e = g.request(ctx, "GET", "/health", nil, &health); e == nil && health.Backend == "google_voice" {
				for _, capability := range health.Capabilities {
					if capability == "event-stream" {
						return startBridgeStream(ctx, g)
					}
				}
			}
		}
	}
	setup, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ip, err := pushAddress(setup, g.config.Server)
	if err != nil {
		return nil, err
	}
	// A fixed port makes crash recovery replace our registrations rather than add duplicates.
	listener, err := net.Listen("tcp", net.JoinHostPort(ip.String(), "8766"))
	if err != nil {
		return nil, errors.New("push port 8766 is unavailable")
	}
	identity, cert, err := loadPushIdentity(setup, path, ip)
	if err != nil {
		listener.Close()
		return nil, err
	}
	p := &pushListener{gateway: g, events: make(chan pushEvent, 64), done: make(chan error, 1)}
	route := "/events/" + identity.Token
	p.server = &http.Server{Handler: p.handler(route), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	secure := tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	go func() { p.done <- p.server.Serve(secure) }()
	endpoint := "https://" + net.JoinHostPort(ip.String(), "8766") + route
	p.endpoint = endpoint
	for _, event := range pushEvents {
		hash := sha256.Sum256([]byte(endpoint + event))
		id := "sg-" + hex.EncodeToString(hash[:16])
		// Track before POST so a lost response is cleaned up too.
		p.ids = append(p.ids, id)
		if _, err = g.request(setup, "POST", "/webhooks", map[string]string{"id": id, "url": endpoint, "event": event}, nil); err != nil {
			p.close()
			return nil, err
		}
	}
	return p, nil
}
func (p *pushListener) close() {
	if p == nil {
		return
	}
	if p.streamCancel != nil {
		p.streamCancel()
		return
	}
	// Deregister first so a clean exit does not leave retries targeting a closed app.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range p.ids {
		_, _ = p.gateway.request(ctx, "DELETE", "/webhooks/"+url.PathEscape(id), nil, nil)
	}
	_ = p.server.Close()
	p.gateway.client.CloseIdleConnections()
}

// Upstream IncomingMessagesService.buildId and ReceiverService use the same
// signed Java hash in decimal (inbox) and unsigned hex (SMS webhook).
// Reuse that identity so notifications need no incoming-message GET at all.
func javaHash(s string) int32 {
	var h int32
	for _, c := range utf16.Encode([]rune(s)) {
		h = 31*h + int32(c)
	}
	return h
}
func (e pushEvent) incoming() (Message, bool) {
	p := e.Payload
	if p.Sender == "" || p.ID == "" {
		return Message{}, false
	}
	if _, err := time.Parse(time.RFC3339Nano, p.Received); err != nil {
		return Message{}, false
	}
	m := Message{Number: phoneKey(p.Sender), Direction: "in", Timestamp: p.Received, State: "Received"}
	switch e.Event {
	case "sms:received":
		hash, err := strconv.ParseUint(p.ID, 16, 32)
		if err != nil {
			return Message{}, false
		}
		m.ID = "text:" + strconv.FormatInt(int64(int32(hash)), 10)
		m.Type, m.Text = "SMS", p.Message
	case "mms:received":
		if p.TransactionID == "" {
			return Message{}, false
		}
		m.ID = "mms-header:" + strconv.FormatInt(int64(javaHash(p.TransactionID)), 10)
		m.Type, m.Text, m.Subject = "MMS", p.Subject, p.Subject
		if m.Text == "" {
			m.Text = "MMS notification"
		}
	case "mms:downloaded":
		m.ID = "mms:" + strconv.FormatInt(int64(javaHash(p.ID)), 10)
		m.Type, m.Text, m.Subject = "MMS_DOWNLOADED", p.Body, p.Subject
		for _, p := range p.Attachments {
			part := p.Attachment
			part.Inline = p.Data
			m.Media = append(m.Media, part)
		}
	default:
		return Message{}, false
	}
	return m, true
}
func (g *Gateway) eventMessages(ctx context.Context, events []pushEvent) ([]Message, error) {
	messages := []Message{}
	ids := map[string]bool{}
	var since time.Time
	for _, e := range events {
		if e.Event == "bridge:reset" {
			rows, err := g.sync(ctx, nil)
			if err != nil {
				return nil, err
			}
			messages = append(messages, rows...)
			continue
		}
		if e.Event == "bridge:messages" {
			for _, row := range e.Messages {
				messages = append(messages, row.messages()...)
			}
			continue
		}
		if m, ok := e.incoming(); ok {
			messages = append(messages, m)
			continue
		}
		if e.Event == "sms:received" || strings.HasPrefix(e.Event, "mms:") {
			t, err := time.Parse(time.RFC3339Nano, e.Payload.Received)
			if err != nil {
				return nil, errors.New("invalid incoming event timestamp")
			}
			t = t.Add(-2 * time.Minute)
			if since.IsZero() || t.Before(since) {
				since = t
			}
		} else if strings.HasPrefix(e.Event, "sms:") && e.Payload.ID != "" {
			ids[e.Payload.ID] = true
		}
	}
	if !since.IsZero() {
		rows, err := g.listing(ctx, "/inbox?includeAttachments=true&from="+url.QueryEscape(since.Format(time.RFC3339)))
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			messages = append(messages, r.messages()...)
		}
	}
	// Read authoritative recipient states for multipart SMS delivery reports.
	for id := range ids {
		row, err := g.message(ctx, id)
		if err != nil {
			return nil, err
		}
		messages = append(messages, row.messages()...)
	}
	return messages, nil
}
