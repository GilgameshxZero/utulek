package main

import (
	"strings"
	"unicode"
)

// A substring wholly inside a displayed scalar is a definite positive. This
// avoids rendering the complete document for common name/value searches, but
// never accepts raw syntax, unknown fields, or whitespace across display lines.
func quickVisibleMatch(c *Card, cfg Config, term string) bool {
	if term == "" || strings.TrimSpace(term) != term || strings.ContainsAny(term, "\r\n") {
		return false
	}
	has := func(s string) bool { return strings.Contains(strings.ToLower(s), term) }
	if has("Name:") || has("Address book:") || has("Kind:") || has(c.first("FN")) || has(c.first("UID")) || has(c.Book) || has(contactKind(c)) {
		return true
	}
	if c.first("UID") != "" && (has("Miscellaneous") || has("UID:")) {
		return true
	}
	for _, p := range c.Props {
		switch p.Key {
		case "N":
			parts := splitEscaped(p.Value, ';')
			if len(parts) < 2 || (parts[0] == "" && parts[1] == "") {
				continue
			}
			if has("Full name:") {
				return true
			}
			for _, part := range parts {
				for _, v := range splitEscaped(part, ',') {
					if has(unescape(v)) {
						return true
					}
				}
			}
		case "ORG":
			if has("Organisation:") || has(propertyGroupLabel(c, p)) {
				return true
			}
			for _, part := range splitEscaped(p.Value, ';') {
				if has(unescape(part)) {
					return true
				}
			}
		case "ADR":
			if has("Address") || has(searchLabel(c, p, "ADR")) {
				return true
			}
			for _, part := range splitEscaped(p.Value, ';') {
				for _, v := range splitEscaped(part, ',') {
					if has(unescape(v)) {
						return true
					}
				}
			}
		case "NICKNAME", "CATEGORIES":
			heading := "Nickname:"
			section := "General:"
			if p.Key == "CATEGORIES" {
				heading = "Categories:"
				section = "Miscellaneous"
			}
			if has(heading) || has(section) || (p.Key == "NICKNAME" && has(propertyGroupLabel(c, p))) {
				return true
			}
			for _, v := range splitEscaped(p.Value, ',') {
				if has(unescape(v)) {
					return true
				}
			}
		case "NOTE", "URL", "TITLE", "ROLE":
			heading, section := "Note:", "Miscellaneous"
			switch p.Key {
			case "URL":
				heading = "Webpage:"
			case "TITLE":
				heading = "Title:"
				section = "General:"
			case "ROLE":
				heading = "Role:"
				section = "General:"
			}
			if has(heading) || has(section) || has(unescape(p.Value)) || has(propertyGroupLabel(c, p)) {
				return true
			}
		case "IMPP":
			scheme, handle, ok := strings.Cut(unescape(p.Value), ":")
			if ok && (has("IMPP") || has(scheme) || has(handle)) {
				return true
			}
			for _, param := range splitQuoted(p.Head, ';')[1:] {
				key, value, ok := strings.Cut(param, "=")
				if ok && strings.EqualFold(key, "PREF") && has("pref="+value) {
					return true
				}
			}
		case "EMAIL":
			if has("E-Mail") || has(unescape(p.Value)) || has(searchLabel(c, p, "EMAIL")) {
				return true
			}
		case "TEL":
			value := unescape(p.Value)
			if strings.HasPrefix(strings.ToLower(value), "tel:") {
				value = value[4:]
			}
			if has("Phone") || has(value) || has(searchLabel(c, p, "TEL")) {
				return true
			}
		default:
			if strings.HasPrefix(p.Key, "X-") {
				for _, name := range cfg.Private {
					if p.Key == "X-"+strings.ToUpper(name) && (has("Private:") || has(name) || has(unescape(p.Value)) || has(propertyGroupLabel(c, p))) {
						return true
					}
				}
			}
		}
	}
	for _, field := range []string{"birthday", "anniversary"} {
		if text := formatContactDate(c, field, cfg); text != "" && (has(text) || has(field+":") || has("General:")) {
			return true
		}
	}
	return false
}

// With no whitespace or colon, a term cannot straddle rendered fields, name
// components or list items. The exhaustive visible-scalar check is definitive.
func simpleSearchAtom(term string) bool {
	return term != "" && !strings.ContainsRune(term, ':') && strings.IndexFunc(term, unicode.IsSpace) < 0
}

// Search uses upstream's implied labels and custom TYPE normalization without
// modifying the stored data or the intentionally empty labels in TSV output.
func searchLabel(c *Card, p Property, key string) string { return labelValue(c, p, key, true) }
func labelValue(c *Card, p Property, key string, implicit bool) string {
	var labels []string
	if p.Group != "" {
		for _, q := range c.Props {
			if q.Group == p.Group && q.Key == "X-ABLABEL" {
				if s := strings.TrimSpace(unescape(q.Value)); s != "" {
					labels = append(labels, s)
				}
			}
		}
	}
	preference := ""
	for _, field := range splitQuoted(p.Head, ';')[1:] {
		k, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "\"")
		switch strings.ToUpper(k) {
		case "TYPE":
			for _, v := range strings.Split(value, ",") {
				v = strings.TrimSpace(v)
				if strings.EqualFold(v, "pref") {
					if preference == "" {
						preference = "pref"
					}
					continue
				}
				if strings.HasPrefix(strings.ToLower(v), "x-") {
					v = v[2:]
					duplicate := false
					for _, old := range labels {
						if strings.EqualFold(old, v) {
							duplicate = true
						}
					}
					if duplicate {
						continue
					}
				}
				if v != "" {
					labels = append(labels, v)
				}
			}
		case "PREF":
			preference = "pref=" + value
		}
	}
	if preference != "" {
		labels = append(labels, preference)
	}
	if len(labels) == 0 && implicit {
		switch key {
		case "TEL":
			return "voice"
		case "EMAIL":
			return "internet"
		case "ADR":
			return "home"
		}
	}
	return strings.Join(labels, ", ")
}
