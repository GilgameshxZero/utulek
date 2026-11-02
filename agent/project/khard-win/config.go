package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Book struct{ Name, Path string }
type Config struct {
	Path                                         string
	Books                                        []Book
	Editor                                       []string
	DefaultAction, Version, Display, Sort        string
	Reverse, Group, Nicknames, UIDs, Kinds, Skip bool
	Private, PreferredPhone, PreferredEmail      []string
}

// ConfigObj does not use backslash escapes in quoted values. In particular,
// Windows paths must survive parsing unchanged.
func configValue(s string) ([]string, bool, error) {
	var values []string
	var b strings.Builder
	quote := byte(0)
	list := false
	closed := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				closed = true
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if c == '#' {
			break
		}
		if c == ',' {
			values = append(values, strings.TrimSpace(b.String()))
			b.Reset()
			list = true
			closed = false
			continue
		}
		if (c == '\'' || c == '"') && strings.TrimSpace(b.String()) == "" && !closed {
			quote = c
			continue
		}
		if closed && c != ' ' && c != '\t' && c != '\r' {
			return nil, false, fmt.Errorf("text after quoted value")
		}
		b.WriteByte(c)
	}
	if quote != 0 {
		return nil, false, fmt.Errorf("unclosed quote")
	}
	v := strings.TrimSpace(b.String())
	if v != "" || !list {
		values = append(values, v)
	}
	return values, list, nil
}

func expandPath(s string) (string, error) {
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if len(s) == 1 {
			s = home
		} else {
			s = filepath.Join(home, filepath.FromSlash(s[2:]))
		}
	} else if strings.HasPrefix(s, "~") {
		return "", fmt.Errorf("named-user home paths are unsupported: %s; use ~/", s)
	}
	return filepath.Abs(filepath.FromSlash(s))
}

func defaultConfigPath() (string, error) {
	if s := os.Getenv("KHARD_CONFIG"); s != "" {
		return expandPath(s)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = "~/.config"
	}
	return expandPath(filepath.Join(base, "khard", "khard.conf"))
}

