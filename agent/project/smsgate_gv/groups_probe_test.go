package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var phoneValuePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// Read-only discovery of non-one-to-one thread structures. Stores only counts,
// field shapes and phone-value positions; never message text or numbers.
func TestLiveGroupDiscovery(t *testing.T) {
	if os.Getenv("GV_LIVE_GROUPS") != "1" {
		t.Skip("explicit opt-in; read only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	c.SyncNumbers = nil
	c.MaxThreads = 1000
	b, e := startBrowser(ctx, home, c)
	if e != nil {
		t.Fatal(e)
	}
	defer b.close()
	if _, e = b.readNetworkHistory(); e != nil {
		t.Fatal(e)
	}
	body, e := b.voiceEndpoint(coldSyncPath, coldSyncRequest(c))
	if e != nil {
		t.Fatal(e)
	}
	var envelope []json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || len(envelope) < 2 {
		t.Fatal("invalid cold sync envelope")
	}
	var views [][]json.RawMessage
	if e = json.Unmarshal(envelope[0], &views); e != nil {
		t.Fatal(e)
	}
	prefixes := map[string]int{}
	type group struct {
		IDLength         int         `json:"id_length"`
		Prefix           string      `json:"prefix"`
		ThreadLength     int         `json:"thread_length"`
		RecordCount      int         `json:"record_count"`
		ThreadFields     []string    `json:"thread_fields"`
		RecordFields     []string    `json:"record_fields"`
		PhonePositions   []string    `json:"phone_positions"`
		FirstRecordShape any         `json:"first_record_shape"`
		LabelMatches     int         `json:"label_matches"`
		TextLengths      []int       `json:"text_lengths"`
		Kinds            map[int]int `json:"kinds"`
		Outgoing         int         `json:"outgoing"`
		Media0Nonempty   int         `json:"media0_nonempty"`
		Media1Nonempty   int         `json:"media1_nonempty"`
		Media1Lengths    []int       `json:"media1_lengths"`
	}
	groups := []group{}
	for _, view := range views {
		var kind int
		if len(view) > 0 {
			_ = json.Unmarshal(view[0], &kind)
		}
		if kind != 2 || len(view) < 3 {
			continue
		}
		var threads [][]json.RawMessage
		if json.Unmarshal(view[2], &threads) != nil {
			continue
		}
		for _, thread := range threads {
			var id string
			if len(thread) == 0 || json.Unmarshal(thread[0], &id) != nil || id == "" {
				continue
			}
			prefix := "unknown"
			if cut := strings.Index(id, "."); cut >= 0 {
				prefix = id[:cut+1]
			}
			prefixes[prefix]++
			if prefix == "t." {
				continue
			}
			g := group{IDLength: len(id), Prefix: prefix, ThreadLength: len(thread), ThreadFields: fieldTypes(thread)}
			var records [][]json.RawMessage
			if len(thread) > 2 && json.Unmarshal(thread[2], &records) == nil {
				g.RecordCount = len(records)
				if len(records) > 0 {
					g.FirstRecordShape = payloadShape(records[0], 0)
					g.RecordFields = fieldTypes(records[0])
					g.PhonePositions = phoneFieldPositions(records[0])
				}
				labels := 0
				textLens := []int{}
				kinds := map[int]int{}
				outgoing := 0
				media0Nonempty, media1Nonempty := 0, 0
				media1Lens := []int{}
				for _, record := range records {
					var kind int
					_ = json.Unmarshal(record[4], &kind)
					kinds[kind]++
					if kind == 11 {
						outgoing++
					}
					var text string
					_ = json.Unmarshal(record[9], &text)
					textLens = append(textLens, len(text))
					var media []json.RawMessage
					if len(record) > 14 && json.Unmarshal(record[14], &media) == nil && len(media) > 4 {
						var subject, sender, body string
						_ = json.Unmarshal(media[0], &subject)
						_ = json.Unmarshal(media[1], &body)
						_ = json.Unmarshal(media[4], &sender)
						if subject != "" {
							media0Nonempty++
						}
						if body != "" {
							media1Nonempty++
						}
						media1Lens = append(media1Lens, len(body))
						if subject != "" && sender != "" && text == sender+" - "+subject {
							labels++
						}
					}
				}
				g.LabelMatches = labels
				g.TextLengths = textLens
				g.Kinds = kinds
				g.Outgoing = outgoing
				g.Media0Nonempty = media0Nonempty
				g.Media1Nonempty = media1Nonempty
				g.Media1Lengths = media1Lens
			}
			groups = append(groups, g)
		}
	}
	keys := []string{}
	for key := range prefixes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if e = writeJSON(filepath.Join(".obj", "groups_probe.json"), map[string]any{"prefixes": prefixes, "groups": groups}); e != nil {
		t.Fatal(e)
	}
	t.Logf("prefixes=%v groups=%d", keys, len(groups))
}

func fieldTypes(fields []json.RawMessage) []string {
	out := []string{}
	for i, field := range fields {
		if i >= 40 {
			break
		}
		var value any
		_ = json.Unmarshal(field, &value)
		switch x := value.(type) {
		case string:
			out = append(out, fmt.Sprintf("string:%d", len(x)))
		case float64:
			out = append(out, "number")
		case nil:
			out = append(out, "null")
		case []any:
			out = append(out, fmt.Sprintf("array:%d", len(x)))
		case map[string]any:
			out = append(out, fmt.Sprintf("object:%d", len(x)))
		default:
			out = append(out, fmt.Sprintf("%T", x))
		}
	}
	return out
}

func phoneFieldPositions(fields []json.RawMessage) []string {
	out := []string{}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch x := value.(type) {
		case string:
			if phoneValuePattern.MatchString(x) {
				out = append(out, path)
			}
		case []any:
			for i, child := range x {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		case map[string]any:
			for key, child := range x {
				walk(child, path+"."+key)
			}
		}
	}
	for i, field := range fields {
		var value any
		_ = json.Unmarshal(field, &value)
		walk(value, fmt.Sprintf("[%d]", i))
	}
	return out
}
