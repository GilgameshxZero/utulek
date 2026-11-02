package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

const version = "0.1.4-preview"

var errNoResults = fmt.Errorf("found no contacts")

type Options struct {
	SourceSearch, Debug                                                    bool
	Command, Config, Input, Output, Format, Display, Sort, Fields, Version string
	Books, Terms, ListFields                                               []string
	Help, ShowVersion, Parsable, Force, OpenEditor, Reverse, Group, Skip   bool
	Mutt                                                                   bool
	TargetBooks                                                            []string
	Target, Headers                                                        string
	SkipAdded                                                              bool
}

var aliases = map[string]string{"list": "list", "ls": "list", "show": "show", "details": "show", "template": "template", "phone": "phone", "email": "email", "new": "new", "add": "new", "edit": "edit", "modify": "edit", "ed": "edit", "remove": "remove", "delete": "remove", "del": "remove", "rm": "remove", "addressbooks": "addressbooks", "abooks": "addressbooks", "filename": "filename", "file": "filename", "birthdays": "birthdays", "bdays": "birthdays", "postaddress": "postaddress", "post": "postaddress", "postaddr": "postaddress", "merge": "merge", "copy": "copy", "cp": "copy", "move": "move", "mv": "move", "add-email": "add-email"}

func parseArgs(args []string) (Options, error) {
	o := Options{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			o.Terms = append(o.Terms, args[i+1:]...)
			break
		}
		key, value, has := strings.Cut(a, "=")
		next := func() (string, error) {
			if has {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s needs a value", key)
			}
			return args[i], nil
		}
		var dst *string
		switch key {
		case "-h", "--help":
			o.Help = true
		case "-v", "--version":
			o.ShowVersion = true
		case "-c", "--config":
			dst = &o.Config
		case "-t", "--target-contact", "--target":
			dst = &o.Target
		case "-H", "--headers":
			dst = &o.Headers
		case "--skip-already-added":
			o.SkipAdded = true
		case "-A", "--target-addressbook":
			s, e := next()
			if e != nil {
				return o, e
			}
			for _, n := range strings.Split(s, ",") {
				o.TargetBooks = append(o.TargetBooks, strings.TrimSpace(n))
			}
		case "-i", "--input-file":
			dst = &o.Input
		case "-o", "--output-file":
			dst = &o.Output
		case "--format":
			dst = &o.Format
		case "-d", "--display":
			dst = &o.Display
		case "-s", "--sort":
			dst = &o.Sort
		case "-F", "--fields":
			dst = &o.Fields
		case "--vcard-version":
			dst = &o.Version
		case "-a", "--addressbook":
			s, e := next()
			if e != nil {
				return o, e
			}
			for _, n := range strings.Split(s, ",") {
				o.Books = append(o.Books, strings.TrimSpace(n))
			}
		case "-p", "--parsable":
			o.Parsable = true
		case "--mutt":
			o.Mutt = true
			if has {
				return o, fmt.Errorf("--mutt does not take a value")
			}
		case "--force":
			o.Force = true
		case "--open-editor", "--edit":
			o.OpenEditor = true
		case "-r", "--reverse":
			o.Reverse = true
		case "-g", "--group-by-addressbook":
			o.Group = true
		case "--skip-unparsable":
			o.Skip = true
		case "-f", "--search-in-source-files":
			o.SourceSearch = true
		case "--debug":
			o.Debug = true
		default:
			if strings.HasPrefix(a, "-") {
				return o, fmt.Errorf("unknown option %s", a)
			}
			if cmd, ok := aliases[a]; ok && o.Command == "" && len(o.Terms) == 0 {
				o.Command = cmd
			} else {
				o.Terms = append(o.Terms, a)
			}
		}
		if dst != nil {
			s, e := next()
			if e != nil {
				return o, e
			}
			*dst = s
		}
	}
	return o, nil
}

