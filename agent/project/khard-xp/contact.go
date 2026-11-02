package main

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Model map[string]any

var nameFields = []string{"Last name", "First name", "Additional", "Prefix", "Suffix"}
var addressFields = []string{"Box", "Extended", "Street", "City", "Region", "Code", "Country"}
var simpleFields = map[string]string{"Formatted name": "FN", "Nickname": "NICKNAME", "Birthday": "BDAY", "Anniversary": "ANNIVERSARY", "Title": "TITLE", "Role": "ROLE", "Webpage": "URL", "Note": "NOTE", "Categories": "CATEGORIES"}
var fieldOrder = []string{"Formatted name", "Kind", "Prefix", "First name", "Additional", "Last name", "Suffix", "Nickname", "Anniversary", "Birthday", "Organisation", "Title", "Role", "Phone", "Email", "Impp", "Address", "Categories", "Webpage", "Private", "Note"}

func stringsValue(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var r []string
		for _, x := range t {
			r = append(r, stringsValue(x)...)
		}
		return r
	case map[string]any:
		var r []string
		for _, k := range sortedKeys(t) {
			r = append(r, k)
			r = append(r, stringsValue(t[k])...)
		}
		return r
	}
	return nil
}
func scalar(v any) string { return strings.Join(stringsValue(v), ", ") }
func sortedKeys(m map[string]any) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func oneOrMany(ss []string) any {
	if len(ss) == 0 {
		return nil
	}
	if len(ss) == 1 {
		return ss[0]
	}
	r := make([]any, len(ss))
	for i, s := range ss {
		r[i] = s
	}
	return r
}
func appendModel(m map[string]any, k string, v any) {
	if m[k] == nil {
		m[k] = v
		return
	}
	if a, ok := m[k].([]any); ok {
		m[k] = append(a, v)
	} else {
		m[k] = []any{m[k], v}
	}
}

func encodeModel(m Model) ([]byte, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range fieldOrder {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		val := &yaml.Node{}
		value := m[k]
		if e := val.Encode(value); e != nil {
			return nil, e
		}
		emptyNullNodes(val)
		n.Content = append(n.Content, key, val)
	}
	var b bytes.Buffer
	b.WriteString("# khard-xp contact template. Empty Phone/Email labels (\"\") mean no TYPE.\n# Unchanged vCard fields, including photos and custom properties, are preserved.\n")
	e := yaml.NewEncoder(&b)
	e.SetIndent(4)
	err := e.Encode(n)
	if err == nil {
		err = e.Close()
	}
	return b.Bytes(), err
}

func decodeNode(n *yaml.Node, depth int) (any, error) {
	budget := 100000
	return decodeNodeLimited(n, depth, &budget)
}
func decodeNodeLimited(n *yaml.Node, depth int, budget *int) (any, error) {
	*budget--
	if *budget < 0 {
		return nil, fmt.Errorf("YAML expansion exceeds 100000 nodes")
	}
	if depth > 32 {
		return nil, fmt.Errorf("YAML nesting is too deep")
	}
	switch n.Kind {
	case yaml.AliasNode:
		if n.Alias == nil {
			return nil, fmt.Errorf("unresolved YAML alias")
		}
		return decodeNodeLimited(n.Alias, depth+1, budget)
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			return nil, nil
		}
		if strings.ContainsRune(n.Value, 0) {
			return nil, fmt.Errorf("NUL is not allowed")
		}
		return n.Value, nil
	case yaml.SequenceNode:
		r := []any{}
		for _, v := range n.Content {
			x, e := decodeNodeLimited(v, depth+1, budget)
			if e != nil {
				return nil, e
			}
			r = append(r, x)
		}
		return r, nil
	case yaml.MappingNode:
		r := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("YAML keys must be strings")
			}
			if _, ok := r[k.Value]; ok {
				return nil, fmt.Errorf("duplicate YAML key %q", k.Value)
			}
			x, e := decodeNodeLimited(n.Content[i+1], depth+1, budget)
			if e != nil {
				return nil, e
			}
			r[k.Value] = x
		}
		return r, nil
	default:
		return nil, fmt.Errorf("YAML aliases and custom structures are unsupported in contact templates")
	}
}

