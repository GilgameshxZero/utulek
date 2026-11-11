package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type historyRequest struct {
	URL         string
	Method      string
	HasPostData bool
}

const threadListPath = "/voice/v1/voiceclient/api2thread/list"

// Observe only the page's completed history request. No request headers, cookies,
// unrelated response bodies, or raw history are written to logs or disk.
func (b *Browser) readNetworkHistory() ([]Message, error) {
	if b.historyRequest != nil {
		return b.readIncremental()
	}
	b.watchUpdates()
	listenCtx, stop := context.WithCancel(b.ctx)
	defer stop()
	var mu sync.Mutex
	pending := map[network.RequestID]bool{}
	requests := map[network.RequestID]*historyRequest{}
	completed := make(chan network.RequestID, 8)
	chromedp.ListenTarget(listenCtx, func(event any) {
		mu.Lock()
		defer mu.Unlock()
		switch e := event.(type) {
		case *network.EventRequestWillBeSent:
			u, err := url.Parse(e.Request.URL)
			if err == nil && u.Hostname() == "clients6.google.com" && u.Path == threadListPath {
				requests[e.RequestID] = &historyRequest{URL: e.Request.URL, Method: e.Request.Method, HasPostData: e.Request.HasPostData}
			}
		case *network.EventResponseReceived:
			u, err := url.Parse(e.Response.URL)
			if err == nil && u.Hostname() == "clients6.google.com" && u.Path == threadListPath && e.Type == network.ResourceTypeXHR && e.Response.Status == 200 {
				pending[e.RequestID] = true
			}
		case *network.EventLoadingFinished:
			if pending[e.RequestID] {
				delete(pending, e.RequestID)
				select {
				case completed <- e.RequestID:
				default:
				}
			}
		case *network.EventLoadingFailed:
			delete(pending, e.RequestID)
		}
	})
	if e := b.run(network.Enable(), chromedp.Navigate(b.base+"messages")); e != nil {
		return nil, e
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	case <-timer.C:
		return nil, errors.New("Google Voice history response unavailable")
	case id := <-completed:
		mu.Lock()
		request := requests[id]
		mu.Unlock()
		var postData string
		if request != nil && request.HasPostData {
			if e := b.run(chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				postData, err = network.GetRequestPostData(id).Do(ctx)
				return err
			})); e != nil {
				return nil, errors.New("cannot read history request structure")
			}
		}
		var body []byte
		if e := b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			body, err = network.GetResponseBody(id).Do(ctx)
			return err
		})); e != nil {
			return nil, errors.New("cannot read Google Voice history response")
		}
		_, e := decodeHistory(body, b.config)
		if e == nil {
			b.historyRequest = request
			b.historyPostBody = postData
		}
		if e != nil {
			return nil, e
		}
		return b.readIncremental()
	}
}

func nativeBridgeID(account, nativeID string) string {
	sum := sha256.Sum256([]byte(account + "\x00" + nativeID))
	return "gv-" + hex.EncodeToString(sum[:])
}

func bridgeAttachments(items []VoiceAttachment) []Attachment {
	out := make([]Attachment, 0, len(items))
	for i, item := range items {
		name := fmt.Sprintf("attachment-%d", i+1)
		if extension := preferredExtension(item.ContentType); extension != "" {
			name += extension
		} else if extensions, e := mime.ExtensionsByType(item.ContentType); e == nil && len(extensions) > 0 {
			name += extensions[0]
		}
		out = append(out, Attachment{PartID: int64(i + 1), Name: name, ContentType: item.ContentType, SourceID: item.ID})
	}
	return out
}

func preferredExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/3gpp":
		return ".3gp"
	case "audio/mpeg":
		return ".mp3"
	case "audio/amr":
		return ".amr"
	case "audio/ogg":
		return ".ogg"
	case "text/plain":
		return ".txt"
	}
	return ""
}