const help = `khard-xp — native Linux and Windows address book, compatible with khard configuration

Usage: khard [-c CONFIG] COMMAND [OPTIONS] [SEARCH TERMS...]

Commands:
  list (ls)          List contacts (default)
  show (details)     Show one contact; select interactively if needed
  phone / email      List phone numbers / email addresses
  postaddress        List postal addresses
  birthdays          List birthdays by month/day
  filename           Print matching contact filenames
  addressbooks       List configured address books
  template           Print a YAML contact template
  new (add)          Create a contact using YAML input or your editor
  edit (modify, ed)  Edit one contact using YAML (or --format vcard)
  remove (rm)        Remove one contact after confirmation
  copy / move        Copy or move a contact to another address book
  merge              Merge source into target using the configured merge editor
  add-email          Extract email headers and add addresses interactively

Options:
  -A, --target-addressbook NAMES  Destination books for copy/move/merge
  -t, --target-contact QUERY      Merge target query
  -H, --headers HEADERS           add-email headers (default from; or all)
  --skip-already-added            Skip known addresses in add-email
  -c, --config FILE       ConfigObj khard.conf (same format as Linux khard)
  -a, --addressbook NAMES Select comma-separated address books
  -p, --parsable          Tab-separated output for list, phone, email
  --mutt                 email only: NeoMutt status line followed by TSV rows
  -i, --input-file FILE   Contact input or add-email message; '-' means stdin
  -o, --output-file FILE  Write show output to a new file
  --format FORMAT        show: pretty/yaml/vcard; edit/new: yaml/vcard
  --open-editor          Open input in editor before saving
  --force                Remove the selected contact without confirmation
  --vcard-version 3.0|4.0 Version for new contacts
  -d, --display ORDER    first_name / last_name / formatted_name
  -s, --sort ORDER       Same orders as display
  -r, --reverse          Reverse name order
  -g, --group-by-addressbook Group the contact list
  -F, --fields FIELDS    list columns, including nested paths; -F help lists fields
  -f, --search-in-source-files Prefilter raw vCards before parsing (less accurate)
  --debug                Write loading diagnostics to stderr
  --skip-unparsable      Skip malformed contact files with a warning
  -v, --version         Print version

Search terms are case-insensitive and ANDed; name:foo, emails:foo,
phone_numbers:123 and other YAML field names restrict the search.
With email --mutt, whitespace in each query argument separates ANDed terms;
quotes, apostrophes and shell metacharacters are literal query characters.
Config: KHARD_CONFIG, else XDG_CONFIG_HOME/khard/khard.conf,
else ~/.config/khard/khard.conf on both Windows and Linux.

Preview limitations:
Name sorting is Unicode lexical, not locale collation. Dates remain ISO.
`

type Runner struct {
	In          *bufio.Reader
	Out, Err    io.Writer
	Interactive bool
}

func (r Runner) choose(cards []*Card, cfg Config) (*Card, error) {
	if len(cards) == 0 {
		return nil, fmt.Errorf("no matching contacts")
	}
	if len(cards) == 1 {
		return cards[0], nil
	}
	if !r.Interactive {
		return nil, fmt.Errorf("%d contacts match; narrow the search or use uid:<UID>", len(cards))
	}
	for i, c := range cards {
		fmt.Fprintf(r.Out, "%d  %s  [%s]\n", i+1, safe(c.name(cfg.Display)), safe(c.Book))
	}
	fmt.Fprint(r.Out, "Select contact number (blank cancels): ")
	s, e := r.In.ReadString('\n')
	if e != nil && s == "" {
		return nil, e
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("cancelled")
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < 1 || n > len(cards) {
		return nil, fmt.Errorf("invalid selection")
	}
	return cards[n-1], nil
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, s)
}

// Detect the requested output protocol even when an earlier argument is invalid.
// Values of options and everything after -- are data, never protocol flags.
func muttRequested(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		key, _, inline := strings.Cut(args[i], "=")
		if key == "--mutt" {
			return true
		}
		if !inline {
			switch key {
			case "-c", "--config", "-i", "--input-file", "-o", "--output-file", "--format", "-d", "--display", "-s", "--sort", "-F", "--fields", "--vcard-version", "-a", "--addressbook", "-A", "--target-addressbook", "-t", "--target-contact", "--target", "-H", "--headers":
				i++
			}
		}
	}
	return false
}

func (r Runner) run(args []string) error {
	o, e := parseArgs(args)
	if !o.Mutt && !muttRequested(args) {
		if e != nil {
			return e
		}
		return r.runOptions(o)
	}
	// NeoMutt consumes the first stdout line as status, including on failure.
	// Buffer results so an error cannot leave a partial successful query behind.
	var rows bytes.Buffer
	query := r
	query.Out = &rows
	if e == nil {
		e = query.runOptions(o)
	}
	if e != nil {
		fmt.Fprintf(r.Out, "khard: %s\n", safe(e.Error()))
		return e // main prints the full diagnostic to stderr and exits nonzero
	}
	_, e = r.Out.Write(append([]byte{'\n'}, rows.Bytes()...))
	return e
}

