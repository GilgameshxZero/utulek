package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type Property struct{ Raw, Head, Key, Group, Value string }
type Card struct {
	Path, Book string
	Original   []byte
	Props      []Property
}

func validToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func splitEscaped(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func splitQuoted(s string, sep byte) []string {
	var out []string
	start := 0
	quote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			quote = !quote
		}
		if s[i] == sep && !quote {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' || s[i] == 'N' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
func escape(s string) string {
	return strings.NewReplacer("\\", `\\`, "\r\n", `\n`, "\r", `\n`, "\n", `\n`, ";", `\;`, ",", `\,`).Replace(s)
}

func parseProperty(line string) (Property, error) {
	parts := splitQuoted(line, ':')
	if len(parts) < 2 {
		return Property{}, fmt.Errorf("vCard property has no colon")
	}
	head := parts[0]
	key := strings.SplitN(head, ";", 2)[0]
	group := ""
	if i := strings.LastIndex(key, "."); i >= 0 {
		group = key[:i]
		key = key[i+1:]
	}
	if !validToken(key) || (group != "" && !validToken(group)) {
		return Property{}, fmt.Errorf("invalid vCard property name")
	}
	return Property{Head: head, Key: strings.ToUpper(key), Group: group, Value: line[len(head)+1:]}, nil
}

func parseCard(data []byte) (*Card, error) {
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("vCard must be UTF-8 without NUL bytes")
	}
	c := &Card{Original: append([]byte(nil), data...)}
	var lines, raw []string
	for _, physical := range strings.SplitAfter(string(data), "\n") {
		if physical == "" {
			continue
		}
		s := strings.TrimSuffix(strings.TrimSuffix(physical, "\n"), "\r")
		if len(s) > 0 && (s[0] == ' ' || s[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += s[1:]
			raw[len(raw)-1] += physical
		} else {
			lines = append(lines, s)
			raw = append(raw, physical)
		}
	}
	active, ended := false, false
	for i, line := range lines {
		if i == 0 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" {
			if len(c.Props) > 0 {
				c.Props[len(c.Props)-1].Raw += raw[i]
			}
			continue
		}
		p, e := parseProperty(line)
		if e != nil {
			return nil, e
		}
		p.Raw = raw[i]
		if p.Key == "BEGIN" {
			if active || ended || !strings.EqualFold(p.Value, "VCARD") {
				return nil, fmt.Errorf("expected one vCard per file")
			}
			active = true
		} else if p.Key == "END" {
			if !active || !strings.EqualFold(p.Value, "VCARD") {
				return nil, fmt.Errorf("invalid vCard end")
			}
			active = false
			ended = true
		} else if !active {
			return nil, fmt.Errorf("property outside vCard")
		}
		c.Props = append(c.Props, p)
	}
	if active || !ended {
		return nil, fmt.Errorf("incomplete vCard")
	}
	if len(c.properties("VERSION")) != 1 {
		return nil, fmt.Errorf("vCard needs exactly one VERSION")
	}
	return c, nil
}

func (c *Card) properties(key string) []Property {
	var out []Property
	for _, p := range c.Props {
		if p.Key == key {
			out = append(out, p)
		}
	}
	return out
}
func (c *Card) first(key string) string {
	for _, p := range c.Props {
		if p.Key == key {
			return unescape(p.Value)
		}
	}
	return ""
}
func (c *Card) label(p Property) string {
	var labels []string
	if p.Group != "" {
		for _, q := range c.Props {
			if q.Group == p.Group && q.Key == "X-ABLABEL" {
				labels = append(labels, unescape(q.Value))
			}
		}
	}
	for _, field := range splitQuoted(p.Head, ';')[1:] {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		if strings.EqualFold(k, "TYPE") {
			labels = append(labels, strings.Split(v, ",")...)
		} else if strings.EqualFold(k, "PREF") {
			labels = append(labels, "pref="+v)
		}
	}
	return strings.Join(labels, ", ")
}

func foldLine(line string) string {
	var b strings.Builder
	limit := 75
	for len(line) > limit {
		n := limit
		for n > 0 && !utf8.RuneStart(line[n]) {
			n--
		}
		b.WriteString(line[:n])
		b.WriteString("\r\n ")
		line = line[n:]
		limit = 74
	}
	b.WriteString(line)
	b.WriteString("\r\n")
	return b.String()
}

func newProp(head, value string) Property {
	p, _ := parseProperty(head + ":" + value)
	p.Raw = foldLine(head + ":" + value)
	return p
}

// Replace only properties for the changed field. Reuse the exact original
// property when possible, and retain parameter/group metadata when its value
// changes. Everything else stays byte-for-byte intact.
func (c *Card) replace(key string, values []Property) {
	old := c.properties(key)
	used := make([]bool, len(old))
	for i := range values {
		label := (&Card{}).label(values[i])
		best := -1
		for j, p := range old {
			if !used[j] && c.label(p) == label {
				if best < 0 {
					best = j
				}
				if p.Value == values[i].Value {
					best = j
					break
				}
			}
		}
		if best >= 0 {
			p := old[best]
			used[best] = true
			if p.Value == values[i].Value {
				values[i] = p
			} else {
				value := values[i].Value
				if key == "TEL" {
					if strings.HasPrefix(strings.ToLower(p.Value), "tel:") {
						if !strings.HasPrefix(strings.ToLower(value), "tel:") {
							value = "tel:" + value
						}
					} else if !strings.Contains(strings.ToLower(p.Head), "value=uri") {
						value = strings.TrimPrefix(value, "tel:")
					}
				}
				values[i] = newProp(p.Head, value)
			}
		}
	}
	var props []Property
	inserted := false
	for _, p := range c.Props {
		if p.Key == key {
			if !inserted {
				props = append(props, values...)
				inserted = true
			}
			continue
		}
		if p.Key == "END" && !inserted {
			props = append(props, values...)
			inserted = true
		}
		props = append(props, p)
	}
	c.Props = props
}

func (c *Card) bytes() []byte {
	var b strings.Builder
	for i, p := range c.Props {
		if i > 0 && b.Len() > 0 {
			s := b.String()
			if s[len(s)-1] != '\n' {
				b.WriteString("\r\n")
			}
		}
		b.WriteString(p.Raw)
	}
	return []byte(b.String())
}

func (c *Card) name(order string) string {
	n := strings.Split(c.first("N"), ";")
	// N components must be split before unescaping embedded semicolons.
	if p := c.properties("N"); len(p) > 0 {
		n = splitEscaped(p[0].Value, ';')
		for i := range n {
			n[i] = unescape(n[i])
		}
	}
	for len(n) < 5 {
		n = append(n, "")
	}
	var s string
	joinName := func(parts ...string) string {
		var nonempty []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				nonempty = append(nonempty, p)
			}
		}
		return strings.Join(nonempty, " ")
	}
	switch order {
	case "last_name":
		s = joinName(n[0], n[1], n[2], n[3], n[4])
	case "first_name":
		s = joinName(n[3], n[1], n[2], n[0], n[4])
	default:
		s = c.first("FN")
	}
	if s == "" {
		s = c.first("FN")
	}
	if s == "" {
		s = "Unnamed contact"
	}
	return s
}

