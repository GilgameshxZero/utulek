package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func printTable(out io.Writer, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}
	var lines [][]string
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		cells := make([][]string, len(row))
		height := 1
		for i, cell := range row {
			cells[i] = strings.Split(cell, "\n")
			if len(cells[i]) > height {
				height = len(cells[i])
			}
		}
		for n := 0; n < height; n++ {
			line := make([]string, len(row))
			for i, cell := range cells {
				if n < len(cell) {
					line[i] = cell[n]
				}
				if width := columnWidth(line[i]); width > widths[i] {
					widths[i] = width
				}
			}
			lines = append(lines, line)
		}
	}
	for _, line := range lines {
		for i, cell := range line {
			if i > 0 {
				if _, e := io.WriteString(out, " "); e != nil {
					return e
				}
			}
			if _, e := io.WriteString(out, cell+strings.Repeat(" ", widths[i]+3-columnWidth(cell))); e != nil {
				return e
			}
		}
		if _, e := io.WriteString(out, "\n"); e != nil {
			return e
		}
	}
	return nil
}
func displayName(c *Card, cfg Config, parsable bool) string {
	name := c.name(cfg.Display)
	if cfg.Nicknames && !parsable {
		view := columnValues{card: c}
		if nicknames, ok := view.property("nicknames").([]any); ok && len(nicknames) > 0 {
			name += " (Nickname: " + pythonValue(nicknames[0], false) + ")"
		}
	}
	return name
}
func preferred(view *columnValues, field string, prefs []string) string {
	values, _ := view.property(field).(map[string]any)
	if len(values) == 0 {
		return ""
	}
	var keys []string
	for _, pref := range prefs {
		if pref == "" {
			continue
		}
		for key := range values {
			if strings.Contains(strings.ToLower(key), strings.ToLower(pref)) {
				keys = append(keys, key)
			}
		}
		if len(keys) > 0 {
			break
		}
	}
	if len(keys) == 0 {
		for key := range values {
			if strings.Contains(strings.ToLower(key), "pref") {
				keys = append(keys, key)
			}
		}
	}
	if len(keys) == 0 {
		for key := range values {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
	label := keys[0]
	entries := values[label].([]any)
	if len(entries) == 0 {
		return ""
	}
	text := fmt.Sprint(entries[0])
	if label == "" {
		return text
	}
	return label + ": " + text
}
func shortUIDs(cards []*Card, scope map[string]bool) map[string]string {
	ids := map[string]bool{}
	for id := range scope {
		ids[id] = true
	}
	for _, c := range cards {
		if id := c.first("UID"); id != "" {
			ids[id] = true
		}
	}
	keys := make([]string, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(ids))
	prefix := func(a, b string) int {
		ar, br := []rune(a), []rune(b)
		n := 0
		for n < len(ar) && n < len(br) && ar[n] == br[n] {
			n++
		}
		return n
	}
	for i, id := range keys {
		n := 1
		if i > 0 {
			n = prefix(id, keys[i-1]) + 1
		}
		if i+1 < len(keys) {
			if next := prefix(id, keys[i+1]) + 1; next > n {
				n = next
			}
		}
		runes := []rune(id)
		if n > len(runes) {
			n = len(runes)
		}
		result[id] = string(runes[:n])
	}
	return result
}
func (r Runner) list(cards []*Card, cfg Config, o Options) error {
	if o.Command == "list" {
		return r.contactTable(cards, cfg, o)
	}
	if o.Command == "filename" {
		for _, c := range cards {
			fmt.Fprintln(r.Out, c.Path)
		}
		return nil
	}
	if o.Command == "birthdays" {
		return r.birthdays(cards, cfg, o)
	}
	field := map[string]string{"phone": "phone_numbers", "email": "emails", "postaddress": "post_addresses"}[o.Command]
	if field == "" {
		return fmt.Errorf("unknown listing command")
	}
	var rows [][]string
	query := compileSearch(o.Terms)
	for _, c := range cards {
		view := columnValues{card: c}
		values, _ := view.property(field).(map[string]any)
		var contactRows, matchedRows [][]string
		keys := sortedKeys(values)
		sort.SliceStable(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
		for _, label := range keys {
			for _, value := range values[label].([]any) {
				text := fmt.Sprint(value)
				if field == "post_addresses" {
					if o.Parsable {
						text = publicPythonValue(value)
					} else {
						text = searchAddress(value.(map[string]any))
					}
				}
				name := displayName(c, cfg, o.Parsable || o.Mutt)
				var row []string
				if o.Parsable || o.Mutt {
					row = []string{safe(text), safe(name), safe(label)}
				} else {
					row = []string{name, label, text}
				}
				contactRows = append(contactRows, row)
				if query.matchesLine(strings.Join(row, "\t")) {
					matchedRows = append(matchedRows, row)
				}
			}
		}
		if len(matchedRows) > 0 {
			rows = append(rows, matchedRows...)
		} else {
			rows = append(rows, contactRows...)
		}
	}
	if len(rows) == 0 {
		if o.Mutt {
			return nil
		}
		if !o.Parsable {
			fmt.Fprintf(r.Out, "Found no %s\n", map[string]string{"phone": "phone numbers", "email": "email addresses", "postaddress": "post addresses"}[o.Command])
		}
		return errNoResults
	}
	if o.Parsable || o.Mutt {
		for _, row := range rows {
			fmt.Fprintln(r.Out, strings.Join(row, "\t"))
		}
		return nil
	}
	heading := map[string]string{"phone": "Phone", "email": "E-Mail", "postaddress": "Post address"}[o.Command]
	return printTable(r.Out, append([][]string{{"Name", "Type", heading}}, rows...))
}
func (q searchQuery) matchesLine(line string) bool {
	if len(q) == 0 {
		return true
	}
	lower := strings.ToLower(line)
	for _, t := range q {
		if t.field == "phone_numbers" {
			if !strings.Contains(lower, t.value) && !matchPhoneDigits(phoneDigits(lower), t.digits) {
				return false
			}
		} else if !strings.Contains(lower, t.value) {
			return false
		}
	}
	return true
}
func (r Runner) contactTable(cards []*Card, cfg Config, o Options) error {
	if len(cards) == 0 {
		if !o.Parsable {
			fmt.Fprintln(r.Out, "Found no contacts")
		}
		return errNoResults
	}
	var books []string
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, c := range cards {
		if !seen[c.Book] {
			seen[c.Book] = true
			books = append(books, c.Book)
		}
		kinds[contactKind(c)] = true
	}
	fields := []string{"index", "name", "phone", "email"}
	if cfg.Kinds || len(kinds) > 1 || !kinds["individual"] {
		fields = append(fields, "kind")
	}
	if len(books) > 1 {
		fields = append(fields, "address_book")
	}
	if cfg.UIDs {
		fields = append(fields, "uid")
	}
	if o.Parsable {
		fields = []string{"uid", "name", "address_book"}
	}
	if len(o.ListFields) > 0 {
		fields = o.ListFields
	}
	var rows [][]string
	short := map[string]string{}
	if !o.Parsable {
		plural := ""
		if len(books) > 1 {
			plural = "s"
		}
		fmt.Fprintf(r.Out, "Address book%s: %s\n", plural, strings.Join(books, ", "))
		header := make([]string, len(fields))
		for i, field := range fields {
			header[i] = strings.Title(strings.ReplaceAll(field, "_", " "))
		}
		rows = append(rows, header)
		if contains(fields, "uid") {
			short = shortUIDs(cards, cfg.UIDScope)
		}
	}
	for i, c := range cards {
		view := columnValues{card: c}
		row := make([]string, len(fields))
		for j, field := range fields {
			switch field {
			case "index":
				row[j] = strconv.Itoa(i + 1)
			case "name":
				row[j] = displayName(c, cfg, o.Parsable)
			case "phone":
				row[j] = preferred(&view, "phone_numbers", cfg.PreferredPhone)
			case "email":
				row[j] = preferred(&view, "emails", cfg.PreferredEmail)
			case "impp":
				row[j] = preferred(&view, "impp", nil)
			case "uid":
				row[j] = c.first("UID")
				if !o.Parsable {
					row[j] = short[row[j]]
				}
			case "addressbook", "address_book":
				row[j] = c.Book
			default:
				row[j] = view.nested(field)
			}
			if o.Parsable {
				row[j] = safe(row[j])
			}
		}
		if o.Parsable {
			fmt.Fprintln(r.Out, strings.Join(row, "\t"))
		} else {
			rows = append(rows, row)
		}
	}
	if o.Parsable {
		return nil
	}
	return printTable(r.Out, rows)
}
func (r Runner) birthdays(cards []*Card, cfg Config, o Options) error {
	type entry struct {
		card       *Card
		month, day int
		text       string
	}
	var entries []entry
	for _, c := range cards {
		date, text, present, _ := parsedSearchDate(c, "birthday")
		if !present {
			continue
		}
		entry := entry{card: c}
		if text != nil {
			entry.text = *text
		} else {
			entry.month = int(date.Month())
			entry.day = date.Day()
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.month != b.month {
			return a.month < b.month
		}
		if a.day != b.day {
			return a.day < b.day
		}
		return a.text < b.text
	})
	if len(entries) == 0 {
		if !o.Parsable {
			fmt.Fprintln(r.Out, "Found no birthdays")
		}
		return errNoResults
	}
	rows := [][]string{{"Name", "Birthday"}}
	for _, entry := range entries {
		c := entry.card
		name := displayName(c, cfg, o.Parsable)
		if o.Parsable {
			date, text, _, _ := parsedSearchDate(c, "birthday")
			value := date.Format("2006.01.02")
			if text != nil {
				value = *text
			}
			fmt.Fprintf(r.Out, "%s\t%s\n", safe(value), safe(name))
		} else {
			rows = append(rows, []string{name, formatContactDate(c, "birthday", cfg)})
		}
	}
	if o.Parsable {
		return nil
	}
	return printTable(r.Out, rows)
}

// Match terminal cells for CJK text and combining marks, rather than counting
// UTF-8 bytes or assuming every Chinese character occupies one column.
func columnWidth(s string) int {
	width := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || (r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff01 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) || (r >= 0x20000 && r <= 0x3fffd)) {
			width += 2
		} else {
			width++
		}
	}
	return width
}