func (r Runner) runOptions(o Options) error {
	if o.Mutt && o.Command != "email" {
		return fmt.Errorf("--mutt requires the email command")
	}
	var e error
	if o.Help {
		_, e = fmt.Fprint(r.Out, help)
		return e
	}
	if o.ShowVersion {
		_, e = fmt.Fprintf(r.Out, "khard-xp %s\n", version)
		return e
	}
	cfg, e := loadConfig(o.Config)
	if e != nil {
		return e
	}
	cfg.SourceSearch = cfg.SourceSearch || o.SourceSearch
	cfg.Debug = cfg.Debug || o.Debug
	cfg.Diagnostics = r.Err
	cfg.debugf("command=%s config=%s source_search=%t", o.Command, cfg.Path, cfg.SourceSearch)
	if o.Command == "" {
		cmd, ok := aliases[cfg.DefaultAction]
		if !ok {
			return fmt.Errorf("unsupported default_action %q", cfg.DefaultAction)
		}
		o.Command = cmd
	}
	if o.Mutt {
		var terms []string
		for _, arg := range o.Terms {
			terms = append(terms, strings.Fields(arg)...)
		}
		o.Terms = terms
	}
	if o.Display != "" {
		cfg.Display = o.Display
	}
	if o.Sort != "" {
		cfg.Sort = o.Sort
	}
	if o.Reverse {
		cfg.Reverse = true
	}
	if o.Group {
		cfg.Group = true
	}
	if o.Skip {
		cfg.Skip = true
	}
	for _, s := range []string{cfg.Display, cfg.Sort} {
		if s != "first_name" && s != "last_name" && s != "formatted_name" {
			return fmt.Errorf("invalid name order %q", s)
		}
	}
	if o.Version != "" {
		if o.Command != "new" && o.Command != "add-email" {
			return fmt.Errorf("--vcard-version applies only to new/add-email")
		}
		if o.Version != "3.0" && o.Version != "4.0" {
			return fmt.Errorf("vCard version must be 3.0 or 4.0")
		}
		cfg.Version = o.Version
	}
	if o.Target != "" && o.Command != "merge" {
		return fmt.Errorf("--target-contact requires merge")
	}
	if (len(o.TargetBooks) > 0 || o.Target != "") && o.Command != "copy" && o.Command != "move" && o.Command != "merge" {
		return fmt.Errorf("target options require copy, move or merge")
	}
	if (o.Headers != "" || o.SkipAdded) && o.Command != "add-email" {
		return fmt.Errorf("email header options require add-email")
	}
	if o.Force && o.Command != "remove" {
		return fmt.Errorf("--force applies only to remove")
	}
	if o.Input != "" && o.Command != "new" && o.Command != "edit" && o.Command != "add-email" {
		return fmt.Errorf("-i applies only to new/edit/add-email")
	}
	if o.Output != "" && o.Command != "show" {
		return fmt.Errorf("-o applies only to show")
	}
	if o.Format != "" {
		allowed := []string{"yaml", "vcard"}
		if o.Command == "show" {
			allowed = append(allowed, "pretty")
		}
		if !contains(allowed, o.Format) || (o.Command != "show" && o.Command != "new" && o.Command != "edit") {
			return fmt.Errorf("invalid --format for %s", o.Command)
		}
	}
	if o.Command == "list" && o.Fields != "" {
		if strings.EqualFold(o.Fields, "help") {
			_, e = fmt.Fprintln(r.Out, listFieldsHelp())
			return e
		}
		o.ListFields, e = parseListFields(o.Fields)
		if e != nil {
			return e
		}
	}

	if o.Command == "template" {
		m := (&Card{}).model(cfg)
		m["Phone"] = map[string]any{"": nil}
		m["Email"] = map[string]any{"": nil}
		data, e := encodeModel(m)
		if e != nil {
			return e
		}
		_, e = r.Out.Write(data)
		return e
	}
	if o.Command == "addressbooks" {
		for _, b := range cfg.Books {
			fmt.Fprintf(r.Out, "%s\t%s\n", safe(b.Name), safe(b.Path))
		}
		return nil
	}
	if o.Command == "new" {
		return r.writeContact(nil, cfg, o)
	}
	if o.Command == "add-email" {
		return r.addEmail(cfg, o)
	}
	matches, e := loadMatchingCards(cfg, o.Books, o.Terms)
	if e != nil {
		return e
	}
	switch o.Command {
	case "list", "phone", "email", "postaddress", "birthdays", "filename":
		return r.list(matches, cfg, o)
	case "copy", "move", "merge":
		c, e := r.choose(matches, cfg)
		if e != nil {
			return e
		}
		return r.contactOperation(c, cfg, o)
	case "show", "edit", "remove":
		c, e := r.choose(matches, cfg)
		if e != nil {
			return e
		}
		if o.Command == "edit" {
			return r.writeContact(c, cfg, o)
		}
		if o.Command == "remove" {
			if !o.Force {
				if !r.Interactive {
					return fmt.Errorf("removal requires interactive confirmation or --force")
				}
				fmt.Fprintf(r.Out, "Remove %s [%s]? [y/N] ", safe(c.name(cfg.Display)), safe(c.Book))
				s, _ := r.In.ReadString('\n')
				if !strings.EqualFold(strings.TrimSpace(s), "y") && !strings.EqualFold(strings.TrimSpace(s), "yes") {
					return fmt.Errorf("cancelled")
				}
			}
			if e = removeCard(c); e != nil {
				return e
			}
			fmt.Fprintln(r.Out, "Contact removed.")
			return nil
		}
		var data []byte
		switch o.Format {
		case "vcard":
			data = c.Original
		case "yaml":
			data, e = encodeModel(c.model(cfg))
		default:
			data = pretty(c, cfg)
		}
		if e != nil {
			return e
		}
		if o.Output != "" && o.Output != "-" {
			p, e := expandPath(o.Output)
			if e != nil {
				return e
			}
			f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return e
			}
			_, e = f.Write(data)
			ce := f.Close()
			if e != nil {
				return e
			}
			return ce
		}
		_, e = r.Out.Write(data)
		return e
	}
	return fmt.Errorf("unknown command %q", o.Command)
}

