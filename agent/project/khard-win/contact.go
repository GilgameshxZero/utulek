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
var fieldOrder = []string{"Formatted name", "Prefix", "First name", "Additional", "Last name", "Suffix", "Nickname", "Anniversary", "Birthday", "Organisation", "Title", "Role", "Phone", "Email", "Address", "Categories", "Webpage", "Private", "Note"}

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

func (c *Card) model(cfg Config) Model {
	m := Model{}
	for _, k := range fieldOrder {
		m[k] = nil
	}
	for f, key := range simpleFields {
		var ss []string
		for _, p := range c.properties(key) {
			if key == "CATEGORIES" || key == "NICKNAME" {
				for _, s := range splitEscaped(p.Value, ',') {
					ss = append(ss, unescape(s))
				}
			} else {
				ss = append(ss, unescape(p.Value))
			}
		}
		m[f] = oneOrMany(ss)
	}
	if p := c.properties("N"); len(p) > 0 {
		parts := splitEscaped(p[0].Value, ';')
		for i, f := range nameFields {
			if i < len(parts) {
				var ss []string
				for _, s := range splitEscaped(parts[i], ',') {
					ss = append(ss, unescape(s))
				}
				m[f] = oneOrMany(ss)
			}
		}
	}
	var orgs []any
	for _, p := range c.properties("ORG") {
		var ss []string
		for _, s := range splitEscaped(p.Value, ';') {
			ss = append(ss, unescape(s))
		}
		orgs = append(orgs, oneOrMany(ss))
	}
	if len(orgs) == 1 {
		if _, ok := orgs[0].([]any); ok {
			m["Organisation"] = orgs
		} else {
			m["Organisation"] = orgs[0]
		}
	} else if len(orgs) > 1 {
		m["Organisation"] = orgs
	}
	for f, key := range map[string]string{"Phone": "TEL", "Email": "EMAIL", "Address": "ADR"} {
		values := map[string]any{}
		for _, p := range c.properties(key) {
			label := c.label(p)
			var v any = unescape(p.Value)
			if key == "TEL" && strings.HasPrefix(strings.ToLower(p.Value), "tel:") {
				v = unescape(p.Value[4:])
			}
			if key == "ADR" {
				a := map[string]any{}
				parts := splitEscaped(p.Value, ';')
				for i, k := range addressFields {
					a[k] = nil
					if i < len(parts) && parts[i] != "" {
						a[k] = unescape(parts[i])
					}
				}
				v = a
			}
			appendModel(values, label, v)
		}
		if len(values) > 0 {
			m[f] = values
		}
	}
	private := map[string]any{}
	for _, name := range cfg.Private {
		if name == "" {
			continue
		}
		var ss []string
		for _, p := range c.properties("X-" + strings.ToUpper(name)) {
			ss = append(ss, unescape(p.Value))
		}
		private[name] = oneOrMany(ss)
	}
	if len(private) > 0 {
		m["Private"] = private
	}
	return m
}

func encodeModel(m Model) ([]byte, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range fieldOrder {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		val := &yaml.Node{}
		if e := val.Encode(m[k]); e != nil {
			return nil, e
		}
		n.Content = append(n.Content, key, val)
	}
	var b bytes.Buffer
	b.WriteString("# khard-win contact template. Empty Phone/Email labels (\"\") mean no TYPE.\n# Unchanged vCard fields, including photos and custom properties, are preserved.\n")
	e := yaml.NewEncoder(&b)
	e.SetIndent(4)
	err := e.Encode(n)
	if err == nil {
		err = e.Close()
	}
	return b.Bytes(), err
}

func decodeNode(n *yaml.Node, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("YAML nesting is too deep")
	}
	switch n.Kind {
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
			x, e := decodeNode(v, depth+1)
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
			x, e := decodeNode(n.Content[i+1], depth+1)
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

func labelHead(key, label, version string) (string, error) {
	head := key
	if key == "TEL" && version == "4.0" {
		head += ";VALUE=uri"
	}
	var types []string
	for _, v := range strings.Split(label, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(v), "pref=") {
			n := strings.TrimPrefix(strings.ToLower(v), "pref=")
			if n == "" || strings.Trim(n, "0123456789") != "" {
				return "", fmt.Errorf("invalid preference %q", v)
			}
			if version == "4.0" {
				head += ";PREF=" + n
			} else {
				types = append(types, "pref")
			}
			continue
		}
		if !validToken(v) {
			return "", fmt.Errorf("unsupported label %q; keep existing labels or use comma-separated type names", label)
		}
		types = append(types, v)
	}
	if len(types) > 0 {
		head += ";TYPE=" + strings.Join(types, ",")
	}
	return head, nil
}

func (c *Card) applyModel(m Model, cfg Config) (bool, error) {
	old := c.model(cfg)
	changed := false
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
				head, e := labelHead(key, label, version)
				if e != nil {
					return false, e
				}
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
					pp = append(pp, newProp(head, s))
				}
			}
			c.replace(key, pp)
			changed = true
		case "Organisation":
			var aa []any
			if v != nil {
				if seq, ok := v.([]any); ok {
					aa = seq
				} else {
					aa = []any{v}
				}
			}
			var pp []Property
			for _, org := range aa {
				ss, e := textList(org)
				if e != nil {
					return false, e
				}
				for i := range ss {
					ss[i] = escape(ss[i])
				}
				if len(ss) > 0 {
					pp = append(pp, newProp("ORG", strings.Join(ss, ";")))
				}
			}
			c.replace("ORG", pp)
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
				ss, e := textList(values[k])
				if e != nil {
					return false, e
				}
				key := "X-" + strings.ToUpper(k)
				var pp []Property
				for _, s := range ss {
					pp = append(pp, newProp(key, escape(s)))
				}
				c.replace(key, pp)
			}
			changed = true
		}
	}
	if c.first("FN") == "" {
		n := c.name("first_name")
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

func (c *Card) matches(terms []string, cfg Config) bool {
	m := c.model(cfg)
	fields := map[string][]string{"name": {c.name("first_name"), c.first("FN"), c.first("NICKNAME")}, "uid": {c.first("UID")}, "formatted_name": {c.first("FN")}, "kind": {c.first("KIND")}}
	for f, v := range m {
		key := strings.ReplaceAll(strings.ToLower(f), " ", "_")
		fields[key] = stringsValue(v)
	}
	fields["phone_numbers"] = fields["phone"]
	fields["emails"] = fields["email"]
	fields["addresses"] = fields["address"]
	fields["notes"] = fields["note"]
	fields["nicknames"] = fields["nickname"]
	fields["webpages"] = fields["webpage"]
	var all []string
	for _, p := range c.Props {
		if p.Key != "PHOTO" && p.Key != "LOGO" {
			all = append(all, unescape(p.Value))
		}
	}
	all = append(all, stringsValue(map[string]any(m))...)
	for _, term := range terms {
		hay := all
		field, value, ok := strings.Cut(term, ":")
		if vals, found := fields[strings.ToLower(field)]; ok && found {
			hay = vals
			term = value
		}
		term = strings.ToLower(term)
		found := false
		for _, s := range hay {
			if strings.Contains(strings.ToLower(s), term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
