package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const sendPath = "/voice/v1/voiceclient/api2thread/sendsms"
const composerSelector = `gv-message-entry textarea[placeholder="Type a message"]`

func recipientDigits(number string) []string {
	values := []string{strings.TrimPrefix(number, "+")}
	if strings.HasPrefix(number, "+1") && len(number) == 12 {
		values = append(values, number[2:])
	}
	return values
}
func recipientReadyJS(number string) string {
	return `(()=>{const d=s=>(s||'').replace(/\D/g,'');const allowed=` + literal(recipientDigits(number)) + `;const header=document.querySelector('gv-thread-details-header .header-title');const chips=[...document.querySelectorAll('mat-chip,mat-chip-row,md-chip,.recipient-chip')];const selected=header?allowed.includes(d(header.innerText)):chips.length===1&&allowed.includes(d(chips[0].innerText));return selected&&!!document.querySelector('` + composerSelector + `')})()`
}

func (b *Browser) openRecipient(number string) error {
	var selected bool
	js := `(()=>{const allowed=` + literal(recipientDigits(number)) + `;const rows=[...document.querySelectorAll('gv-thread-list-item')].filter(e=>allowed.includes((e.querySelector('.participants')?.innerText||'').replace(/\D/g,'')));if(rows.length!==1)return false;rows[0].querySelector('[role="button"]').click();return true})()`
	if e := b.run(chromedp.Evaluate(js, &selected)); e != nil {
		return e
	}
	if !selected {
		return b.openNewRecipient(number)
	}
	return b.run(chromedp.Poll(recipientReadyJS(number), &selected, chromedp.WithPollingTimeout(15*time.Second)))
}

func (b *Browser) openNewRecipient(number string) error {
	var selected bool
	if e := b.run(chromedp.Click(`[aria-label="Send new message"]`, chromedp.ByQuery), chromedp.WaitVisible(`input[placeholder="Type a name or phone number"]`, chromedp.ByQuery), chromedp.SendKeys(`input[placeholder="Type a name or phone number"]`, number, chromedp.ByQuery)); e != nil {
		return e
	}
	js := `(()=>{const allowed=` + literal(recipientDigits(number)) + `;const options=[...document.querySelectorAll('[role="option"],.send-to-button')].filter(e=>allowed.includes((e.querySelector('.send-to-label')?.innerText||e.getAttribute('aria-label')||e.innerText||'').replace(/\D/g,'')));if(options.length!==1)return false;options[0].click();return true})()`
	if e := b.run(chromedp.Poll(js, &selected, chromedp.WithPollingTimeout(15*time.Second))); e != nil {
		return e
	}
	return b.run(chromedp.Poll(recipientReadyJS(number), &selected, chromedp.WithPollingTimeout(15*time.Second)))
}

func sendToken(account, id string) string {
	sum := sha256.Sum256([]byte(account + "\x00" + id))
	var token uint64
	for _, v := range sum[:6] {
		token = token<<8 | uint64(v)
	}
	if token == 0 {
		token = 1
	}
	return strconv.FormatUint(token, 10)
}

// Only replace the app-generated correlation ID. Its freshly generated
// authentication/abuse-protection fields remain unchanged and are never logged.
func prepareSendPayload(body []byte, m Message) ([]byte, error) {
	if m.Body == nil || len(m.Recipients) != 1 {
		return nil, errors.New("invalid send envelope")
	}
	var fields []json.RawMessage
	if e := json.Unmarshal(body, &fields); e != nil || len(fields) < 9 {
		return nil, errors.New("unrecognized Google Voice send request")
	}
	var text, thread string
	if json.Unmarshal(fields[4], &text) != nil || json.Unmarshal(fields[5], &thread) != nil || text != m.Body.Text || thread != "t."+m.Recipients[0].Number {
		return nil, errors.New("Google Voice send request recipient or text mismatch")
	}
	token, e := strconv.ParseUint(m.SendToken, 10, 48)
	if e != nil || token == 0 {
		return nil, errors.New("invalid send correlation token")
	}
	fields[8], _ = json.Marshal([]uint64{token})
	return json.Marshal(fields)
}