func decodeHistory(body []byte, c Config) ([]Message, error) {
	var envelope []json.RawMessage
	if e := json.Unmarshal(body, &envelope); e != nil || len(envelope) < 1 {
		return nil, errors.New("invalid Google Voice history response")
	}
	var threads [][]json.RawMessage
	if e := json.Unmarshal(envelope[0], &threads); e != nil {
		return nil, errors.New("invalid Google Voice history thread list")
	}
	allowed := map[string]bool{}
	for _, number := range c.SyncNumbers {
		allowed[number] = true
	}
	result := []Message{}
	selected := 0
	for _, thread := range threads {
		var threadID string
		if len(thread) < 1 || json.Unmarshal(thread[0], &threadID) != nil {
			return nil, errors.New("invalid Google Voice thread identity")
		}
		if !strings.HasPrefix(threadID, "t.") && !strings.HasPrefix(threadID, "g.") {
			continue
		}
		group := strings.HasPrefix(threadID, "g.")
		number := strings.TrimPrefix(threadID, "t.")
		if !group && (!historyPeer(number) || (len(allowed) > 0 && !allowed[number])) {
			continue
		}
		if selected >= c.MaxThreads {
			break
		}
		rows, e := decodeVoiceThread(thread, c.Number)
		if e != nil {
			return nil, e
		}
		if group && len(allowed) > 0 {
			included := false
			for _, r := range rows {
				for _, participant := range r.Participants {
					if allowed[participant] {
						included = true
					}
				}
			}
			if !included {
				continue
			}
		}
		selected++
		for _, r := range rows {
			m := Message{Account: c.Number, ID: nativeBridgeID(c.Number, r.ID), Source: r.ID, Created: r.Timestamp, Type: "SMS"}
			text := r.Text
			if group {
				m.Type = "MMS"
				m.GroupID = r.GroupID
				m.Participants = append([]string(nil), r.Participants...)
			}
			if len(r.Attachments) > 0 || r.Subject != "" {
				m.Type = "MMS"
			}
			if len(r.Attachments) > 0 {
				m.Attachments = bridgeAttachments(r.Attachments)
			}
			if group {
				m.MMS = &MediaText{Text: text, Subject: r.GroupName}
			}
			if r.Outgoing {
				m.SendToken = r.ClientID
				m.State = "Sent"
				m.Body = &Text{Text: text}
				if group {
					for _, participant := range r.Participants {
						if participant != c.Number {
							m.Recipients = append(m.Recipients, Recipient{participant, "Sent"})
						}
					}
				} else {
					m.Recipients = []Recipient{{Number: number, State: "Sent"}}
				}
				if m.Type == "MMS" && !group {
					m.MMS = &MediaText{Text: text, Subject: r.Subject}
				}
			} else {
				m.State = "Received"
				m.Sender = number
				if group {
					m.Sender = r.GroupSender
				}
				m.Preview = text
				if r.Subject != "" {
					m.Preview = "Subject: " + r.Subject + "\n" + m.Preview
				}
			}
			result = append(result, m)
		}
	}
	return result, nil
}

func (b *Browser) voiceEndpoint(path string, body any) ([]byte, error) {
	b.apiInFlight.Add(1)
	defer b.apiInFlight.Add(-1)
	if b.historyRequest == nil {
		return nil, errors.New("Google Voice API transport not initialized")
	}
	u, e := url.Parse(b.historyRequest.URL)
	if e != nil {
		return nil, e
	}
	u.Path = path
	var response struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	request := map[string]any{"path": u.String(), "method": "POST", "body": body, "headers": map[string]string{"Content-Type": "application/json+protobuf"}}
	js := `(async()=>{try{const r=await gapi.client.request(` + literal(request) + `);return {status:r.status,body:r.body||''}}catch(e){return {status:e.status||0,body:''}}})()`
	if e = b.run(chromedp.Evaluate(js, &response, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) })); e != nil {
		return nil, errors.New("Google Voice endpoint transport unavailable")
	}
	if response.Status == 401 || response.Status == 403 {
		return nil, errAuthPaused
	}
	if response.Status != 200 {
		return nil, fmt.Errorf("Google Voice endpoint HTTP %d", response.Status)
	}
	return []byte(response.Body), nil
}
func (b *Browser) refreshHistoryEndpoint() ([]Message, error) {
	body, e := b.voiceEndpoint(threadListPath, b.historyPostBody)
	if e != nil {
		return nil, e
	}
	return decodeHistory(body, b.config)
}
