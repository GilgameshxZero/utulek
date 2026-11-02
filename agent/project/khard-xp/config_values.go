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
		emptyTail := strings.TrimSpace(strings.TrimPrefix(s, ","))
		if !quoted && value == "" && (list || (emptyTail != "" && !strings.HasPrefix(emptyTail, "#"))) {
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
			if value.Len() > 1024*1024 {
				return nil, false, fmt.Errorf("multiline value exceeds 1 MiB")
			}
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

// Section names may contain # and brackets when quoted; only text after the
// closing delimiter is a comment. Do not split on # inside the section name.
func configSection(line string) (int, string, error) {
	depth := 0
	for depth < len(line) && line[depth] == '[' {
		depth++
	}
	if depth < 1 || depth > 2 {
		return 0, "", fmt.Errorf("invalid section depth")
	}
	closing := strings.Repeat("]", depth)
	quote := byte(0)
	for i := depth; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if strings.HasPrefix(line[i:], closing) {
			tail := strings.TrimSpace(line[i+depth:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return 0, "", fmt.Errorf("text after section header")
			}
			name := strings.TrimSpace(line[depth:i])
			if len(name) > 0 && (name[0] == '\'' || name[0] == '"') {
				if len(name) < 2 || name[len(name)-1] != name[0] {
					return 0, "", fmt.Errorf("invalid quoted section name")
				}
				name = name[1 : len(name)-1]
			}
			if name == "" {
				return 0, "", fmt.Errorf("empty section name")
			}
			return depth, name, nil
		}
	}
	return 0, "", fmt.Errorf("unclosed section header")
}