func loadConfig(path string) (Config, error) {
	c := Config{DefaultAction: "list", Version: "3.0", Sort: "first_name", Display: "first_name", UIDs: true, PreferredPhone: []string{"pref"}, PreferredEmail: []string{"pref"}}
	var err error
	if path == "" {
		path, err = defaultConfigPath()
	} else {
		path, err = expandPath(path)
	}
	if err != nil {
		return c, err
	}
	c.Path = path
	f, err := os.Open(path)
	if err != nil {
		return c, fmt.Errorf("configuration: %w (copy your Linux khard.conf to this location, or use -c)", err)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 1024*1024)
	section, book := "", -1
	seen := map[string]bool{}
	lineNo := 0
	for scan.Scan() {
		lineNo++
		line := strings.TrimSpace(strings.TrimPrefix(scan.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fail := func(e error) (Config, error) { return c, fmt.Errorf("%s:%d: %w", path, lineNo, e) }
		if strings.HasPrefix(line, "[") {
			header := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			if strings.HasPrefix(header, "[[") && strings.HasSuffix(header, "]]") && section == "addressbooks" {
				name := strings.TrimSpace(header[2 : len(header)-2])
				name = strings.Trim(name, `"'`)
				if name == "" || seen["book:"+name] {
					return fail(fmt.Errorf("empty or duplicate address book %q", name))
				}
				seen["book:"+name] = true
				c.Books = append(c.Books, Book{Name: name})
				book = len(c.Books) - 1
			} else if strings.HasPrefix(header, "[") && strings.HasSuffix(header, "]") && !strings.HasPrefix(header, "[[") {
				section = strings.TrimSpace(header[1 : len(header)-1])
				book = -1
				if seen["section:"+section] {
					return fail(fmt.Errorf("duplicate section %q", section))
				}
				seen["section:"+section] = true
			} else {
				return fail(fmt.Errorf("invalid section header"))
			}
			continue
		}
		k, val, ok := strings.Cut(line, "=")
		if !ok {
			return fail(fmt.Errorf("expected key = value"))
		}
		k = strings.TrimSpace(k)
		id := section + ":" + strconv.Itoa(book) + ":" + k
		if seen[id] {
			return fail(fmt.Errorf("duplicate key %s", k))
		}
		seen[id] = true
		values, list, e := configValue(strings.TrimSpace(val))
		if e != nil {
			return fail(e)
		}
		s := ""
		if len(values) > 0 {
			s = values[0]
		}
		boolean := func(dst *bool) error {
			switch strings.ToLower(s) {
			case "yes", "true", "on", "1":
				*dst = true
			case "no", "false", "off", "0":
				*dst = false
			default:
				return fmt.Errorf("%s must be a boolean", k)
			}
			return nil
		}
		switch section {
		case "addressbooks":
			if book < 0 || k != "path" || list {
				return fail(fmt.Errorf("expected [[book name]] and a single path"))
			}
			c.Books[book].Path, e = expandPath(s)
		case "general":
			switch k {
			case "default_action":
				if s != "" {
					c.DefaultAction = s
				}
			case "editor":
				if list {
					c.Editor = values
				} else if s != "" {
					c.Editor, e = splitCommand(s)
				}
			case "debug":
				var b bool
				e = boolean(&b)
			case "merge_editor": /* accepted for config interchange; merge is not in this preview */
			}
		case "contact table":
			switch k {
			case "display":
				c.Display = s
			case "sort":
				c.Sort = s
			case "reverse":
				e = boolean(&c.Reverse)
			case "group_by_addressbook":
				e = boolean(&c.Group)
			case "show_nicknames":
				e = boolean(&c.Nicknames)
			case "show_uids":
				e = boolean(&c.UIDs)
			case "show_kinds":
				e = boolean(&c.Kinds)
			case "localize_dates":
				var b bool
				e = boolean(&b)
			case "preferred_phone_number_type":
				c.PreferredPhone = values
			case "preferred_email_address_type":
				c.PreferredEmail = values
			}
		case "vcard":
			switch k {
			case "preferred_version":
				c.Version = s
			case "private_objects":
				c.Private = values
			case "skip_unparsable":
				e = boolean(&c.Skip)
			case "search_in_source_files":
				var b bool
				e = boolean(&b)
			}
		}
		if e != nil {
			return fail(e)
		}
	}
	if err = scan.Err(); err != nil {
		return c, err
	}
	if len(c.Books) == 0 {
		return c, fmt.Errorf("%s: no address books configured", path)
	}
	for _, b := range c.Books {
		if b.Path == "" {
			return c, fmt.Errorf("address book %q needs a path", b.Name)
		}
	}
	if c.Version != "3.0" && c.Version != "4.0" {
		return c, fmt.Errorf("preferred_version must be 3.0 or 4.0")
	}
	for _, v := range []string{c.Sort, c.Display} {
		if v != "first_name" && v != "last_name" && v != "formatted_name" {
			return c, fmt.Errorf("invalid name order %q", v)
		}
	}
	for _, v := range c.Private {
		if v != "" && !validToken(v) {
			return c, fmt.Errorf("invalid private object %q", v)
		}
	}
	return c, nil
}

// Shell-like command syntax is only tokenized: no shell is invoked. Comma
// lists are preferable for commands shared between Windows and Unix.
func splitCommand(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	var quote rune
	active := false
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		v := r[i]
		if quote != 0 {
			if v == quote {
				quote = 0
			} else {
				b.WriteRune(v)
			}
			active = true
			continue
		}
		if v == '\'' || v == '"' {
			quote = v
			active = true
			continue
		}
		if v == ' ' || v == '\t' {
			if active {
				out = append(out, b.String())
				b.Reset()
				active = false
			}
			continue
		}
		b.WriteRune(v)
		active = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed editor command quote")
	}
	if active {
		out = append(out, b.String())
	}
	if len(out) == 0 || out[0] == "" {
		return nil, fmt.Errorf("empty editor command")
	}
	return out, nil
}