func (b *Browser) send(store *Store, m Message) (result Message) {
	result = m
	if m.Body == nil || len(m.Recipients) != 1 || m.Account != b.config.Number {
		setState(&result, "Failed", "invalid send account or recipient")
		return
	}
	if e := b.openRecipient(m.Recipients[0].Number); e != nil {
		setState(&result, "Failed", "cannot verify selected recipient; no send attempted")
		return
	}
	var ready bool
	js := `(()=>{const e=document.querySelector('` + composerSelector + `');if(!e||e.value!=='')return false;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,` + literal(m.Body.Text) + `);e.dispatchEvent(new Event('input',{bubbles:true}));return e.value===` + literal(m.Body.Text) + `})()`
	if e := b.run(chromedp.Evaluate(js, &ready)); e != nil || !ready {
		setState(&result, "Failed", "composer unavailable or contains a draft; no send attempted")
		return
	}
	if e := b.run(chromedp.Poll(`!!document.querySelector('gv-message-entry button[aria-label="Send message"]:not([disabled])')`, &ready, chromedp.WithPollingTimeout(10*time.Second))); e != nil {
		setState(&result, "Failed", "send button unavailable; no send attempted")
		return
	}
	listenerCtx, stop := context.WithCancel(b.ctx)
	defer stop()
	paused := make(chan *fetch.EventRequestPaused, 4)
	completed := make(chan network.RequestID, 4)
	var claimed atomic.Bool
	var mu sync.Mutex
	responses := map[network.RequestID]int64{}
	chromedp.ListenTarget(listenerCtx, func(event any) {
		switch e := event.(type) {
		case *fetch.EventRequestPaused:
			if claimed.CompareAndSwap(false, true) {
				paused <- e
			} else {
				// The page must never automatically retry a send behind the journal.
				go func() { _ = b.run(fetch.FailRequest(e.RequestID, network.ErrorReasonAborted)) }()
			}
		case *network.EventResponseReceived:
			if strings.Contains(e.Response.URL, sendPath) && e.Type == network.ResourceTypeXHR {
				mu.Lock()
				responses[e.RequestID] = e.Response.Status
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			_, ok := responses[e.RequestID]
			mu.Unlock()
			if ok {
				select {
				case completed <- e.RequestID:
				default:
				}
			}
		}
	})
	if e := b.run(fetch.Enable().WithPatterns([]*fetch.RequestPattern{{URLPattern: "https://clients6.google.com" + sendPath + "*", ResourceType: network.ResourceTypeXHR, RequestStage: fetch.RequestStageRequest}})); e != nil {
		setState(&result, "Failed", "cannot establish send journal boundary; no send attempted")
		return
	}
	defer func() {
		// Stop late app retries before lifting interception on an unknown outcome.
		if result.State != "Sent" {
			b.close()
		} else {
			_ = b.run(fetch.Disable())
		}
	}()
	result.SendToken = sendToken(m.Account, m.ID)
	result.Submitted = true
	result.AttemptedAt = now()
	setState(&result, "Processed", "")
	if e := store.put(result); e != nil {
		setState(&result, "Failed", "cannot persist send journal; no send attempted")
		return
	}
	// From here every unknown outcome stays uncertain; never click or POST again.
	setState(&result, "Uncertain", "send outcome unconfirmed; reconciliation only, never automatic resend")
	js = `(()=>{if(!(` + recipientReadyJS(m.Recipients[0].Number) + `))return false;const e=document.querySelector('` + composerSelector + `');const b=document.querySelector('gv-message-entry button[aria-label="Send message"]');if(!e||e.value!==` + literal(m.Body.Text) + `||!b||b.disabled)return false;b.click();return true})()`
	if e := b.run(chromedp.Evaluate(js, &ready)); e != nil {
		return
	}
	if !ready {
		setState(&result, "Failed", "recipient or composer changed; no send attempted")
		return
	}
	var request *fetch.EventRequestPaused
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	select {
	case request = <-paused:
	case <-b.ctx.Done():
		return
	case <-timer.C:
		return
	}
	// Keep the network request paused until its exact content and saved token
	// have been checked. A rejected request is aborted, never released by Disable.
	continued := false
	defer func() {
		if !continued {
			_ = b.run(fetch.FailRequest(request.RequestID, network.ErrorReasonAborted))
		}
	}()
	var body []byte
	for _, entry := range request.Request.PostDataEntries {
		data, e := base64.StdEncoding.DecodeString(entry.Bytes)
		if e != nil {
			setState(&result, "Failed", "cannot inspect send request; request aborted")
			return
		}
		body = append(body, data...)
	}
	if len(body) == 0 {
		var text string
		if e := b.run(chromedp.ActionFunc(func(ctx context.Context) error {
			var e error
			text, e = network.GetRequestPostData(request.NetworkID).Do(ctx)
			return e
		})); e != nil {
			setState(&result, "Failed", "cannot inspect send request; request aborted")
			return
		}
		body = []byte(text)
	}
	prepared, e := prepareSendPayload(body, result)
	if e != nil {
		setState(&result, "Failed", e.Error()+"; request aborted")
		return
	}
	// The token was durable before the UI click and remains durable before the
	// paused request is released. A crash after this point is reconciled by token.
	if e = b.run(fetch.ContinueRequest(request.RequestID).WithPostData(base64.StdEncoding.EncodeToString(prepared))); e != nil {
		return
	}
	continued = true
	var id network.RequestID
	select {
	case id = <-completed:
	case <-b.ctx.Done():
		return
	case <-timer.C:
		return
	}
	mu.Lock()
	status := responses[id]
	mu.Unlock()
	if status == 401 || status == 403 {
		setState(&result, "Failed", errAuthPaused.Error())
		return
	}
	if status >= 400 && status < 500 {
		setState(&result, "Failed", "Google Voice rejected the send request")
		return
	}
	if status != 200 {
		return
	}
	var response []byte
	if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
		var e error
		response, e = network.GetResponseBody(id).Do(ctx)
		return e
	})); e != nil {
		return
	}
	var fields []json.RawMessage
	var source, thread string
	if json.Unmarshal(response, &fields) != nil || len(fields) < 4 || json.Unmarshal(fields[1], &thread) != nil || thread != "t."+m.Recipients[0].Number || json.Unmarshal(fields[2], &source) != nil || source == "" {
		return
	}
	result.Source = source
	rows, e := b.readNetworkHistory()
	if e != nil {
		return
	}
	for _, row := range rows {
		if row.Source == source && row.Account == m.Account && row.Body != nil && row.Body.Text == m.Body.Text && len(row.Recipients) == 1 && row.Recipients[0].Number == m.Recipients[0].Number && row.SendToken == result.SendToken {
			result.Created = row.Created
			setState(&result, "Sent", "")
			return
		}
	}
	return
}
