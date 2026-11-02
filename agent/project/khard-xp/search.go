package main

import (
	"fmt"
	"strings"
	"time"
)

type searchTerm struct{ field, value, digits string }
type searchQuery []searchTerm

// Compile once per load, not once per contact. Extra YAML aliases and
// case-insensitive field names remain documented preview extensions.
func compileSearch(terms []string) searchQuery {
	if len(terms) == 0 {
		return nil
	}
	q := make(searchQuery, 0, len(terms))
	for _, s := range terms {
		field, value, qualified := strings.Cut(s, ":")
		field = strings.ToLower(field)
		_, public := listProperties[field]
		_, alias := searchModelFields[field]
		if !qualified || (!public && !alias && field != "name") {
			field = ""
			value = s
		}
		if field == "kind" {
			canonical := ""
			for _, kind := range []string{"individual", "group", "org", "location", "application", "device"} {
				if strings.HasPrefix(kind, strings.ToLower(value)) {
					canonical = kind
					break
				}
			}
			if canonical == "" {
				field = ""
				value = s
			} else {
				value = canonical
			}
		}
		t := searchTerm{field: field, value: strings.ToLower(value)}
		if field == "phone_numbers" {
			t.digits = phoneDigits(value)
		}
		q = append(q, t)
	}
	return q
}

// Upstream's fast mode matches each query value against the raw file, then
// still checks the parsed contact. Folding/escaping can therefore hide matches.
func (q searchQuery) matchesSource(data []byte) bool {
	if len(q) == 0 {
		return true
	}
	source := strings.ToLower(strings.ReplaceAll(string(data), "\r\n", "\n"))
	for _, t := range q {
		if t.field == "phone_numbers" {
			if !strings.Contains(source, t.value) && !matchPhoneDigits(phoneDigits(source), t.digits) {
				return false
			}
		} else if !strings.Contains(source, t.value) {
			return false
		}
	}
	return true
}

func (q searchQuery) match(c *Card, cfg Config) (bool, error) {
	if len(q) == 0 {
		return true, nil
	}
	view := columnValues{card: c, search: true}
	document := ""
	documentReady := false
	var model Model
	for _, t := range q {
		var value any
		switch t.field {
		case "":
			if !documentReady {
				if quickVisibleMatch(c, cfg, t.value) {
					continue
				}
				if simpleSearchAtom(t.value) {
					return false, nil
				}
				document = strings.ToLower(searchDocument(c, cfg, &view))
				documentReady = true
			}
			if !strings.Contains(document, t.value) {
				return false, nil
			}
			continue
		case "phone_numbers":
			if !c.matchesPhoneTerm(t) {
				return false, nil
			}
			continue
		case "name":
			if strings.Contains(strings.ToLower(c.name("first_name")), t.value) || strings.Contains(strings.ToLower(c.name("last_name")), t.value) || strings.Contains(strings.ToLower(c.first("FN")), t.value) || matchSearchValue(view.property("nicknames"), t.value) {
				continue
			}
			return false, nil
		case "birthday", "anniversary":
			matched, e := matchDateQuery(c, t.field, t.value)
			if e != nil || !matched {
				return false, e
			}
			continue
		default:
			if _, public := listProperties[t.field]; public {
				value = view.property(t.field)
			} else {
				if model == nil {
					model = c.model(cfg)
				}
				value = model[searchModelFields[t.field]]
			}
		}
		if !matchSearchValue(value, t.value) {
			return false, nil
		}
	}
	return true, nil
}

func matchSearchValue(value any, term string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(strings.ToLower(v), term)
	case []any:
		for _, item := range v {
			if matchSearchValue(item, term) {
				return true
			}
		}
	case map[string]any:
		for k, item := range v {
			if strings.Contains(strings.ToLower(k), term) || matchSearchValue(item, term) {
				return true
			}
		}
	}
	return false
}
func (c *Card) matches(terms []string, cfg Config) bool {
	if len(terms) == 0 {
		return true
	}
	matched, _ := compileSearch(terms).match(c, cfg)
	return matched
}

func matchPhoneDigits(number, digits string) bool {
	switch {
	case strings.HasPrefix(digits, "+") && strings.HasPrefix(number, "0"):
		return strings.Contains(digits, number[1:])
	case strings.HasPrefix(digits, "0") && strings.HasPrefix(number, "+") && len(digits) >= 5:
		return strings.Contains(number, digits[1:])
	default:
		return digits != "" && strings.Contains(number, digits)
	}
}
func (c *Card) matchesPhoneTerm(t searchTerm) bool {
	for _, p := range c.Props {
		if p.Key == "TEL" {
			label := searchLabel(c, p, "TEL")
			if strings.Contains(strings.ToLower(label), t.value) || matchPhoneDigits(phoneDigits(unescape(p.Value)), t.digits) {
				return true
			}
		}
	}
	return false
}

// Dates are exact ISO-date comparisons, not substring matches. VALUE=text
// remains ordinary text; reduced dates use upstream's implicit year 1900.
func parsedSearchDate(c *Card, field string) (time.Time, *string, bool, bool) {
	keys := []string{"BDAY"}
	if field == "anniversary" {
		keys = []string{"ANNIVERSARY", "X-ANNIVERSARY"}
	}
	for _, key := range keys {
		for _, p := range c.Props {
			if p.Key == key {
				date, text, present, zoned := parseSearchDateProperty(p)
				if present {
					return date, text, present, zoned
				}
				break
			}
		}
	}
	return time.Time{}, nil, false, false
}
func parseSearchDateProperty(p Property) (time.Time, *string, bool, bool) {
	for _, param := range splitQuoted(p.Head, ';')[1:] {
		k, v, ok := strings.Cut(param, "=")
		if ok && strings.EqualFold(k, "VALUE") && strings.EqualFold(strings.Trim(v, "\""), "text") {
			text := unescape(p.Value)
			return time.Time{}, &text, true, false
		}
	}
	value := p.Value
	if strings.HasPrefix(value, "--") {
		if value == "--0229" || value == "--02-29" {
			value = "2000-02-29"
		} else {
			value = "1900-" + value[2:]
		}
	}
	for _, layout := range []string{"2006-01-02", "20060102", "2006-0102", "2006-01-02T15:04:05", "20060102T150405", "2006-01-02T15:04:05Z", "20060102T150405Z"} {
		if d, e := time.Parse(layout, value); e == nil {
			return d, nil, true, false
		}
	}
	for _, layout := range []string{time.RFC3339, "20060102T150405Z07:00", "20060102T150405Z0700", "2006-01-02T15:04:05Z0700"} {
		if d, e := time.Parse(layout, value); e == nil {
			return d, nil, true, true
		}
	}
	return time.Time{}, nil, false, false
}
func matchDateQuery(c *Card, field, term string) (bool, error) {
	date, text, present, zoned := parsedSearchDate(c, field)
	if !present {
		return false, nil
	}
	if text == nil {
		query, e := time.Parse("2006-1-2", term)
		if e != nil {
			return false, fmt.Errorf("%s query requires an ISO date (YYYY-MM-DD): %q", field, term)
		}
		return !zoned && date.Equal(query), nil
	}
	return strings.Contains(strings.ToLower(*text), term), nil
}
