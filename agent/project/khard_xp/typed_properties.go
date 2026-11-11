package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func (c *Card) typedProperty(key, label, value string, preserve bool) (Property, error) {
	standards := map[string][]string{"EMAIL": {"home", "internet", "work"}, "TEL": {"home", "work", "text", "voice", "fax", "cell", "video", "pager", "textphone"}, "ADR": {"home", "work"}}[key]
	if c.first("VERSION") == "3.0" {
		switch key {
		case "EMAIL":
			standards = append(standards, "x400")
		case "TEL":
			standards = append(standards, "bbs", "car", "isdn", "msg", "modem", "pcs")
		case "ADR":
			standards = append(standards, "dom", "intl", "parcel", "postal")
		}
	}
	var types, regular []string
	custom := ""
	preference := 0
	for _, part := range strings.Split(label, ",") {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		switch {
		case part == "":
			continue
		case lower == "pref":
			preference++
		case strings.HasPrefix(lower, "pref="):
			n, e := strconv.Atoi(part[5:])
			if e != nil || n < 0 || n > 99 {
				return Property{}, fmt.Errorf("invalid preference")
			}
			preference += n
		case contains(standards, lower):
			types = append(types, part)
			regular = append(regular, part)
		default:
			if !validLabelToken(part) || custom != "" {
				return Property{}, fmt.Errorf("label requires standard types and at most one custom name")
			}
			custom = part
			if strings.HasPrefix(lower, "x-") {
				custom = part[2:]
				types = append(types, part)
			} else {
				types = append(types, "X-"+part)
			}
		}
	}
	desired := append([]string(nil), regular...)
	if custom != "" {
		desired = append([]string{custom}, desired...)
	}
	head := key
	if key == "TEL" && c.first("VERSION") == "4.0" {
		head += ";VALUE=uri"
	}
	if preference > 0 {
		if c.first("VERSION") == "4.0" {
			head += ";PREF=" + strconv.Itoa(preference)
			desired = append(desired, "pref="+strconv.Itoa(preference))
		} else {
			types = append(types, "pref")
			desired = append(desired, "pref")
		}
	}
	if preserve {
		wanted := strings.Join(desired, ", ")
		var candidate *Property
		for _, p := range c.properties(key) {
			if strings.EqualFold(c.label(p), wanted) {
				if p.Value == value {
					return p, nil
				}
				copy := p
				candidate = &copy
			}
		}
		if candidate != nil {
			return newProp(candidate.Head, value), nil
		}
	}
	if len(types) > 0 {
		head += ";TYPE=" + strings.Join(types, ",")
	}
	if custom != "" {
		groupKey := strings.ToLower(key)
		group := c.newGroup(groupKey)
		head = group + "." + head
		c.replaceExact("X-ABLABEL", append(c.properties("X-ABLABEL"), newProp(group+".X-ABLabel", escape(custom))))
	}
	return newProp(head, value), nil
}

func validLabelToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) {
			return false
		}
	}
	return true
}
