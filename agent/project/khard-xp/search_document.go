package main

import (
	"fmt"
	"strings"
)

// Build the upstream-visible search text only when a general query needs it.
// This renderer is shared by detailed CLI output and general search.
func searchDocument(c *Card, cfg Config, view *columnValues) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", c.first("FN"))
	names := c.nameParts()
	if names[0] != "" || names[1] != "" {
		var parts []string
		for _, i := range []int{3, 1, 2, 0, 4} {
			if names[i] != "" {
				parts = append(parts, names[i])
			}
		}
		fmt.Fprintf(&b, "Full name: %s\n", strings.Join(parts, " "))
	}
	write := func(label, field string, indent int) {
		value := view.property(field)
		if searchValuePresent(value) {
			writeSearchYAML(&b, label, value, indent)
		}
	}
	fmt.Fprintf(&b, "Kind: %s\n", contactKind(c))
	write("Organisation", "organisations", 0)
	fmt.Fprintf(&b, "Address book: %s\n", c.Book)
	anniversary, birthday := formatContactDate(c, "anniversary", cfg), formatContactDate(c, "birthday", cfg)
	if anniversary != "" || birthday != "" || searchValuePresent(view.property("nicknames")) || searchValuePresent(view.property("roles")) || searchValuePresent(view.property("titles")) {
		b.WriteString("General:\n")
		if anniversary != "" {
			fmt.Fprintf(&b, "    Anniversary: %s\n", anniversary)
		}
		if birthday != "" {
			fmt.Fprintf(&b, "    Birthday: %s\n", birthday)
		}
		write("Nickname", "nicknames", 4)
		write("Role", "roles", 4)
		write("Title", "titles", 4)
	}
	for _, section := range []struct{ heading, field, implicit string }{{"Phone", "phone_numbers", "voice"}, {"E-Mail", "emails", "internet"}, {"IMPP", "impp", ""}, {"Address", "post_addresses", "home"}} {
		values, _ := view.property(section.field).(map[string]any)
		if len(values) == 0 {
			continue
		}
		b.WriteString(section.heading + "\n")
		for _, label := range sortedLabelKeys(values) {
			value := values[label]
			if section.field == "post_addresses" {
				var addresses []any
				for _, raw := range value.([]any) {
					addresses = append(addresses, searchAddress(raw.(map[string]any)))
				}
				value = addresses
			}
			if label == "" {
				label = section.implicit
			}
			writeSearchYAML(&b, label, value, 4)
		}
	}
	privateHeading := false
	for _, name := range cfg.Private {
		var values []any
		for _, p := range c.Props {
			if p.Key == "X-"+strings.ToUpper(name) {
				var value any = unescape(p.Value)
				if label := propertyGroupLabel(c, p); label != "" {
					value = map[string]any{label: value}
				}
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			sortPropertyValues(values)
			if !privateHeading {
				b.WriteString("Private:\n")
				privateHeading = true
			}
			writeSearchYAML(&b, name, values, 4)
		}
	}
	if c.first("UID") != "" || searchValuePresent(view.property("categories")) || searchValuePresent(view.property("webpages")) || searchValuePresent(view.property("notes")) {
		b.WriteString("Miscellaneous\n")
		if c.first("UID") != "" {
			fmt.Fprintf(&b, "    UID: %s\n", c.first("UID"))
		}
		write("Categories", "categories", 4)
		write("Webpage", "webpages", 4)
		write("Note", "notes", 4)
	}
	return b.String()
}
func searchValuePresent(value any) bool {
	switch v := value.(type) {
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return false
}
func writeSearchYAML(b *strings.Builder, label string, value any, indent int) {
	if list, ok := value.([]any); ok && len(list) == 1 {
		if _, ok := list[0].(string); ok {
			value = list[0]
		} else if sub, ok := list[0].([]any); ok && len(sub) == 1 {
			if _, ok := sub[0].(string); ok {
				value = sub[0]
			}
		}
	}
	prefix := strings.Repeat(" ", indent)
	if text, ok := value.(string); ok {
		b.WriteString(prefix + label + ": ")
		writeSearchText(b, text, indent+4)
		b.WriteByte('\n')
		return
	}
	list, ok := value.([]any)
	if !ok {
		return
	}
	b.WriteString(prefix + label + ": \n")
	for _, item := range list {
		if sub, ok := item.([]any); ok && len(sub) == 1 {
			if _, ok := sub[0].(string); ok {
				item = sub[0]
			}
		}
		switch v := item.(type) {
		case string:
			b.WriteString(prefix + "    - ")
			writeSearchText(b, v, indent+8)
			b.WriteByte('\n')
		case []any:
			b.WriteString(prefix + "    - \n")
			for _, s := range v {
				if text, ok := s.(string); ok {
					b.WriteString(prefix + "        - ")
					writeSearchText(b, text, indent+12)
					b.WriteByte('\n')
				}
			}
		case map[string]any:
			for _, key := range sortedKeys(v) {
				writeSearchYAML(b, "- "+key, v[key], indent+4)
			}
		}
	}
}
func writeSearchText(b *strings.Builder, text string, indent int) {
	if strings.Contains(text, "\n") || strings.Contains(text, ": ") {
		for _, line := range strings.Split(text, "\n") {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(strings.TrimSpace(line))
		}
	} else {
		b.WriteString(strings.TrimSpace(text))
	}
}
func propertyGroupLabel(c *Card, p Property) string {
	if p.Group == "" {
		return ""
	}
	count := 0
	label := ""
	for _, child := range c.Props {
		if child.Group == p.Group {
			count++
			if child.Key == "X-ABLABEL" {
				label = unescape(child.Value)
			}
		}
	}
	if count == 2 {
		return label
	}
	return ""
}
func searchAddress(address map[string]any) string {
	get := func(key string, separator string) string {
		value := address[key]
		if list, ok := value.([]any); ok {
			var parts []string
			for _, v := range list {
				parts = append(parts, v.(string))
			}
			return strings.Join(parts, separator)
		}
		s, _ := value.(string)
		return s
	}
	var lines []string
	if s := get("street", "\n"); s != "" {
		lines = append(lines, s)
	}
	for _, pair := range []struct{ a, b, sep string }{{"box", "extended", " "}, {"code", "city", " "}, {"region", "country", ", "}} {
		a, b := get(pair.a, " "), get(pair.b, " ")
		if a != "" && b != "" {
			lines = append(lines, a+pair.sep+b)
		} else if a != "" {
			lines = append(lines, a)
		} else if b != "" {
			lines = append(lines, b)
		}
	}
	return strings.Join(lines, "\n")
}
