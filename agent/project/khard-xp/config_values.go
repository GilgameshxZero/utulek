package main

import (
	"bufio"
	"fmt"
	"strings"
)

// ConfigObj quotes are literal: never interpret Windows backslashes or trim
// whitespace inside quotes. A final comma denotes a list, not an extra item.
func configValue(s string) ([]string, bool, error) {
	s = strings.TrimSpace(s)
	var values []string
	list := false
	for {
		if s == "" || s[0] == '#' {
			if !list && len(values) == 0 {
				return []string{""}, false, nil
			}
			return values, list, nil
		}
		var value string
		quoted := false
		if s[0] == '\'' || s[0] == '"' {
			quoted = true
			quote := s[0]
			end := strings.IndexByte(s[1:], quote)
			if end < 0 {
				return nil, false, fmt.Errorf("unclosed quote")
			}
			value = s[1 : end+1]
			s = strings.TrimSpace(s[end+2:])
			if s != "" && s[0] != ',' && s[0] != '#' {
				return nil, false, fmt.Errorf("text after quoted value")
			}
		} else {
			end := strings.IndexAny(s, ",#")
			if end < 0 {
				end = len(s)
			}
			value = strings.TrimSpace(s[:end])
			s = s[end:]
		}
		if !quoted && value == "" && list {
			return nil, false, fmt.Errorf("empty unquoted list item")
		}
		if quoted || value != "" || len(values) > 0 || s == "" || s[0] != ',' {
			values = append(values, value)
		}
		if s == "" || s[0] == '#' {
			return values, list, nil
		}
		list = true
		s = strings.TrimSpace(s[1:])
	}
}

func configScannedValue(scan *bufio.Scanner, lineNo *int, s string) ([]string, bool, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "\"\"\"") && !strings.HasPrefix(s, "'''") {
		return configValue(s)
	}
	delimiter := s[:3]
	s = s[3:]
	var value strings.Builder
	for {
		if end := strings.Index(s, delimiter); end >= 0 {
			value.WriteString(s[:end])
			tail := strings.TrimSpace(s[end+3:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return nil, false, fmt.Errorf("text after multiline value")
			}
			return []string{value.String()}, false, nil
		}
		value.WriteString(s)
		if value.Len() > 1024*1024 {
			return nil, false, fmt.Errorf("multiline value exceeds 1 MiB")
		}
		if !scan.Scan() {
			if e := scan.Err(); e != nil {
				return nil, false, e
			}
			return nil, false, fmt.Errorf("unclosed multiline quote")
		}
		*lineNo++
		value.WriteByte('\n')
		s = scan.Text()
	}
}
