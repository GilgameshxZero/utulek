package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Public property names used by upstream queries and list columns. Keep column
// traversal independent of the editable YAML model: lists remain lists even
// when they contain only one value.
var listProperties = map[string]string{
	"anniversary": "ANNIVERSARY", "birthday": "BDAY", "categories": "CATEGORIES",
	"emails": "EMAIL", "first_name": "N", "formatted_name": "FN", "kind": "KIND",
	"last_name": "N", "nicknames": "NICKNAME", "notes": "NOTE", "organisations": "ORG",
	"impp": "IMPP", "phone_numbers": "TEL", "post_addresses": "ADR", "roles": "ROLE", "titles": "TITLE",
	"uid": "UID", "version": "VERSION", "webpages": "URL",
}

func contactKind(c *Card) string {
	key := "KIND"
	if c.first("VERSION") == "3.0" {
		key = "X-KIND"
	}
	if kind := c.first(key); kind != "" {
		return kind
	}
	return "individual"
}

func parseListFields(input string) ([]string, error) {
	fields := strings.Split(input, ",")
	for i := range fields {
		fields[i] = strings.ToLower(strings.TrimSpace(fields[i]))
		root, _, nested := strings.Cut(fields[i], ".")
		if _, ok := listProperties[root]; ok {
			continue
		}
		if !nested && contains([]string{"index", "name", "phone", "email", "address_book", "addressbook"}, root) {
			continue
		}
		return nil, fmt.Errorf("unsupported list field %q", fields[i])
	}
	return fields, nil
}

