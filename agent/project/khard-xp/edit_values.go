package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type pythonDate struct {
	time.Time
	Zoned bool
}

func (d pythonDate) String() string {
	if d.Zoned {
		return d.Format("2006-01-02 15:04:05.999999999-07:00")
	}
	return d.Format("2006-01-02 15:04:05.999999999")
}
func sortPropertyValues(values []any) {
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		am, aMap := a.(map[string]any)
		bm, bMap := b.(map[string]any)
		if aMap != bMap {
			return !aMap
		}
		if aMap {
			return sortedKeys(am)[0] < sortedKeys(bm)[0]
		}
		aa, aList := a.([]any)
		bb, bList := b.([]any)
		if aList && bList {
			for n := 0; n < len(aa) && n < len(bb); n++ {
				as, bs := fmt.Sprint(aa[n]), fmt.Sprint(bb[n])
				if as != bs {
					return as < bs
				}
			}
			return len(aa) < len(bb)
		}
		return fmt.Sprint(a) < fmt.Sprint(b)
	})
}
func (c *Card) applyDateField(field string, value any) error {
	key := "BDAY"
	if field == "Anniversary" {
		key = "ANNIVERSARY"
	}
	var props []Property
	if value != nil && value != "" {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a date string", field)
		}
		head := key
		if strings.HasPrefix(strings.TrimSpace(text), "text") {
			prefix, body, ok := strings.Cut(text, "=")
			if !ok || strings.TrimSpace(prefix) != "text" {
				return fmt.Errorf("invalid %s date", field)
			}
			if c.first("VERSION") != "4.0" {
				return fmt.Errorf("text dates require vCard 4.0")
			}
			text = strings.TrimSpace(body)
			head += ";VALUE=text"
			if text != "" {
				props = []Property{newProp(head, escape(text))}
			}
		} else {
			if strings.HasPrefix(text, "--") && c.first("VERSION") != "4.0" {
				return fmt.Errorf("reduced dates require vCard 4.0")
			}
			parsed, _, present, zoned := parseSearchDateProperty(newProp(key, text))
			if !present {
				return fmt.Errorf("invalid %s date %q; use YYYY-MM-DD, a date-time, or a vCard 4 text= value", field, text)
			}
			if strings.HasPrefix(text, "--") {
				text = parsed.Format("--0102")
			} else if zoned {
				text = parsed.Format("20060102T150405.999999999Z0700")
			} else if parsed.Hour() != 0 || parsed.Minute() != 0 || parsed.Second() != 0 || parsed.Nanosecond() != 0 {
				text = parsed.Format("20060102T150405.999999999")
			} else {
				text = parsed.Format("20060102")
			}
			if field == "Anniversary" && c.first("VERSION") == "3.0" {
				head = "X-ANNIVERSARY"
			}
			props = []Property{newProp(head, text)}
		}
	}
	if field == "Anniversary" {
		c.replaceExact("X-ANNIVERSARY", nil)
	}
	c.replaceExact(key, nil)
	for _, p := range props {
		c.replaceExact(p.Key, []Property{p})
	}
	return nil
}
func (c *Card) applyImpp(value any) error {
	values, ok := value.(map[string]any)
	if value != nil && !ok {
		return fmt.Errorf("Impp must be a scheme-to-handle mapping")
	}
	var props []Property
	for _, label := range sortedKeys(values) {
		var schemes []string
		pref := 0
		for _, part := range strings.Split(label, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "pref=") {
				n, e := strconv.Atoi(part[5:])
				if e != nil || n < 1 || n > 100 {
					return fmt.Errorf("invalid IMPP preference")
				}
				pref = n
			} else if strings.EqualFold(part, "pref") {
				pref = 1
			} else {
				if !validToken(part) {
					return fmt.Errorf("invalid IMPP scheme")
				}
				schemes = append(schemes, part)
			}
		}
		handles, e := textList(values[label])
		if e != nil {
			return e
		}
		for _, scheme := range schemes {
			for _, handle := range handles {
				head := "IMPP"
				if pref > 0 {
					head += ";PREF=" + strconv.Itoa(pref)
				}
				props = append(props, newProp(head, scheme+":"+escape(handle)))
			}
		}
	}
	c.replace("IMPP", props)
	return nil
}

func (c *Card) applyCategories(value any) error {
	var groups []any
	if list, ok := value.([]any); ok {
		plain := true
		for _, v := range list {
			if _, ok := v.(string); !ok && v != nil {
				plain = false
			}
		}
		if plain {
			groups = []any{list}
		} else {
			groups = list
		}
	} else if value != nil {
		groups = []any{value}
	}
	var props []Property
	for _, group := range groups {
		values, e := textList(group)
		if e != nil {
			return e
		}
		for i := range values {
			values[i] = escape(values[i])
		}
		if len(values) > 0 {
			props = append(props, newProp("CATEGORIES", strings.Join(values, ",")))
		}
	}
	c.replace("CATEGORIES", props)
	return nil
}