func loadCards(c Config, names []string) ([]*Card, error) {
	wanted := map[string]bool{}
	for _, s := range names {
		wanted[s] = true
	}
	for _, b := range c.Books {
		delete(wanted, b.Name)
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("unknown address book %s", strings.Join(names, ","))
	}
	var cards []*Card
	for _, b := range c.Books {
		if len(names) > 0 {
			found := false
			for _, n := range names {
				if n == b.Name {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		entries, e := os.ReadDir(b.Path)
		if e != nil {
			return nil, fmt.Errorf("address book %s: %w", b.Name, e)
		}
		for _, ent := range entries {
			if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".vcf") || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			path := filepath.Join(b.Path, ent.Name())
			data, e := os.ReadFile(path)
			if e != nil {
				return nil, e
			}
			card, e := parseCard(data)
			if e != nil {
				if c.Skip {
					fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", path, e)
					continue
				}
				return nil, fmt.Errorf("%s: %w (use --skip-unparsable to skip)", path, e)
			}
			card.Path = path
			card.Book = b.Name
			cards = append(cards, card)
		}
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if c.Group && cards[i].Book != cards[j].Book {
			return cards[i].Book < cards[j].Book
		}
		a, b := strings.ToLower(cards[i].name(c.Sort)), strings.ToLower(cards[j].name(c.Sort))
		if a == b {
			return cards[i].Path < cards[j].Path
		}
		if c.Reverse {
			return a > b
		}
		return a < b
	})
	return cards, nil
}