func listFieldsHelp() string {
	fields := []string{"index", "name", "phone", "email", "address_book", "addressbook"}
	for field := range listProperties {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return strings.Join(fields, ",")
}

type columnValues struct {
	search bool
	card   *Card
	cache  map[string]any
}

func (v *columnValues) property(field string) any {
	if val, ok := v.cache[field]; ok {
		return val
	}
	if v.cache == nil {
		v.cache = make(map[string]any)
	}
	c, key := v.card, listProperties[field]
	var result any
	switch field {
	case "birthday", "anniversary":
		date, text, present, zoned := parsedSearchDate(c, field)
		if present {
			if text != nil {
				result = *text
			} else {
				result = pythonDate{Time: date, Zoned: zoned}
			}
		}
	case "uid", "version", "formatted_name":
		result = c.first(key)
	case "kind":
		result = contactKind(c)
	case "first_name", "last_name":
		index := 1
		if field == "last_name" {
			index = 0
		}
		names := c.nameParts()
		result = names[index]
	case "impp":
		values := map[string]any{}
		for _, p := range c.properties("IMPP") {
			scheme, handle, ok := strings.Cut(unescape(p.Value), ":")
			if !ok {
				continue
			}
			for _, param := range splitQuoted(p.Head, ';')[1:] {
				k, value, ok := strings.Cut(param, "=")
				if ok && strings.EqualFold(k, "PREF") {
					scheme += ", pref=" + value
				}
			}
			values[scheme] = append(anyList(values[scheme]), handle)
		}
		for _, value := range values {
			sort.SliceStable(value.([]any), func(i, j int) bool { return value.([]any)[i].(string) < value.([]any)[j].(string) })
		}
		result = values
	case "phone_numbers", "emails", "post_addresses":
		values := map[string]any{}
		for _, p := range c.Props {
			if p.Key != key {
				continue
			}
			label := c.label(p)
			if v.search {
				label = searchLabel(c, p, key)
			}
			var value any = unescape(p.Value)
			if key == "TEL" && strings.HasPrefix(strings.ToLower(p.Value), "tel:") {
				value = unescape(p.Value[4:])
			}
			if key == "ADR" {
				address := map[string]any{}
				parts := splitEscaped(p.Value, ';')
				for i, name := range addressFields {
					text := ""
					if i < len(parts) {
						text = unescape(parts[i])
					}
					components := splitEscaped(partsOrEmpty(parts, i), ',')
					if len(components) > 1 {
						var list []any
						for _, part := range components {
							list = append(list, unescape(part))
						}
						address[strings.ToLower(name)] = list
					} else {
						address[strings.ToLower(name)] = text
					}
				}
				value = address
			}
			values[label] = append(anyList(values[label]), value)
		}
		for label, raw := range values {
			entries := raw.([]any)
			sort.SliceStable(entries, func(i, j int) bool {
				if key == "ADR" {
					a, b := entries[i].(map[string]any), entries[j].(map[string]any)
					return strings.ToLower(joinPublicText(a["city"], " ")+"\x00"+joinPublicText(a["street"], " ")) < strings.ToLower(joinPublicText(b["city"], " ")+"\x00"+joinPublicText(b["street"], " "))
				}
				return entries[i].(string) < entries[j].(string)
			})
			values[label] = entries
		}
		result = values
	default:
		var values []any
		for _, p := range c.Props {
			if p.Key != key {
				continue
			}
			if key == "NICKNAME" || key == "CATEGORIES" {
				var parts []any
				for _, part := range splitEscaped(p.Value, ',') {
					var val any = unescape(part)
					if key == "NICKNAME" {
						if label := propertyGroupLabel(c, p); label != "" {
							val = map[string]any{label: val}
						}
					}
					parts = append(parts, val)
				}
				if key == "CATEGORIES" {
					values = append(values, parts)
				} else {
					values = append(values, parts...)
				}
			} else if key == "ORG" {
				var parts []any
				for _, part := range splitEscaped(p.Value, ';') {
					parts = append(parts, unescape(part))
				}
				var value any = parts
				if label := propertyGroupLabel(c, p); label != "" {
					value = map[string]any{label: value}
				}
				values = append(values, value)
			} else {
				var value any = unescape(p.Value)
				if label := propertyGroupLabel(c, p); label != "" {
					value = map[string]any{label: value}
				}
				values = append(values, value)
			}
		}
		if key == "CATEGORIES" && len(values) == 1 {
			result = values[0]
		} else {
			sortPropertyValues(values)
			result = values
		}
	}
	v.cache[field] = result
	return result
}

func anyList(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	return nil
}

func (v *columnValues) nested(field string) string {
	parts := strings.Split(field, ".")
	value := v.property(parts[0])
	for _, part := range parts[1:] {
		switch current := value.(type) {
		case map[string]any:
			value = current[part]
			if value == nil {
				value = current[strings.ToUpper(part)]
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) {
				return ""
			}
			value = current[index]
		default:
			return ""
		}
	}
	if len(parts) == 1 {
		if mapping, ok := value.(map[string]any); ok {
			return v.publicDictionary(parts[0], mapping)
		}
	}
	return pythonValue(value, false)
}

// Upstream's parsable list columns use Python's list/dict representation.
// Preserve that convention for collection columns as well as scalar paths.
func pythonValue(value any, quoted bool) string {
	switch val := value.(type) {
	case pythonDate:
		if quoted {
			return fmt.Sprintf("datetime.datetime(%d, %d, %d, %d, %d)", val.Year(), val.Month(), val.Day(), val.Hour(), val.Minute())
		}
		return val.String()
	case nil:
		if quoted {
			return "None"
		}
		return ""
	case string:
		if !quoted {
			return val
		}
		quote := "'"
		if strings.Contains(val, "'") && !strings.Contains(val, `"`) {
			quote = `"`
		}
		var escaped strings.Builder
		escaped.Grow(len(val) + 2)
		escaped.WriteString(quote)
		for _, r := range val {
			switch r {
			case '\\':
				escaped.WriteString(`\\`)
			case '\n':
				escaped.WriteString(`\n`)
			case '\r':
				escaped.WriteString(`\r`)
			case '\t':
				escaped.WriteString(`\t`)
			default:
				if !unicode.IsPrint(r) || unicode.IsSpace(r) && r != ' ' {
					if r <= 255 {
						fmt.Fprintf(&escaped, `\x%02x`, r)
					} else if r <= 65535 {
						fmt.Fprintf(&escaped, `\u%04x`, r)
					} else {
						fmt.Fprintf(&escaped, `\U%08x`, r)
					}
					continue
				}
				if string(r) == quote {
					escaped.WriteByte('\\')
				}
				escaped.WriteRune(r)
			}
		}
		escaped.WriteString(quote)
		return escaped.String()
	case []any:
		if len(val) == 0 && !quoted {
			return ""
		}
		parts := make([]string, len(val))
		for i := range val {
			parts[i] = pythonValue(val[i], true)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		if len(val) == 0 && !quoted {
			return ""
		}
		var parts []string
		for _, key := range publicMapKeys(val) {
			parts = append(parts, pythonValue(key, true)+": "+pythonValue(val[key], true))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprint(value)
	}
}

func partsOrEmpty(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return ""
}

func publicMapKeys(value map[string]any) []string {
	if len(value) == 7 {
		keys := []string{"box", "extended", "street", "code", "city", "region", "country"}
		isAddress := true
		for _, key := range keys {
			if _, ok := value[key]; !ok {
				isAddress = false
			}
		}
		if isAddress {
			return keys
		}
	}
	return sortedKeys(value)
}
func publicPythonValue(value any) string { return pythonValue(value, false) }

func (v *columnValues) publicDictionary(field string, values map[string]any) string {
	var keys []string
	seen := map[string]bool{}
	key := listProperties[field]
	for _, p := range v.card.Props {
		if p.Key != key {
			continue
		}
		label := v.card.label(p)
		if field == "impp" {
			label, _, _ = strings.Cut(unescape(p.Value), ":")
			for _, param := range splitQuoted(p.Head, ';')[1:] {
				k, x, ok := strings.Cut(param, "=")
				if ok && strings.EqualFold(k, "PREF") {
					label += ", pref=" + x
				}
			}
		}
		if _, ok := values[label]; ok && !seen[label] {
			keys = append(keys, label)
			seen[label] = true
		}
	}
	for _, label := range sortedKeys(values) {
		if !seen[label] {
			keys = append(keys, label)
		}
	}
	var parts []string
	for _, label := range keys {
		parts = append(parts, pythonValue(label, true)+": "+pythonValue(values[label], true))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func joinPublicText(value any, separator string) string {
	if list, ok := value.([]any); ok {
		parts := make([]string, len(list))
		for i, v := range list {
			parts[i] = fmt.Sprint(v)
		}
		return strings.Join(parts, separator)
	}
	text, _ := value.(string)
	return text
}
func sortedLabelKeys(values map[string]any) []string {
	keys := sortedKeys(values)
	sort.SliceStable(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
	return keys
}