func pretty(c *Card, cfg Config) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nAddress book: %s\nUID: %s\n", safe(c.name(cfg.Display)), safe(c.Book), safe(c.first("UID")))
	m := c.model(cfg)
	for _, k := range fieldOrder {
		if k == "Formatted name" {
			continue
		}
		if ss := stringsValue(m[k]); len(ss) > 0 {
			fmt.Fprintf(&b, "%s: %s\n", k, safe(strings.Join(ss, " | ")))
		}
	}
	return []byte(b.String())
}

func preferred(c *Card, key string, prefs []string) string {
	pp := c.properties(key)
	sort.SliceStable(pp, func(i, j int) bool { return strings.ToLower(c.label(pp[i])) < strings.ToLower(c.label(pp[j])) })
	for _, pref := range prefs {
		if pref == "" {
			continue
		}
		for _, p := range pp {
			if strings.Contains(strings.ToLower(c.label(p)), strings.ToLower(pref)) {
				return unescape(p.Value)
			}
		}
	}
	if len(pp) > 0 {
		return unescape(pp[0].Value)
	}
	return ""
}

func (r Runner) list(cards []*Card, cfg Config, o Options) error {
	if o.Command == "filename" {
		for _, c := range cards {
			fmt.Fprintln(r.Out, c.Path)
		}
		return nil
	}
	if o.Command == "birthdays" {
		sort.SliceStable(cards, func(i, j int) bool { return birthdayKey(cards[i].first("BDAY")) < birthdayKey(cards[j].first("BDAY")) })
	}
	w := tabwriter.NewWriter(r.Out, 0, 4, 2, ' ', 0)
	if o.Command == "list" {
		fields := []string{"name", "phone", "email"}
		if cfg.UIDs {
			fields = append(fields, "uid")
		}
		if cfg.Kinds {
			fields = append(fields, "kind")
		}
		if cfg.Group {
			fields = append(fields, "addressbook")
		}

		if o.Parsable {
			fields = []string{"uid", "name", "address_book"}
		} else {
			fields = append([]string{"index"}, fields...)
		}
		if len(o.ListFields) > 0 {
			fields = o.ListFields
		}
		if len(cards) == 0 {
			if !o.Parsable {
				fmt.Fprintln(r.Out, "Found no contacts")
			}
			return errNoResults
		}
		output := io.Writer(w)
		if o.Parsable {
			output = r.Out
		} else {
			fmt.Fprintln(w, strings.Join(fields, "\t"))
		}
		for i, c := range cards {
			view := columnValues{card: c}
			row := make([]string, len(fields))
			for j, field := range fields {
				var value string
				switch field {
				case "index":
					value = strconv.Itoa(i + 1)
				case "name":
					value = c.name(cfg.Display)
					if cfg.Nicknames && !o.Parsable && c.first("NICKNAME") != "" {
						value += " (Nickname: " + c.first("NICKNAME") + ")"
					}
				case "phone":
					value = preferred(c, "TEL", cfg.PreferredPhone)
				case "email":
					value = preferred(c, "EMAIL", cfg.PreferredEmail)
				case "address_book", "addressbook":
					value = c.Book
				default:
					value = view.nested(field)
				}
				row[j] = safe(value)
			}
			fmt.Fprintln(output, strings.Join(row, "\t"))
		}
		return w.Flush()
	}
	key := map[string]string{"phone": "TEL", "email": "EMAIL", "postaddress": "ADR", "birthdays": "BDAY"}[o.Command]
	if !o.Parsable && !o.Mutt {
		fmt.Fprintln(w, "Name\tType\tValue")
	}
	for _, c := range cards {
		for _, p := range c.properties(key) {
			v := unescape(p.Value)
			if key == "TEL" && strings.HasPrefix(strings.ToLower(v), "tel:") {
				v = v[4:]
			}
			name := c.name(cfg.Display)
			label := c.label(p)
			if o.Parsable || o.Mutt {
				fmt.Fprintf(r.Out, "%s\t%s\t%s\n", safe(v), safe(name), safe(label))
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\n", safe(name), safe(label), safe(v))
			}
		}
	}
	return w.Flush()
}
func birthdayKey(s string) string {
	if len(s) >= 10 && s[4] == '-' {
		return s[5:10]
	}
	if strings.HasPrefix(s, "--") {
		return strings.TrimPrefix(s, "--")
	}
	if s == "" {
		return "zz"
	}
	return s
}

