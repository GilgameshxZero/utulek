package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"
)

type Recipient struct {
	Number string `json:"phoneNumber"`
	State  string `json:"state"`
}
type Text struct {
	Text string `json:"text"`
}
type MediaText struct {
	Text    string `json:"text"`
	Subject string `json:"subject,omitempty"`
}
type Message struct {
	Account     string      `json:"googleVoiceNumber,omitempty"`
	ID          string      `json:"id"`
	Created     string      `json:"createdAt"`
	State       string      `json:"state"`
	Recipients  []Recipient `json:"recipients,omitempty"`
	Body        *Text       `json:"textMessage,omitempty"`
	Sender      string      `json:"sender,omitempty"`
	Preview     string      `json:"contentPreview,omitempty"`
	Type        string      `json:"type"`
	Source      string      `json:"googleVoiceId,omitempty"`
	Error       string      `json:"error,omitempty"`
	MMS         *MediaText  `json:"mmsMessage,omitempty"`
	SendToken   string      `json:"googleVoiceClientId,omitempty"`
	Submitted   bool        `json:"submitted,omitempty"`
	AttemptedAt string      `json:"attemptedAt,omitempty"`
}
type Store struct {
	mu          sync.Mutex
	path        string
	rows        map[string]Message
	subscribers map[chan []Message]struct{}
}

func openStore(home string) (*Store, error) {
	s := &Store{path: filepath.Join(home, "messages.json"), rows: map[string]Message{}}
	b, e := os.ReadFile(s.path)
	if e == nil {
		e = json.Unmarshal(b, &s.rows)
	} else if os.IsNotExist(e) {
		e = nil
	}
	if e != nil {
		return nil, e
	}
	// Nothing pending is automatically retried following a restart.
	for id, m := range s.rows {
		if m.State == "Pending" {
			setState(&m, "Failed", "bridge stopped before dispatch; no send attempted")
			s.rows[id] = m
		} else if m.State == "Processed" {
			setState(&m, "Uncertain", "bridge stopped before send outcome was confirmed")
			s.rows[id] = m
		}
	}
	return s, writeJSON(s.path, s.rows)
}
func setState(m *Message, state, reason string) {
	m.State = state
	m.Error = reason
	for i := range m.Recipients {
		m.Recipients[i].State = state
	}
}
func (s *Store) get(id string) (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.rows[id]
	return cloneMessage(m), ok
}
func (s *Store) put(m Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.rows[m.ID]
	s.rows[m.ID] = cloneMessage(m)
	if e := writeJSON(s.path, s.rows); e != nil {
		if ok {
			s.rows[m.ID] = old
		} else {
			delete(s.rows, m.ID)
		}
		return e
	}
	s.publishLocked([]Message{m})
	return nil
}
func (s *Store) reserve(m Message) (Message, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.rows[m.ID]; ok {
		if old.Account != m.Account || old.Body == nil || old.Body.Text != m.Body.Text || len(old.Recipients) != 1 || old.Recipients[0].Number != m.Recipients[0].Number {
			return cloneMessage(old), false, errors.New("message ID already exists with different content")
		}
		return cloneMessage(old), false, nil
	}
	s.rows[m.ID] = cloneMessage(m)
	if e := writeJSON(s.path, s.rows); e != nil {
		delete(s.rows, m.ID)
		return m, false, e
	}
	s.publishLocked([]Message{m})
	return m, true, nil
}
func (s *Store) list(incoming bool) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Message{}
	for _, m := range s.rows {
		if (m.Sender != "") == incoming {
			out = append(out, cloneMessage(m))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, out[i].Created)
		right, _ := time.Parse(time.RFC3339Nano, out[j].Created)
		if left.Equal(right) {
			return out[i].ID < out[j].ID
		}
		return left.After(right)
	})
	return out
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// One durable commit per refresh; source IDs retain locally assigned send IDs.
func (s *Store) mergeImports(rows []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]Message, len(s.rows)+len(rows))
	bySource := map[string]string{}
	byToken := map[string][]string{}
	for id, m := range s.rows {
		next[id] = m
		if m.Submitted && m.SendToken != "" {
			key := m.Account + "\x00" + m.SendToken
			byToken[key] = append(byToken[key], id)
		}
		if m.Source != "" {
			bySource[m.Account+"\x00"+m.Source] = id
		}
	}
	changed := false
	changedRows := []Message{}
	for _, m := range rows {
		if id, ok := bySource[m.Account+"\x00"+m.Source]; ok {
			m.ID = id
		}
		if ids := byToken[m.Account+"\x00"+m.SendToken]; m.SendToken != "" && len(ids) == 1 {
			candidate := next[ids[0]]
			if candidate.Body != nil && m.Body != nil && candidate.Body.Text == m.Body.Text && len(candidate.Recipients) == 1 && len(m.Recipients) == 1 && candidate.Recipients[0].Number == m.Recipients[0].Number && (candidate.Source == "" || candidate.Source == m.Source) {
				if m.ID != candidate.ID {
					if _, exists := next[m.ID]; exists {
						delete(next, m.ID)
						changed = true
					}
				}
				m.ID = candidate.ID
			}
		}
		if old, ok := next[m.ID]; ok && old.Submitted {
			m.Submitted = true
			m.AttemptedAt = old.AttemptedAt
		}
		if old, ok := next[m.ID]; !ok || !reflect.DeepEqual(old, m) {
			next[m.ID] = cloneMessage(m)
			changedRows = append(changedRows, cloneMessage(m))
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if e := writeJSON(s.path, next); e != nil {
		return e
	}
	s.rows = next
	s.publishLocked(changedRows)
	return nil
}

// Published journal records are immutable snapshots; callers may update their
// own send state without racing API readers or mutating a committed snapshot.
func cloneMessage(m Message) Message {
	m.Recipients = append([]Recipient(nil), m.Recipients...)
	if m.Body != nil {
		body := *m.Body
		m.Body = &body
	}
	if m.MMS != nil {
		media := *m.MMS
		m.MMS = &media
	}
	return m
}

func (s *Store) subscribe() (<-chan []Message, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscribers == nil {
		s.subscribers = map[chan []Message]struct{}{}
	}
	ch := make(chan []Message, 32)
	s.subscribers[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}
func (s *Store) publishLocked(rows []Message) {
	if len(rows) == 0 {
		return
	}
	snapshot := make([]Message, len(rows))
	for i, m := range rows {
		snapshot[i] = cloneMessage(m)
	}
	for ch := range s.subscribers {
		select {
		case ch <- snapshot:
		default:
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}
