package main

import (
	"bytes"
	"fmt"
	"io"
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
	if !strings.ContainsRune(s, '\\') {
		return s
	}
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

var vcardEscaper = strings.NewReplacer("\\", `\\`, "\r\n", `\n`, "\r", `\n`, "\n", `\n`, ";", `\;`, ",", `\,`)

func escape(s string) string { return vcardEscaper.Replace(s) }

func parseProperty(line string) (Property, error) {
	colon := -1
	quoted := false
	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			quoted = !quoted
		}
		if line[i] == ':' && !quoted {
			colon = i
			break
		}
	}
	if colon < 0 {
		return Property{}, fmt.Errorf("vCard property has no colon")
	}
	head := line[:colon]
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

// Split structured names before unescaping punctuation inside components.
func (c *Card) nameParts() (names [5]string) {
	for _, p := range c.Props {
		if p.Key != "N" {
			continue
		}
		for i, component := range splitEscaped(p.Value, ';') {
			if i >= len(names) {
				break
			}
			parts := splitEscaped(component, ',')
			for j := range parts {
				parts[j] = unescape(parts[j])
			}
			names[i] = strings.Join(parts, " ")
		}
		break
	}
	return names
}

func (c *Card) name(order string) string {
	if order == "formatted_name" {
		if name := c.first("FN"); name != "" {
			return name
		}
		return "Unnamed contact"
	}
	n := c.nameParts()
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
		s = joinName(n[1], n[2])
		if n[0] != "" && s != "" {
			s = strings.TrimSpace(n[0]) + ", " + s
		} else if n[0] != "" {
			s = strings.TrimSpace(n[0])
		}
	case "first_name":
		s = joinName(n[1], n[2], n[0])
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
	return loadMatchingCards(c, names, nil)
}

func loadMatchingCards(c Config, names, terms []string) ([]*Card, error) {
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
	query := compileSearch(terms)
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
		c.debugf("loading book=%s source_search=%t", b.Name, c.SourceSearch)
		read, filtered, parsed, matched := 0, 0, 0, 0
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
			read++
			if c.SourceSearch && !query.matchesSource(data) {
				filtered++
				continue
			}
			parsed++
			card, e := parseCard(data)
			if e != nil {
				if c.Skip {
					fmt.Fprintf(c.diagnosticWriter(), "Skipping %s: %v\n", path, e)
					continue
				}
				return nil, fmt.Errorf("%s: %w (use --skip-unparsable to skip)", path, e)
			}
			card.Path = path
			card.Book = b.Name
			ok, e := query.match(card, c)
			if e != nil {
				return nil, fmt.Errorf("%s: %w", path, e)
			}
			if ok {
				cards = append(cards, card)
				matched++
			}
		}
		c.debugf("book=%s read=%d prefiltered=%d parsed=%d matched=%d", b.Name, read, filtered, parsed, matched)
	}
	sortCards(cards, c)
	return cards, nil
}

// Compute decoded, case-folded names once per card rather than once per
// comparison. Keep the path tie-break and stable ordering used by the loader.
func sortCards(cards []*Card, cfg Config) {
	type entry struct {
		card *Card
		name string
	}
	entries := make([]entry, len(cards))
	for i, c := range cards {
		entries[i] = entry{c, strings.ToLower(c.name(cfg.Sort))}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if cfg.Group && a.card.Book != b.card.Book {
			return a.card.Book < b.card.Book
		}
		if a.name == b.name {
			return a.card.Path < b.card.Path
		}
		if cfg.Reverse {
			return a.name > b.name
		}
		return a.name < b.name
	})
	for i := range entries {
		cards[i] = entries[i].card
	}
}

func (c Config) diagnosticWriter() io.Writer {
	if c.Diagnostics != nil {
		return c.Diagnostics
	}
	return os.Stderr
}
func (c Config) debugf(format string, args ...any) {
	if c.Debug {
		fmt.Fprintf(c.diagnosticWriter(), "DEBUG khard: "+format+"\n", args...)
	}
}