func (r Runner) writeContact(c *Card, cfg Config, o Options) (resultErr error) {
	creating := c == nil
	var book Book
	if creating {
		if len(o.Terms) > 0 {
			return fmt.Errorf("new does not accept search terms")
		}
		if len(o.Books) > 1 {
			return fmt.Errorf("new requires one address book")
		}
		if len(o.Books) == 0 {
			if len(cfg.Books) != 1 {
				return fmt.Errorf("choose an address book with -a")
			}
			book = cfg.Books[0]
		} else {
			for _, b := range cfg.Books {
				if b.Name == o.Books[0] {
					book = b
				}
			}
			if book.Path == "" {
				return fmt.Errorf("unknown address book %q", o.Books[0])
			}
		}
		c = &Card{Props: []Property{newProp("BEGIN", "VCARD"), newProp("VERSION", cfg.Version), newProp("END", "VCARD")}}
	}
	var data []byte
	var e error
	edit := false
	if o.Input != "" && o.Input != "-" {
		p, e := expandPath(o.Input)
		if e != nil {
			return e
		}
		data, e = os.ReadFile(p)
		if e != nil {
			return e
		}
	} else if o.Input == "-" || !r.Interactive {
		data, e = io.ReadAll(io.LimitReader(r.In, 16*1024*1024+1))
		if e != nil {
			return e
		}
		if len(bytesTrim(data)) == 0 {
			return fmt.Errorf("no input; use an interactive terminal or -i FILE")
		}
	} else {
		if o.Format == "vcard" {
			data = c.bytes()
		} else {
			m := c.model(cfg)
			if creating {
				m["Phone"] = map[string]any{"": nil}
				m["Email"] = map[string]any{"": nil}
			}
			data, e = encodeModel(m)
			if e != nil {
				return e
			}
		}
		edit = true
	}
	if edit || o.OpenEditor {
		var recovery string
		data, recovery, e = r.editor(data, cfg, o.Format)
		defer func() {
			if recovery == "" {
				return
			}
			if resultErr != nil {
				fmt.Fprintf(r.Err, "Your editor input is preserved at %s\n", recovery)
			} else {
				_ = os.Remove(recovery)
			}
		}()
		if e != nil {
			return e
		}
	}
	if len(data) > 16*1024*1024 {
		return fmt.Errorf("contact input exceeds 16 MiB")
	}
	changed := true
	if o.Format == "vcard" {
		parsed, e := parseCard(data)
		if e != nil {
			return e
		}
		if parsed.first("VERSION") != "3.0" && parsed.first("VERSION") != "4.0" {
			return fmt.Errorf("vCard version must be 3.0 or 4.0")
		}
		if parsed.first("FN") == "" {
			return fmt.Errorf("contact requires FN")
		}
		if !creating && parsed.first("UID") != c.first("UID") {
			return fmt.Errorf("editing UID is unsupported")
		}
		changed = string(data) != string(c.bytes())
		parsed.Path = c.Path
		parsed.Book = c.Book
		parsed.Original = c.Original
		c = parsed
	} else {
		m, e := decodeModel(data)
		if e != nil {
			return e
		}
		changed, e = c.applyModel(m, cfg)
		if e != nil {
			return e
		}
	}
	if !changed && !creating {
		fmt.Fprintln(r.Out, "No changes.")
		return nil
	}
	if creating {
		e = createCard(c, book)
	} else {
		e = saveCard(c)
	}
	if e != nil {
		return e
	}
	if creating {
		fmt.Fprintf(r.Out, "Created %s [%s]\n", safe(c.name(cfg.Display)), safe(c.Book))
	} else {
		fmt.Fprintf(r.Out, "Updated %s [%s]\n", safe(c.name(cfg.Display)), safe(c.Book))
	}
	return nil
}
func bytesTrim(b []byte) []byte { return []byte(strings.TrimSpace(string(b))) }

