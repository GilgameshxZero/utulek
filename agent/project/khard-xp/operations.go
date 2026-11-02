package main

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func (r Runner) answer(prompt string) (string, error) {
	if !r.Interactive {
		return "", fmt.Errorf("%s requires interactive confirmation", prompt)
	}
	fmt.Fprint(r.Out, prompt+": ")
	s, e := r.In.ReadString('\n')
	if e != nil && s == "" {
		return "", e
	}
	return strings.TrimSpace(s), nil
}
func (r Runner) confirm(prompt string) error {
	s, e := r.answer(prompt + " [y/N]")
	if e != nil {
		return e
	}
	if !strings.EqualFold(s, "y") && !strings.EqualFold(s, "yes") {
		return fmt.Errorf("cancelled")
	}
	return nil
}
func selectedBooks(cfg Config, names []string) ([]Book, error) {
	if len(names) == 0 {
		return cfg.Books, nil
	}
	out := make([]Book, 0, len(names))
	for _, n := range names {
		found := false
		for _, b := range cfg.Books {
			if b.Name == n {
				out = append(out, b)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown address book %q", n)
		}
	}
	return out, nil
}
func (r Runner) chooseBook(books []Book) (Book, error) {
	if len(books) == 1 {
		return books[0], nil
	}
	if len(books) == 0 {
		return Book{}, fmt.Errorf("no available address books")
	}
	for i, b := range books {
		fmt.Fprintf(r.Out, "%d  %s\n", i+1, safe(b.Name))
	}
	s, e := r.answer("Select address book number")
	if e != nil {
		return Book{}, e
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < 1 || n > len(books) {
		return Book{}, fmt.Errorf("invalid address book selection")
	}
	return books[n-1], nil
}
func cloneCard(c *Card) *Card {
	out := *c
	out.Props = append([]Property(nil), c.Props...)
	return &out
}

// Keep the source locked until installation and removal finish. Never clobber
// an existing destination; retain both copies if source removal fails.
func transferCard(source *Card, book Book, move bool) error {
	return withCardLock(source, func() error {
		c := cloneCard(source)
		if e := installCard(c, book, move, func() error { return sameOriginal(source) }); e != nil {
			return e
		}
		if move {
			e := sameOriginal(source)
			if e == nil {
				e = os.Remove(source.Path)
			}
			if e != nil {
				return fmt.Errorf("destination saved at %s; source retained: %w", c.Path, e)
			}
		}
		return nil
	})
}

func (r Runner) contactOperation(source *Card, cfg Config, o Options) error {
	books, e := selectedBooks(cfg, o.TargetBooks)
	if e != nil {
		return e
	}
	if o.Command == "merge" {
		var terms []string
		if o.Target != "" {
			terms = []string{o.Target}
		}
		names := make([]string, len(books))
		for i, b := range books {
			names[i] = b.Name
		}
		targets, e := loadMatchingCards(cfg, names, terms)
		if e != nil {
			return e
		}
		target, e := r.choose(targets, cfg)
		if e != nil {
			return e
		}
		return r.mergeContacts(source, target, cfg, true)
	}
	available := make([]Book, 0, len(books))
	for _, b := range books {
		if b.Name != source.Book && !sameFilePath(b.Path, filepath.Dir(source.Path)) {
			available = append(available, b)
		}
	}
	book, e := r.chooseBook(available)
	if e != nil {
		return e
	}
	targets, e := loadMatchingCards(cfg, []string{book.Name}, nil)
	if e != nil {
		return e
	}
	var duplicates []*Card
	for _, c := range targets {
		if c.matches([]string{source.first("FN")}, cfg) {
			duplicates = append(duplicates, c)
		}
	}
	if len(duplicates) > 0 {
		if !r.Interactive {
			return fmt.Errorf("destination contains a contact with the same name; interactive selection required")
		}
		fmt.Fprintln(r.Out, "Select a matching destination contact, or 0 to add a new entry.")
		for i, c := range duplicates {
			fmt.Fprintf(r.Out, "%d  %s\n", i+1, safe(c.name(cfg.Display)))
		}
		s, e := r.answer("Target number")
		if e != nil {
			return e
		}
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 || n > len(duplicates) {
			return fmt.Errorf("invalid selection")
		}
		if n > 0 {
			target := duplicates[n-1]
			if reflect.DeepEqual(source.model(cfg), target.model(cfg)) {
				if o.Command == "move" {
					return transferCard(source, book, true)
				}
				fmt.Fprintln(r.Out, "The selected contacts are already identical.")
				return nil
			}
			action, e := r.answer("Action (" + o.Command + ", merge, overwrite, quit)")
			if e != nil {
				return e
			}
			switch action {
			case "merge":
				return r.mergeContacts(source, target, cfg, o.Command == "move")
			case "overwrite":
				return withCardLock(target, func() error {
					if e := transferCard(source, book, o.Command == "move"); e != nil {
						return e
					}
					if e := sameOriginal(target); e != nil {
						return fmt.Errorf("transfer saved; old destination retained: %w", e)
					}
					if e := os.Remove(target.Path); e != nil {
						return fmt.Errorf("transfer saved; old destination retained: %w", e)
					}
					return nil
				})
			case "copy", "move":
				if action != o.Command {
					return fmt.Errorf("invalid action")
				}
			default:
				return fmt.Errorf("cancelled")
			}
		}
	}
	if e = transferCard(source, book, o.Command == "move"); e != nil {
		return e
	}
	fmt.Fprintf(r.Out, "Contact %s completed [%s].\n", o.Command, safe(book.Name))
	return nil
}

func (r Runner) mergeContacts(source, target *Card, cfg Config, deleteSource bool) (resultErr error) {
	if sameFilePath(source.Path, target.Path) {
		fmt.Fprintln(r.Out, "The selected contacts are already identical.")
		return nil
	}
	if !r.Interactive {
		return fmt.Errorf("merge requires interactive confirmation")
	}
	src, e := encodeModel(source.model(cfg))
	if e != nil {
		return e
	}
	dst, e := encodeModel(target.model(cfg))
	if e != nil {
		return e
	}
	var data []byte
	var recovery string
	defer func() {
		if recovery != "" {
			if resultErr != nil {
				fmt.Fprintf(r.Err, "Your merge input is preserved at %s\n", recovery)
			} else {
				os.Remove(recovery)
			}
		}
	}()
	if len(cfg.MergeEditor) > 0 {
		f, e := os.CreateTemp("", "khard-xp-merge-source-*.yaml")
		if e != nil {
			return e
		}
		srcPath := f.Name()
		defer os.Remove(srcPath)
		if _, e = f.Write(src); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		f, e = os.CreateTemp("", "khard-xp-merge-target-*.yaml")
		if e != nil {
			return e
		}
		recovery = f.Name()
		if _, e = f.Write(dst); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		args := append(append([]string(nil), cfg.MergeEditor[1:]...), srcPath, recovery)
		cmd := exec.Command(cfg.MergeEditor[0], args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = r.Out
		cmd.Stderr = r.Err
		e = cmd.Run()
		if e == nil {
			data, e = os.ReadFile(recovery)
		}
	} else {
		reference := "# Source contact (reference):\n# " + strings.ReplaceAll(string(src), "\n", "\n# ") + "\n# Edit the target below.\n"
		data, recovery, e = r.editor(append([]byte(reference), dst...), cfg, "yaml")
	}
	if e != nil {
		return e
	}
	if len(data) > 16*1024*1024 {
		return fmt.Errorf("merge input exceeds 16 MiB")
	}
	model, e := decodeModel(data)
	if e != nil {
		return e
	}
	merged := cloneCard(target)
	changed, e := merged.applyModel(model, cfg)
	if e != nil {
		return e
	}
	if !changed {
		fmt.Fprintln(r.Out, "Target contact unmodified.")
		return nil
	}
	fmt.Fprint(r.Out, string(pretty(merged, cfg)))
	completion := "Save merged target and keep source contact"
	if deleteSource {
		completion = "Save merged target and remove source contact"
	}
	if e = r.confirm(completion); e != nil {
		return e
	}
	return withCardLock(source, func() error {
		if e := saveCard(merged); e != nil {
			return e
		}
		if deleteSource {
			if e := sameOriginal(source); e != nil {
				return fmt.Errorf("merged target saved; source retained: %w", e)
			}
			if e := os.Remove(source.Path); e != nil {
				return fmt.Errorf("merged target saved; source retained: %w", e)
			}
		}
		return nil
	})
}

func readMailAddresses(data []byte, headers string) ([]*mail.Address, error) {
	msg, e := mail.ReadMessage(io.MultiReader(bytes.NewReader(data), strings.NewReader("\r\n\r\n")))
	if e != nil {
		return nil, e
	}
	fields := strings.Split(headers, ",")
	if headers == "" {
		fields = []string{"from"}
	}
	if strings.EqualFold(headers, "all") {
		fields = fields[:0]
		for k := range msg.Header {
			fields = append(fields, k)
		}
		sort.Strings(fields)
	}
	var out []*mail.Address
	seen := map[string]bool{}
	for _, field := range fields {
		for _, v := range msg.Header[textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(field))] {
			addresses, e := mail.ParseAddressList(v)
			if e != nil {
				if strings.EqualFold(headers, "all") {
					continue
				}
				return nil, fmt.Errorf("invalid %s header: %w", field, e)
			}
			for _, a := range addresses {
				if !seen[a.Address] {
					seen[a.Address] = true
					out = append(out, a)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no email addresses found in requested headers")
	}
	return out, nil
}
func (r Runner) addEmail(cfg Config, o Options) error {
	var data []byte
	var e error
	if o.Input != "" && o.Input != "-" {
		p, e := expandPath(o.Input)
		if e != nil {
			return e
		}
		data, e = os.ReadFile(p)
		if e != nil {
			return e
		}
	} else {
		data, e = io.ReadAll(io.LimitReader(r.In, 16*1024*1024+1))
		if e != nil {
			return e
		}
	}
	if len(data) > 16*1024*1024 {
		return fmt.Errorf("mail input exceeds 16 MiB")
	}
	addresses, e := readMailAddresses(data, o.Headers)
	if e != nil {
		return e
	}
	cards, e := loadMatchingCards(cfg, o.Books, nil)
	if e != nil {
		return e
	}
	known := map[string]bool{}
	for _, c := range cards {
		for _, p := range c.properties("EMAIL") {
			known[unescape(p.Value)] = true
		}
	}
	for _, a := range addresses {
		if known[a.Address] && o.SkipAdded {
			fmt.Fprintf(r.Out, "Skipping already added email address %s\n", safe(a.Address))
			continue
		}
		if e = r.confirm("Select address " + safe(a.String())); e != nil {
			return e
		}
		if a.Name == "" {
			a.Name, e = r.answer("Contact name")
			if e != nil {
				return e
			}
		}
		terms := o.Terms
		if len(terms) == 0 {
			terms = strings.Fields(strings.ReplaceAll(a.Name, ",", ""))
		}
		var matches []*Card
		for _, c := range cards {
			if c.matches(terms, cfg) {
				matches = append(matches, c)
			}
		}
		var c *Card
		if len(matches) > 0 {
			c, e = r.choose(matches, cfg)
			if e != nil {
				return e
			}
		}
		action, e := r.answer("Use selected contact (yes), create, search, or quit")
		if e != nil {
			return e
		}
		for action == "search" {
			query, e := r.answer("Search contact")
			if e != nil {
				return e
			}
			matches = nil
			for _, v := range cards {
				if v.matches(strings.Fields(query), cfg) {
					matches = append(matches, v)
				}
			}
			c, e = r.choose(matches, cfg)
			if e != nil {
				return e
			}
			action, e = r.answer("Use selected contact (yes), create, or quit")
			if e != nil {
				return e
			}
		}
		creating := action == "create"
		var book Book
		if creating {
			book, e = r.chooseBook(cfg.Books)
			if e != nil {
				return e
			}
			c, e = r.newEmailContact(a.Name, cfg)
			if e != nil {
				return e
			}
		} else if action != "yes" || c == nil {
			return fmt.Errorf("cancelled")
		}
		duplicate := false
		for _, p := range c.properties("EMAIL") {
			if unescape(p.Value) == a.Address {
				duplicate = true
			}
		}
		if duplicate {
			fmt.Fprintln(r.Out, "Contact already contains this address.")
			continue
		}
		updated := cloneCard(c)
		for {
			label, e := r.answer("Email label [internet]")
			if e != nil {
				return e
			}
			if label == "" {
				label = "internet"
			}
			if e = updated.addEmailAddress(label, a.Address); e != nil {
				fmt.Fprintln(r.Out, e)
				continue
			}
			break
		}
		if creating {
			e = createCard(updated, book)
		} else {
			e = saveCard(updated)
		}
		if e != nil {
			return e
		}
		if creating {
			cards = append(cards, updated)
		} else {
			for i, v := range cards {
				if v == c {
					cards[i] = updated
					break
				}
			}
		}
		known[a.Address] = true
		fmt.Fprintln(r.Out, "Email address added.")
	}
	return nil
}

// Detect aliases and case differences using filesystem identity on each platform.
func sameFilePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	return ae == nil && be == nil && os.SameFile(ai, bi)
}
func (r Runner) defaultAnswer(prompt, value string) (string, error) {
	s, e := r.answer(prompt + " [" + value + "] (- clears)")
	if e != nil {
		return "", e
	}
	if s == "-" {
		return "", nil
	}
	if s == "" {
		return value, nil
	}
	return s, nil
}
func (r Runner) newEmailContact(name string, cfg Config) (*Card, error) {
	parts := strings.Fields(strings.NewReplacer("\"", "", "'", "").Replace(name))
	if len(parts) > 1 && strings.HasSuffix(parts[0], ",") {
		parts = append(parts[1:], strings.TrimSuffix(parts[0], ","))
	}
	first, last := "", ""
	if len(parts) > 0 {
		first = parts[0]
	}
	if len(parts) > 1 {
		last = parts[len(parts)-1]
	}
	first, e := r.defaultAnswer("First name", first)
	if e != nil {
		return nil, e
	}
	last, e = r.defaultAnswer("Last name", last)
	if e != nil {
		return nil, e
	}
	orgDefault := ""
	if first == "" && last == "" {
		orgDefault = name
	}
	org, e := r.defaultAnswer("Organisation", orgDefault)
	if e != nil {
		return nil, e
	}
	c := &Card{Props: []Property{newProp("BEGIN", "VCARD"), newProp("VERSION", cfg.Version), newProp("END", "VCARD")}}
	if _, e = c.applyModel(Model{"First name": first, "Last name": last, "Organisation": org}, cfg); e != nil {
		return nil, e
	}
	fmt.Fprint(r.Out, string(pretty(c, cfg)))
	if e = r.confirm("Create contact"); e != nil {
		return nil, e
	}
	return c, nil
}

// Add one property without rebuilding the contact model or changing old labels.
func (c *Card) addEmailAddress(label, address string) error {
	var types []string
	custom := ""
	pref := 0
	for _, part := range strings.Split(label, ",") {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		switch {
		case lower == "":
			continue
		case lower == "pref":
			pref++
		case strings.HasPrefix(lower, "pref="):
			n, e := strconv.Atoi(lower[5:])
			if e != nil || n < 0 || n > 99 {
				return fmt.Errorf("invalid email preference")
			}
			pref += n
		case lower == "home" || lower == "internet" || lower == "work" || (lower == "x400" && c.first("VERSION") == "3.0"):
			types = append(types, part)
		default:
			if !validToken(part) {
				return fmt.Errorf("invalid email label")
			}
			if custom != "" {
				return fmt.Errorf("only one custom email label is allowed")
			}
			if strings.HasPrefix(lower, "x-") {
				custom = part[2:]
				types = append(types, part)
			} else {
				custom = part
				types = append(types, "X-"+part)
			}
		}
	}
	if len(types) == 0 && pref == 0 {
		return fmt.Errorf("email label is missing")
	}
	head := "EMAIL"
	if pref > 0 {
		if c.first("VERSION") == "4.0" {
			head += ";PREF=" + strconv.Itoa(pref)
		} else {
			types = append(types, "pref")
		}
	}
	if len(types) > 0 {
		head += ";TYPE=" + strings.Join(types, ",")
	}
	group := ""
	if custom != "" {
		used := make(map[string]bool, len(c.Props))
		for _, p := range c.Props {
			used[strings.ToLower(p.Group)] = true
		}
		for i := 1; ; i++ {
			group = "itememail" + strconv.Itoa(i)
			if !used[group] {
				break
			}
		}
		head = group + "." + head
	}
	c.replace("EMAIL", append(c.properties("EMAIL"), newProp(head, escape(address))))
	if custom != "" {
		c.replace("X-ABLABEL", append(c.properties("X-ABLABEL"), newProp(group+".X-ABLabel", escape(custom))))
	}
	return nil
}