func decodeModel(data []byte) (Model, error) {
	if len(data) > 16*1024*1024 {
		return nil, fmt.Errorf("contact template exceeds 16 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	var node yaml.Node
	if e := d.Decode(&node); e != nil {
		return nil, e
	}
	if len(node.Content) != 1 {
		return nil, fmt.Errorf("expected one YAML mapping")
	}
	v, e := decodeNode(node.Content[0], 0)
	if e != nil {
		return nil, e
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("contact template must be a YAML mapping")
	}
	var extra yaml.Node
	if e = d.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("expected one YAML document")
	}
	for k := range m {
		found := false
		for _, f := range fieldOrder {
			if f == k {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown contact field %q", k)
		}
	}
	return Model(m), nil
}

func textList(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
	if a, ok := v.([]any); ok {
		var out []string
		for _, v := range a {
			s, ok := v.(string)
			if !ok && v != nil {
				return nil, fmt.Errorf("expected a string or list of strings")
			}
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("expected a string or list of strings")
}

func (c *Card) applyModel(m Model, cfg Config) (bool, error) {
	work := cloneCard(c)
	_, e := work.applyModelInPlace(m, cfg)
	if e != nil {
		return false, e
	}
	if reflect.DeepEqual(c.Props, work.Props) {
		return false, nil
	}
	c.Props = work.Props
	return true, nil
}
func (c *Card) applyModelInPlace(m Model, cfg Config) (bool, error) {
	old := c.model(cfg)
	changed := false
	originalProps := c.Props
	defer func() { c.clearOrphanLabels(originalProps) }()
	version := c.first("VERSION")
	if version != "3.0" && version != "4.0" {
		return false, fmt.Errorf("editing vCard %s is unsupported", version)
	}
	// Missing fields in an input file leave existing fields alone. Explicit null
	// clears a field. Compare representations before touching original properties.
	nameChanged := false
	for _, f := range nameFields {
		if v, ok := m[f]; ok && !reflect.DeepEqual(v, old[f]) {
			nameChanged = true
		}
	}
	if nameChanged {
		var parts []string
		for _, f := range nameFields {
			v := old[f]
			if x, ok := m[f]; ok {
				v = x
			}
			ss, e := textList(v)
			if e != nil {
				return false, fmt.Errorf("%s: %w", f, e)
			}
			for i := range ss {
				ss[i] = escape(ss[i])
			}
			parts = append(parts, strings.Join(ss, ","))
		}
		c.replace("N", []Property{newProp("N", strings.Join(parts, ";"))})
		changed = true
	}
	for _, f := range fieldOrder {
		v, present := m[f]
		if !present || reflect.DeepEqual(v, old[f]) {
			continue
		}
		if contains(nameFields, f) {
			continue
		}
		if f == "Kind" {
			kind := scalar(v)
			key := "KIND"
			if version == "3.0" {
				key = "X-KIND"
			}
			var props []Property
			if kind != "" {
				kind = strings.ToLower(kind)
				if !contains([]string{"individual", "group", "org", "location", "application", "device"}, kind) {
					return false, fmt.Errorf("invalid contact kind %q", kind)
				}
				props = []Property{newProp(key, kind)}
			}
			c.replace(key, props)
			changed = true
			continue
		}
		if f == "Categories" {
			if e := c.applyCategories(v); e != nil {
				return false, e
			}
			changed = true
			continue
		}
		if f == "Birthday" || f == "Anniversary" {
			if e := c.applyDateField(f, v); e != nil {
				return false, e
			}
			changed = true
			continue
		}
		if f == "Impp" {
			if e := c.applyImpp(v); e != nil {
				return false, e
			}
			changed = true
			continue
		}
		if f == "Organisation" || ((f == "Nickname" || f == "Title" || f == "Role" || f == "Webpage" || f == "Note") && hasTagged(v)) {
			key := simpleFields[f]
			if f == "Organisation" {
				key = "ORG"
			}
			props, e := c.taggedProperties(key, v, f == "Organisation")
			if e != nil {
				return false, e
			}
			c.replaceExact(key, props)
			changed = true
			continue
		}
		key := simpleFields[f]
		if key != "" {
			ss, e := textList(v)
			if e != nil {
				return false, fmt.Errorf("%s: %w", f, e)
			}
			var pp []Property
			if key == "FN" && len(ss) > 1 {
				return false, fmt.Errorf("Formatted name must be one string")
			}
			if key == "CATEGORIES" || key == "NICKNAME" {
				for i := range ss {
					ss[i] = escape(ss[i])
				}
				if len(ss) > 0 {
					pp = append(pp, newProp(key, strings.Join(ss, ",")))
				}
			} else {
				for _, s := range ss {
					pp = append(pp, newProp(key, escape(s)))
				}
			}
			c.replace(key, pp)
			changed = true
			continue
		}
		switch f {
		case "Phone", "Email", "Address":
			key = map[string]string{"Phone": "TEL", "Email": "EMAIL", "Address": "ADR"}[f]
			values, ok := v.(map[string]any)
			if v != nil && !ok {
				return false, fmt.Errorf("%s must be a label-to-value mapping", f)
			}
			var pp []Property
			for _, label := range sortedKeys(values) {
				var e error
				var ss []string
				if key == "ADR" {
					a := values[label]
					var aa []any
					if a == nil {
						continue
					}
					if seq, ok := a.([]any); ok {
						aa = seq
					} else {
						aa = []any{a}
					}
					for _, v := range aa {
						addr, ok := v.(map[string]any)
						if !ok {
							return false, fmt.Errorf("Address.%s must contain address components", label)
						}
						for k := range addr {
							if !contains(addressFields, k) {
								return false, fmt.Errorf("unknown address component %s", k)
							}
						}
						var parts []string
						empty := true
						for _, k := range addressFields {
							vals, e := textList(addr[k])
							if e != nil {
								return false, e
							}
							if len(vals) > 0 {
								empty = false
							}
							for i := range vals {
								vals[i] = escape(vals[i])
							}
							parts = append(parts, strings.Join(vals, ","))
						}
						if !empty {
							ss = append(ss, strings.Join(parts, ";"))
						}
					}
				} else {
					ss, e = textList(values[label])
					if e != nil {
						return false, fmt.Errorf("%s.%s: %w", f, label, e)
					}
					for i := range ss {
						if key == "TEL" && strings.HasPrefix(strings.ToLower(ss[i]), "tel:") {
							ss[i] = ss[i][4:]
						}
						if key == "TEL" && version == "4.0" {
							if strings.ContainsAny(ss[i], "\r\n") {
								return false, fmt.Errorf("phone URI must not contain newlines")
							}
							ss[i] = "tel:" + ss[i]
						} else {
							ss[i] = escape(ss[i])
						}
					}
				}
				for _, s := range ss {
					prop, e := c.typedProperty(key, label, s, true)
					if e != nil {
						return false, e
					}
					pp = append(pp, prop)
				}
			}
			c.replaceExact(key, pp)
			changed = true
		case "Private":
			values, ok := v.(map[string]any)
			if v != nil && !ok {
				return false, fmt.Errorf("Private must be a mapping")
			}
			for k := range values {
				if !contains(cfg.Private, k) {
					return false, fmt.Errorf("private object %q is not configured", k)
				}
			}
			oldValues, _ := old[f].(map[string]any)
			for _, k := range cfg.Private {
				if k == "" {
					continue
				}
				if reflect.DeepEqual(values[k], oldValues[k]) {
					continue
				}
				key := "X-" + strings.ToUpper(k)
				props, e := c.taggedProperties(key, values[k], false)
				if e != nil {
					return false, e
				}
				c.replaceExact(key, props)
			}
			changed = true
		}
	}
	if c.first("FN") == "" {
		n := c.autoFormattedName()
		if n == "Unnamed contact" {
			return false, fmt.Errorf("contact needs a formatted name or name components")
		}
		c.replace("FN", []Property{newProp("FN", escape(n))})
		changed = true
	}
	return changed, nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// Search aliases share the same model fields used by YAML editing.
var searchModelFields = func() map[string]string {
	fields := make(map[string]string, len(fieldOrder)+6)
	for _, field := range fieldOrder {
		fields[strings.ReplaceAll(strings.ToLower(field), " ", "_")] = field
	}
	for alias, field := range map[string]string{"phone_numbers": "Phone", "emails": "Email", "addresses": "Address", "notes": "Note", "nicknames": "Nickname", "webpages": "Webpage", "post_addresses": "Address", "organisations": "Organisation", "roles": "Role", "titles": "Title"} {
		fields[alias] = field
	}
	return fields
}()

func phoneDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == '+' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func emptyNullNodes(n *yaml.Node) {
	if n.Tag == "!!null" {
		n.Value = ""
	}
	for _, child := range n.Content {
		emptyNullNodes(child)
	}
}
