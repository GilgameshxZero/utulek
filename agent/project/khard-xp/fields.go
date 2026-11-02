package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Public property names used by upstream queries and list columns. Keep column
// traversal independent of the editable YAML model: lists remain lists even
// when they contain only one value.
var listProperties = map[string]string{
	"anniversary": "ANNIVERSARY", "birthday": "BDAY", "categories": "CATEGORIES",
	"emails": "EMAIL", "first_name": "N", "formatted_name": "FN", "kind": "KIND",
	"last_name": "N", "nicknames": "NICKNAME", "notes": "NOTE", "organisations": "ORG",
	"phone_numbers": "TEL", "post_addresses": "ADR", "roles": "ROLE", "titles": "TITLE",
	"uid": "UID", "version": "VERSION", "webpages": "URL",
}

func contactKind(c *Card) string {
	switch kind := c.first("KIND"); kind {
	case "":
		return "individual"
	case "org":
		return "organisation"
	default:
		return kind
	}
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
	card  *Card
	cache map[string]any
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
	case "uid", "version", "formatted_name", "birthday", "anniversary":
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
	case "phone_numbers", "emails", "post_addresses":
		values := map[string]any{}
		for _, p := range c.Props {
			if p.Key != key {
				continue
			}
			label := c.label(p)
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
					return strings.ToLower(fmt.Sprint(a["city"], "\x00", a["street"])) < strings.ToLower(fmt.Sprint(b["city"], "\x00", b["street"]))
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
				for _, part := range splitEscaped(p.Value, ',') {
					values = append(values, unescape(part))
				}
			} else if key == "ORG" {
				var parts []any
				for _, part := range splitEscaped(p.Value, ';') {
					parts = append(parts, unescape(part))
				}
				values = append(values, parts)
			} else {
				values = append(values, unescape(p.Value))
			}
		}
		sort.SliceStable(values, func(i, j int) bool {
			return strings.ToLower(pythonValue(values[i], false)) < strings.ToLower(pythonValue(values[j], false))
		})
		result = values
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
	return pythonValue(value, false)
}

// Upstream's parsable list columns use Python's list/dict representation.
// Preserve that convention for collection columns as well as scalar paths.
func pythonValue(value any, quoted bool) string {
	switch val := value.(type) {
	case nil:
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
		for _, key := range sortedKeys(val) {
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
