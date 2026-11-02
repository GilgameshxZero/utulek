package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// vCard property values escape separators, backslashes and newlines. Decode in
// one pass so an escaped backslash followed by n stays a literal backslash+n.
func vcardText(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' || s[i] == 'N' {
				out.WriteByte('\n')
			} else {
				out.WriteByte(s[i])
			}
		} else {
			out.WriteByte(s[i])
		}
	}
	return out.String()
}

func vcardParts(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// Read individual vCards or concatenated cards. Unfold before parsing, keeping
// names, aliases, notes and all phone fields searchable like the khard picker.
func parseVCards(ctx context.Context, data string, query string) (map[string]Contact, error) {
	return parseVCardsIndex(ctx, data, query, nil)
}
func parseVCardsIndex(ctx context.Context, data string, query string, index *[]contactCard) (map[string]Contact, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	var lines []string
	for scanner.Scan() {
		s := strings.TrimSuffix(scanner.Text(), "\r")
		if len(s) > 0 && (s[0] == ' ' || s[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += s[1:]
		} else {
			lines = append(lines, s)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	result := map[string]Contact{}
	var name, fallback string
	var searchable []string
	type phone struct{ number, kind string }
	var phones []phone
	active := false
	terms := strings.Fields(strings.ToLower(query))
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.EqualFold(line, "BEGIN:VCARD") {
			active = true
			name = ""
			fallback = ""
			searchable = nil
			phones = nil
			continue
		}
		if strings.EqualFold(line, "END:VCARD") {
			if !active {
				continue
			}
			active = false
			hay := strings.ToLower(strings.Join(searchable, " "))
			match := true
			for _, term := range terms {
				if !strings.Contains(hay, term) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			if name == "" {
				name = fallback
			}
			if name == "" {
				name = "Unnamed contact"
			}
			card := contactCard{Search: hay, Contacts: map[string]Contact{}}
			for _, p := range phones {
				contact := Contact{strings.TrimSpace(clean(name)), p.kind}
				result[p.number] = contact
				card.Contacts[p.number] = contact
			}
			if index != nil {
				*index = append(*index, card)
			}
			continue
		}
		if !active {
			continue
		}
		// Property parameters can contain quoted colons (e.g. labels).
		colon := -1
		quoted := false
		for i, c := range line {
			if c == '"' {
				quoted = !quoted
			}
			if c == ':' && !quoted {
				colon = i
				break
			}
		}
		if colon < 0 {
			continue
		}
		head, value := line[:colon], line[colon+1:]
		fields := strings.Split(head, ";")
		key := strings.ToUpper(fields[0])
		if dot := strings.LastIndex(key, "."); dot >= 0 {
			key = key[dot+1:]
		}
		decoded := vcardText(value)
		searchable = append(searchable, decoded)
		switch key {
		case "FN":
			name = decoded
		case "N":
			parts := vcardParts(value, ';')
			if len(parts) >= 2 {
				fallback = vcardText(parts[1]) + " " + vcardText(parts[0])
			}
		case "TEL":
			number := strings.TrimSpace(decoded)
			if strings.HasPrefix(strings.ToLower(number), "tel:") {
				number = number[4:]
			}
			if strings.HasPrefix(number, "+") || strings.HasPrefix(number, "00") {
				number = phoneKey(number)
			} else {
				number = strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", ".", "").Replace(number)
			}
			if number == "" {
				continue
			}
			kind := ""
			for _, f := range fields[1:] {
				if strings.HasPrefix(strings.ToUpper(f), "TYPE=") {
					kind = strings.Trim(f[5:], "\"")
				}
			}
			phones = append(phones, phone{number, kind})
		}
	}
	return result, nil
}

type contactCard struct {
	Search   string
	Contacts map[string]Contact
}
type contactFile struct {
	Modified time.Time
	Size     int64
	Cards    []contactCard
}
type contactReader struct {
	mu        sync.Mutex
	directory string
	checked   time.Time
	files     map[string]contactFile
	order     []string
}

func readContacts(ctx context.Context, directory, query string) (map[string]Contact, error) {
	return (&contactReader{}).read(ctx, directory, query)
}
func (r *contactReader) read(ctx context.Context, directory, query string) (map[string]Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := map[string]Contact{}
	if directory == "" {
		return result, nil
	}
	directory = expand(directory)
	if r.directory != directory {
		r.directory = directory
		r.files = map[string]contactFile{}
		r.checked = time.Time{}
	}
	if time.Since(r.checked) >= time.Second {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, fmt.Errorf("cannot read vCard directory: %w", err)
		}
		next := map[string]contactFile{}
		order := []string{}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".vcf") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			old, ok := r.files[entry.Name()]
			if !ok || !old.Modified.Equal(info.ModTime()) || old.Size != info.Size() {
				data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
				if err != nil {
					return nil, fmt.Errorf("cannot read vCard: %w", err)
				}
				old = contactFile{Modified: info.ModTime(), Size: info.Size()}
				if _, err = parseVCardsIndex(ctx, string(data), "", &old.Cards); err != nil {
					return nil, err
				}
			}
			next[entry.Name()] = old
			order = append(order, entry.Name())
		}
		sort.Strings(order)
		r.files, r.order, r.checked = next, order, time.Now()
	}
	terms := strings.Fields(strings.ToLower(query))
	for _, name := range r.order {
		for _, card := range r.files[name].Cards {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			matches := true
			for _, term := range terms {
				if !strings.Contains(card.Search, term) {
					matches = false
					break
				}
			}
			if matches {
				for number, contact := range card.Contacts {
					result[number] = contact
				}
			}
		}
	}
	return result, nil
}
