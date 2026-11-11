package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Observed Google Voice JSON+protobuf fields. Unknown message kinds and missing
// identity/time fields are errors, never an invitation to synthesize an ID.
type VoiceAttachment struct {
	ID          string
	ContentType string
	Width       int
	Height      int
}

type VoiceMessage struct {
	ClientID     string            `json:"clientId,omitempty"`
	ID           string            `json:"id"`
	Timestamp    string            `json:"timestamp"`
	Text         string            `json:"text"`
	Outgoing     bool              `json:"outgoing"`
	Subject      string            `json:"subject,omitempty"`
	Attachments  []VoiceAttachment `json:"attachments,omitempty"`
	GroupID      string            `json:"groupId,omitempty"`
	GroupName    string            `json:"groupName,omitempty"`
	GroupSender  string            `json:"groupSender,omitempty"`
	Participants []string          `json:"participants,omitempty"`
}

func decodeGroupMetadata(fields []json.RawMessage) (name, sender, body string, participants []string, attachments []VoiceAttachment, ok bool) {
	if len(fields) <= 14 || string(fields[14]) == "null" {
		return "", "", "", nil, nil, false
	}
	var media []json.RawMessage
	if json.Unmarshal(fields[14], &media) != nil || len(media) < 5 {
		return "", "", "", nil, nil, false
	}
	_ = json.Unmarshal(media[0], &name)
	_ = json.Unmarshal(media[1], &body)
	_ = json.Unmarshal(media[4], &sender)
	if len(media) > 5 {
		_ = json.Unmarshal(media[5], &participants)
	}
	if len(participants) == 0 && len(media) > 3 {
		var rows [][]json.RawMessage
		if json.Unmarshal(media[3], &rows) == nil {
			for _, row := range rows {
				var number string
				if len(row) > 0 && json.Unmarshal(row[0], &number) == nil && phonePattern.MatchString(number) {
					participants = append(participants, number)
				}
			}
		}
	}
	if len(media) > 2 && string(media[2]) != "null" {
		attachments, _ = parseVoiceAttachments(media[2])
	}
	return name, sender, body, participants, attachments, name != "" || len(participants) > 0
}

func parseVoiceAttachments(raw json.RawMessage) ([]VoiceAttachment, error) {
	var attachments [][]json.RawMessage
	if json.Unmarshal(raw, &attachments) != nil {
		return nil, errors.New("invalid Google Voice attachments")
	}
	out := []VoiceAttachment{}
	for _, attachment := range attachments {
		var id, contentType string
		if len(attachment) < 2 || json.Unmarshal(attachment[1], &id) != nil || id == "" {
			return nil, errors.New("invalid Google Voice attachment ID")
		}
		if json.Unmarshal(attachment[0], &contentType) != nil || !strings.Contains(contentType, "/") {
			return nil, errors.New("invalid Google Voice attachment content type")
		}
		meta := VoiceAttachment{ID: id, ContentType: contentType}
		if len(attachment) > 3 {
			var parts [][]int
			if json.Unmarshal(attachment[3], &parts) == nil {
				for _, part := range parts {
					if len(part) >= 3 && part[0] == 1 {
						meta.Width, meta.Height = part[1], part[2]
						break
					}
				}
			}
		}
		out = append(out, meta)
	}
	return out, nil
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
	var threadID string
	if json.Unmarshal(thread[0], &threadID) != nil {
		return nil, errors.New("invalid Google Voice thread identity")
	}
	group := strings.HasPrefix(threadID, "g.")
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
		if group {
			name, sender, body, participants, attachments, ok := decodeGroupMetadata(fields)
			if !ok {
				return nil, errors.New("invalid Google Voice group metadata")
			}
			m.GroupID, m.GroupName, m.GroupSender, m.Participants = threadID, name, sender, participants
			m.Attachments = attachments
			if m.Text == "" {
				m.Text = body
			}
			// Google Voice emits group system records whose text is the
			// "sender - group name" label rather than a message body.
			if !m.Outgoing && sender != "" && name != "" && m.Text == sender+" - "+name {
				continue
			}
		}
		m.Timestamp = time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano)
		if len(fields) > 17 && string(fields[17]) != "null" {
			if e := json.Unmarshal(fields[17], &m.ClientID); e != nil {
				return nil, errors.New("invalid Google Voice client correlation ID")
			}
		}
		if !group && string(fields[14]) != "null" {
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
			m.Attachments, _ = parseVoiceAttachments(media[2])
		}
		result = append(result, m)
	}
	return result, nil
}