func (r Runner) editor(data []byte, cfg Config, format string) ([]byte, string, error) {
	systemEditor := runtime.GOOS == "windows" && len(cfg.Editor) == 0
	ext := ".yaml"
	if format == "vcard" {
		ext = ".vcf"
	}
	if systemEditor {
		ext += ".txt"
	}
	f, e := os.CreateTemp("", "khard-xp-*"+ext)
	if e != nil {
		return nil, "", e
	}
	path := f.Name()
	if _, e = f.Write(data); e != nil {
		f.Close()
		return nil, path, e
	}
	if e = f.Close(); e != nil {
		return nil, path, e
	}
	if systemEditor {
		if !r.Interactive {
			return nil, path, fmt.Errorf("default Windows editor requires interactive input; configure editor or use -i FILE; contact was not saved")
		}
		if e = openDefaultEditor(path); e != nil {
			return nil, path, fmt.Errorf("default Windows editor: %w; contact was not saved", e)
		}
		fmt.Fprint(r.Out, "Save the file in your editor, then press Enter here to continue: ")
		if _, e = r.In.ReadString('\n'); e != nil {
			return nil, path, fmt.Errorf("editor confirmation: %w; contact was not saved", e)
		}
		result, err := os.ReadFile(path)
		return result, path, err
	}
	args := cfg.Editor
	if len(args) == 0 {
		s := os.Getenv("EDITOR")
		if s == "" {
			s = "vim"
		}
		args, e = splitCommand(s)
		if e != nil {
			return nil, path, e
		}
	}
	cmd := exec.Command(args[0], append(append([]string{}, args[1:]...), path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = r.Out
	cmd.Stderr = r.Err
	if e = cmd.Run(); e != nil {
		return nil, path, fmt.Errorf("editor %q: %w; contact was not saved", args[0], e)
	}
	result, e := os.ReadFile(path)
	if e != nil {
		return nil, path, e
	}
	return result, path, nil
}

func main() {
	info, e := os.Stdin.Stat()
	interactive := e == nil && info.Mode()&os.ModeCharDevice != 0
	r := Runner{In: bufio.NewReader(os.Stdin), Out: os.Stdout, Err: os.Stderr, Interactive: interactive}
	if e := r.run(os.Args[1:]); e != nil {
		if e != errNoResults {
			fmt.Fprintln(os.Stderr, "khard:", e)
		}
		os.Exit(1)
	}
}
