package main

import (
	"fmt"
	"strings"
)

func cleanYAML(value any) any {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return v
	case []any:
		if len(v) == 0 {
			return nil
		}
		if len(v) == 1 {
			if _, ok := v[0].(string); ok {
				return cleanYAML(v[0])
			}
			if inner, ok := v[0].([]any); ok && len(inner) == 1 {
				if _, ok := inner[0].(string); ok {
					return cleanYAML(inner[0])
				}
			}
		}
		return v
	case map[string]any:
		if len(v) == 0 {
			return nil
		}
		out := make(map[string]any, len(v))
		for k, x := range v {
			out[k] = cleanYAML(x)
		}
		return out
	}
	return value
}
func (c *Card) model(cfg Config) Model {
	m := Model{}
	for _, field := range fieldOrder {
		m[field] = nil
	}
	view := columnValues{card: c}
	for field, property := range map[string]string{"Formatted name": "formatted_name", "Kind": "kind", "Nickname": "nicknames", "Organisation": "organisations", "Title": "titles", "Role": "roles", "Categories": "categories", "Webpage": "webpages", "Note": "notes", "Impp": "impp"} {
		m[field] = cleanYAML(view.property(property))
	}
	for _, key := range []string{"Birthday", "Anniversary"} {
		m[key] = isoModelDate(c, strings.ToLower(key))
	}
	if props := c.properties("N"); len(props) > 0 {
		parts := splitEscaped(props[0].Value, ';')
		for i, field := range nameFields {
			if i < len(parts) {
				var values []string
				for _, part := range splitEscaped(parts[i], ',') {
					values = append(values, unescape(part))
				}
				m[field] = oneOrMany(values)
			}
		}
	}
	for field, property := range map[string]string{"Phone": "phone_numbers", "Email": "emails", "Address": "post_addresses"} {
		values, _ := view.property(property).(map[string]any)
		if len(values) == 0 {
			continue
		}
		editable := map[string]any{}
		for label, raw := range values {
			if field != "Address" {
				editable[label] = cleanYAML(raw)
				continue
			}
			var addresses []any
			for _, value := range raw.([]any) {
				address := value.(map[string]any)
				out := map[string]any{}
				for _, name := range addressFields {
					out[name] = cleanYAML(address[strings.ToLower(name)])
				}
				addresses = append(addresses, out)
			}
			if len(addresses) == 1 {
				editable[label] = addresses[0]
			} else {
				editable[label] = addresses
			}
		}
		m[field] = editable
	}
	private := map[string]any{}
	for _, name := range cfg.Private {
		if name == "" {
			continue
		}
		var values []any
		for _, p := range c.properties("X-" + strings.ToUpper(name)) {
			var value any = unescape(p.Value)
			if label := propertyGroupLabel(c, p); label != "" {
				value = map[string]any{label: value}
			}
			values = append(values, value)
		}
		sortPropertyValues(values)
		if len(values) == 1 {
			private[name] = cleanYAML(values[0])
		} else {
			private[name] = cleanYAML(values)
		}
	}
	if len(private) > 0 {
		m["Private"] = private
	}
	return m
}

// Replacement is the upstream default. FN is exceptional: omitted FN retains
// the old formatted name; explicitly empty FN requests automatic generation.
func (c *Card) applyYAML(m Model, cfg Config, patch bool) (bool, error) {
	if !patch {
		if scalar(m["First name"]) == "" && scalar(m["Last name"]) == "" && scalar(m["Organisation"]) == "" && scalar(m["Formatted name"]) == "" {
			return false, fmt.Errorf("replacement YAML requires a name or organisation; use --patch for partial updates")
		}
		replacement := Model{}
		for _, field := range fieldOrder {
			replacement[field] = nil
		}
		delete(replacement, "Formatted name")
		for k, v := range m {
			replacement[k] = v
		}
		m = replacement
	}
	return c.applyModel(m, cfg)
}
func (c *Card) newGroup(key string) string {
	used := map[string]bool{}
	for _, p := range c.Props {
		used[strings.ToLower(p.Group)] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("item%s%d", strings.ToLower(key), i)
		if !used[name] {
			return name
		}
	}
}
func (c *Card) replaceExact(key string, values []Property) {
	var out []Property
	inserted := false
	for _, p := range c.Props {
		if p.Key == key {
			if !inserted {
				out = append(out, values...)
				inserted = true
			}
			continue
		}
		if p.Key == "END" && !inserted {
			out = append(out, values...)
			inserted = true
		}
		out = append(out, p)
	}
	c.Props = out
}
func (c *Card) clearOrphanLabels(old []Property) {
	oldGroups := map[string]bool{}
	for _, p := range old {
		if p.Key != "X-ABLABEL" && p.Group != "" {
			oldGroups[p.Group] = true
		}
	}
	used := map[string]bool{}
	for _, p := range c.Props {
		if p.Key != "X-ABLABEL" {
			used[p.Group] = true
		}
	}
	out := c.Props[:0]
	for _, p := range c.Props {
		if p.Key == "X-ABLABEL" && oldGroups[p.Group] && !used[p.Group] {
			continue
		}
		out = append(out, p)
	}
	c.Props = out
}
func (c *Card) taggedProperties(key string, value any, structured bool) ([]Property, error) {
	var items []any
	if value != nil {
		if values, ok := value.([]any); ok {
			items = values
		} else {
			items = []any{value}
		}
	}
	var props []Property
	for _, item := range items {
		if item == nil {
			continue
		}
		label := ""
		if mapping, ok := item.(map[string]any); ok {
			if len(mapping) != 1 {
				return nil, fmt.Errorf("%s tagged entries require exactly one label", key)
			}
			for k, v := range mapping {
				label = k
				item = v
			}
			if strings.ContainsAny(label, "\r\n\x00") {
				return nil, fmt.Errorf("invalid property label")
			}
		}
		values, e := textList(item)
		if e != nil {
			return nil, e
		}
		if len(values) == 0 {
			continue
		}
		if structured {
			for i := range values {
				values[i] = escape(values[i])
			}
			values = []string{strings.Join(values, ";")}
		}
		for _, text := range values {
			if !structured {
				text = escape(text)
			}
			var prop Property
			found := false
			for _, old := range c.properties(key) {
				if propertyGroupLabel(c, old) == label && old.Value == text {
					prop = old
					found = true
					break
				}
			}
			if !found {
				head := key
				if label != "" {
					group := c.newGroup(key)
					head = group + "." + head
					c.replaceExact("X-ABLABEL", append(c.properties("X-ABLABEL"), newProp(group+".X-ABLabel", escape(label))))
				}
				prop = newProp(head, text)
			}
			props = append(props, prop)
		}
	}
	return props, nil
}
func hasTagged(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		return true
	case []any:
		for _, x := range v {
			if hasTagged(x) {
				return true
			}
		}
	}
	return false
}

func (c *Card) autoFormattedName() string {
	names := c.nameParts()
	if names[0] != "" || names[1] != "" {
		var parts []string
		for _, index := range []int{3, 1, 0, 4} {
			if names[index] != "" {
				parts = append(parts, names[index])
			}
		}
		return strings.Join(parts, " ")
	}
	if orgs := c.properties("ORG"); len(orgs) > 0 {
		var parts []string
		for _, part := range splitEscaped(orgs[0].Value, ';') {
			parts = append(parts, unescape(part))
		}
		return strings.NewReplacer("\n", " ", "\\", "").Replace(strings.Join(parts, ", "))
	}
	return "Unnamed contact"
}
