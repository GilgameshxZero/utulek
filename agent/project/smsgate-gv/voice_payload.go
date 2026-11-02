package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Observed Google Voice JSON+protobuf fields. Unknown message kinds and missing
// identity/time fields are errors, never an invitation to synthesize an ID.
type VoiceMessage struct {
	ClientID      string   `json:"clientId,omitempty"`
	ID            string   `json:"id"`
	Timestamp     string   `json:"timestamp"`
	Text          string   `json:"text"`
	Outgoing      bool     `json:"outgoing"`
	Subject       string   `json:"subject,omitempty"`
	AttachmentIDs []string `json:"attachmentIds,omitempty"`
}

func voiceThread(body []byte, threadID string) ([]json.RawMessage, error) {
	var envelope []json.RawMessage
	if e := json.Unmarshal(body, &envelope); e != nil || len(envelope) < 1 {
		return nil, errors.New("invalid Google Voice thread response")
	}
	var threads [][]json.RawMessage
	if e := json.Unmarshal(envelope[0], &threads); e != nil {
		return nil, errors.New("invalid Google Voice thread list")
	}
	for _, thread := range threads {
		var id string
		if len(thread) > 0 && json.Unmarshal(thread[0], &id) == nil && id == threadID {
			return thread, nil
		}
	}
	return nil, errors.New("requested Google Voice thread not found in response")
}

func decodeVoiceThread(thread []json.RawMessage, ownNumber string) ([]VoiceMessage, error) {
	if len(thread) < 3 {
		return nil, errors.New("truncated Google Voice thread")
	}
	var records [][]json.RawMessage
	if e := json.Unmarshal(thread[2], &records); e != nil {
		return nil, errors.New("invalid Google Voice messages")
	}
	result := make([]VoiceMessage, 0, len(records))
	seen := map[string]bool{}
	for _, fields := range records {
		if len(fields) < 15 {
			return nil, errors.New("truncated Google Voice message")
		}
		var m VoiceMessage
		var timestamp int64
		var account string
		var kind int
		if e := json.Unmarshal(fields[0], &m.ID); e != nil || m.ID == "" || len(m.ID) > 128 {
			return nil, errors.New("missing Google Voice message ID")
		}
		if seen[m.ID] {
			return nil, errors.New("duplicate Google Voice message ID in response")
		}
		seen[m.ID] = true
		if e := json.Unmarshal(fields[1], &timestamp); e != nil || timestamp < 946684800000 || timestamp > 7258118400000 {
			return nil, errors.New("invalid Google Voice millisecond timestamp")
		}
		if e := json.Unmarshal(fields[2], &account); e != nil || account != ownNumber {
			return nil, errors.New("Google Voice message account mismatch")
		}
		if e := json.Unmarshal(fields[4], &kind); e != nil {
			return nil, e
		}
		switch kind {
		case 10:
			m.Outgoing = false
		case 11:
			m.Outgoing = true
		default:
			return nil, fmt.Errorf("unsupported Google Voice message kind %d", kind)
		}
		if e := json.Unmarshal(fields[9], &m.Text); e != nil {
			return nil, errors.New("invalid Google Voice text")
		}
		m.Timestamp = time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano)
		if len(fields) > 17 && string(fields[17]) != "null" {
			if e := json.Unmarshal(fields[17], &m.ClientID); e != nil {
				return nil, errors.New("invalid Google Voice client correlation ID")
			}
		}
		if string(fields[14]) != "null" {
			var media []json.RawMessage
			if e := json.Unmarshal(fields[14], &media); e != nil || len(media) < 3 {
				return nil, errors.New("invalid Google Voice media message")
			}
			if e := json.Unmarshal(media[0], &m.Text); e != nil {
				return nil, e
			}
			if e := json.Unmarshal(media[1], &m.Subject); e != nil {
				return nil, e
			}
			if string(media[2]) != "null" {
				var attachments [][]json.RawMessage
				if e := json.Unmarshal(media[2], &attachments); e != nil {
					return nil, e
				}
				for _, attachment := range attachments {
					var id string
					if len(attachment) < 2 || json.Unmarshal(attachment[1], &id) != nil || id == "" {
						return nil, errors.New("invalid Google Voice attachment ID")
					}
					m.AttachmentIDs = append(m.AttachmentIDs, id)
				}
			}
		}
		result = append(result, m)
	}
	return result, nil
}
