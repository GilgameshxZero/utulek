package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestYAMLReplacementAndPatchThroughCLI(t *testing.T) {
	for _, mode := range []string{"replace", "patch", "unsafe-partial"} {
		t.Run(mode, func(t *testing.T) {
			cfg, path := testConfig(t)
			contactPath := filepath.Join(cfg.Books[0].Path, "contact.vcf")
			mustWrite(t, contactPath, []byte(fixture))
			input := "Note: changed\n"
			args := []string{"edit", "uid:example-one"}
			if mode == "replace" {
				input = "First name: Alice\nLast name: Zoë\nNote: changed\n"
			}
			if mode == "patch" {
				args = append(args, "--patch")
			}
			_, _, e := runTest(t, path, input, false, args...)
			data, _ := os.ReadFile(contactPath)
			if mode == "unsafe-partial" {
				if e == nil || !strings.Contains(e.Error(), "--patch") || !bytes.Equal(data, []byte(fixture)) {
					t.Fatal("unsafe partial replacement wrote data", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			c := mustCard(t, string(data))
			if mode == "replace" {
				for _, key := range []string{"TEL", "EMAIL", "ADR", "ORG", "BDAY", "X-SOCIAL-SECURITY"} {
					if len(c.properties(key)) > 0 {
						t.Errorf("omitted %s survived", key)
					}
				}
				if len(c.properties("X-ABLABEL")) != 0 {
					t.Fatal("orphan label survived")
				}
			} else {
				if len(c.properties("TEL")) != 2 || !strings.Contains(string(data), "item2.X-ABLabel:office") {
					t.Fatal("patch removed fields")
				}
			}
			for _, raw := range []string{"X-EXTRA;X-PARAM=preserve:custom\\;data", "PHOTO;ENCODING=b;TYPE=JPEG:YWJj\r\n ZGVm"} {
				if !bytes.Contains(data, []byte(raw)) {
					t.Fatalf("lost raw property %q", raw)
				}
			}
			if c.first("UID") != "example-one" || c.first("NOTE") != "changed" {
				t.Fatal("identity or edit lost")
			}
		})
	}
}
func TestReplacementThroughConfiguredEditor(t *testing.T) {
	cfg, _ := testConfig(t)
	path := filepath.Join(cfg.Books[0].Path, "contact.vcf")
	mustWrite(t, path, []byte(fixture))
	c := mustCard(t, fixture)
	c.Path = path
	c.Book = cfg.Books[0].Name
	model := c.model(cfg)
	delete(model, "Note")
	text, e := encodeModel(model)
	if e != nil {
		t.Fatal(e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Editor = []string{exe, "-test.run=^TestEditorHelper$", "--"}
	t.Setenv("KHARD_XP_EDITOR_HELPER", "1")
	t.Setenv("KHARD_XP_EDITOR_TEXT", string(text))
	var out, errs bytes.Buffer
	r := Runner{In: bufio.NewReader(strings.NewReader("")), Out: &out, Err: &errs, Interactive: true}
	if e = r.writeContact(c, cfg, Options{}); e != nil {
		t.Fatal(e, errs.String())
	}
	data, _ := os.ReadFile(path)
	saved := mustCard(t, string(data))
	if saved.first("NOTE") != "" || len(saved.properties("TEL")) != 2 || !bytes.Contains(data, []byte("X-EXTRA;X-PARAM=preserve:")) {
		t.Fatal("editor replacement damaged fields")
	}
}
func TestYAMLTagsCategoriesImppAndDates(t *testing.T) {
	cfg, _ := testConfig(t)
	c := mustCard(t, strings.Replace(fixture, "VERSION:3.0", "VERSION:4.0", 1))
	before := c.bytes()
	data := []byte("First name: Alice\nLast name: Zoë\nKind: group\nRole:\n - team: Maintainer\nNote:\n - source: tagged note\nOrganisation:\n - unit:\n   - Company\n   - Team\nImpp:\n xmpp, pref=2: alice@example.invalid\nCategories:\n - [One, Two]\n - [Three]\nBirthday: --04-03\nAnniversary: text= Spring day\nPrivate:\n SOCIAL-SECURITY:\n  account: value\n")
	m, e := decodeModel(data)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.applyYAML(m, cfg, false); e != nil {
		t.Fatal(e)
	}
	if c.first("KIND") != "group" || len(c.properties("CATEGORIES")) != 2 || c.first("IMPP") != "xmpp:alice@example.invalid" {
		t.Fatal("missing values", string(c.bytes()))
	}
	exported, e := encodeModel(c.model(cfg))
	if e != nil {
		t.Fatal(e)
	}
	round, e := decodeModel(exported)
	if e != nil {
		t.Fatal(e)
	}
	snapshot := c.bytes()
	changed, e := c.applyYAML(round, cfg, false)
	if e != nil || changed || !bytes.Equal(snapshot, c.bytes()) {
		t.Fatal("tagged round trip rewrote contact", changed, e, string(exported))
	}
	for _, bad := range []Model{{"Birthday": "2000-02-30"}, {"Note": map[string]any{"one": "a", "two": "b"}}, {"Impp": map[string]any{"bad:type": "name"}}} {
		snapshot = c.bytes()
		if _, e := c.applyModel(bad, cfg); e == nil || !bytes.Equal(snapshot, c.bytes()) {
			t.Fatal("invalid model mutated contact", bad, e)
		}
	}
	if bytes.Equal(before, c.bytes()) {
		t.Fatal("fixture did not change")
	}
}
func TestNativeLocaleCollationAndDates(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	loc, e := checkedLocale()
	if e != nil {
		t.Fatal(e)
	}
	cfg := Config{Locale: loc, LocalizeDates: true, Sort: "formatted_name"}
	c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:date\nFN:Date\nBDAY:19900403\nEND:VCARD\n")
	if got := formatContactDate(c, "birthday", cfg); got != "04/03/90" {
		t.Fatal(got)
	}
	cfg.LocalizeDates = false
	if got := formatContactDate(c, "birthday", cfg); got != "1990-04-03" {
		t.Fatal(got)
	}
	cards := []*Card{mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:z\nFN:Zoë\nEND:VCARD\n"), mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:a\nFN:Åke\nEND:VCARD\n")}
	sortCards(cards, cfg)
	if cards[0].first("UID") != "z" {
		t.Fatal("C collation changed")
	}
	loc, e = newNativeLocale("sv_SE.UTF-8", "sv_SE.UTF-8")
	if e != nil {
		t.Setenv("LOCPATH", filepath.Join(".obj", "locales"))
		loc, e = newNativeLocale("sv_SE.UTF-8", "sv_SE.UTF-8")
	}
	if e != nil {
		t.Skip("Swedish locale unavailable", e)
	}
	cfg.Locale = loc
	sortCards(cards, cfg)
	if cards[0].first("UID") != "z" {
		t.Fatal("Swedish Å should sort after Z")
	}
	cfg.LocalizeDates = true
	if got := formatContactDate(c, "birthday", cfg); got != "1990-04-03" {
		t.Fatal("Swedish date", got)
	}
}
func TestDiscoveryAndLiteralConfigValues(t *testing.T) {
	cfg, path := testConfig(t)
	root := filepath.Dir(path)
	for _, dir := range []string{"a/shared", "b/shared", "a/shared/.hidden"} {
		if e := os.MkdirAll(filepath.Join(root, "discover", dir), 0700); e != nil {
			t.Fatal(e)
		}
	}
	data, _ := os.ReadFile(path)
	data = bytes.Replace(data, []byte("[general]"), []byte("[[scan]]\npath = '"+filepath.Join(root, "discover", "**")+"'\ntype = discover\n[general]"), 1)
	mustWrite(t, path, data)
	loaded, e := loadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	var names []string
	for _, b := range loaded.Books {
		names = append(names, b.Name)
	}
	if !reflect.DeepEqual(names, []string{cfg.Books[0].Name, "shared", "shared-1"}) {
		t.Fatal(names)
	}
}
func TestPresentationExecutable(t *testing.T) {
	program := os.Getenv("KHARD_XP_CHECK_EXECUTABLE")
	if program == "" {
		t.Skip("set KHARD_XP_CHECK_EXECUTABLE for release checks")
	}
	cfg, path := testConfig(t)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "contact.vcf"), []byte(fixture))
	for _, tc := range []struct {
		args     []string
		contains string
	}{{[]string{"list"}, "Address book:"}, {[]string{"show", "uid:example-one"}, "Full name: Alice Marie Zoë"}, {[]string{"birthdays", "-p"}, "1990.04.03\tAlice Zoë"}, {[]string{"postaddress"}, "12 Test Street"}} {
		cmd := exec.Command(program, append([]string{"-c", path}, tc.args...)...)
		data, e := cmd.CombinedOutput()
		if e != nil || !strings.Contains(string(data), tc.contains) {
			t.Fatal(tc.args, string(data), e)
		}
	}
}

func TestChineseValuesAndLocale(t *testing.T) {
	c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:中文\nN:李;明;;;\nFN:李明\nBDAY:19900403\nEND:VCARD\n")
	cfg := Config{}
	if _, e := c.applyModel(Model{"Email": map[string]any{"工作": "li@example.invalid"}, "Address": map[string]any{"家庭": map[string]any{"Street": "中山路一号", "City": "上海", "Country": "中国"}}}, cfg); e != nil {
		t.Fatal(e)
	}
	data, e := encodeModel(c.model(cfg))
	if e != nil {
		t.Fatal(e)
	}
	m, e := decodeModel(data)
	if e != nil {
		t.Fatal(e)
	}
	before := c.bytes()
	changed, e := c.applyYAML(m, cfg, false)
	if changed || e != nil || !bytes.Equal(before, c.bytes()) {
		t.Fatal("Chinese YAML round trip changed data", changed, e)
	}
	for _, locale := range []string{"zh_CN.UTF-8", "zh_TW.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			loc, e := newNativeLocale(locale, locale)
			if e != nil {
				t.Setenv("LOCPATH", filepath.Join(".obj", "locales"))
				loc, e = newNativeLocale(locale, locale)
			}
			if e != nil {
				t.Skip("Chinese locale unavailable", e)
			}
			cfg.Locale = loc
			cfg.LocalizeDates = true
			date := formatContactDate(c, "birthday", cfg)
			if !strings.Contains(date, "1990") {
				t.Fatal("date was not localized", date)
			}
			if loc.key("阿明") == "阿明" {
				t.Fatal("Chinese collation fell back to raw Unicode bytes")
			}
			if locale == "zh_CN.UTF-8" && loc.key("阿明") >= loc.key("波明") {
				t.Fatal("Simplified Chinese pinyin ordering failed")
			}
		})
	}
}
func TestCJKTableWidthsAndCollectionQuoting(t *testing.T) {
	if columnWidth("李明") != 4 || columnWidth("A\u0301") != 1 || columnWidth("張美華") != 6 {
		t.Fatal("incorrect display widths")
	}
	var out bytes.Buffer
	if e := printTable(&out, [][]string{{"Name", "Value"}, {"李明", "x"}, {"Zoe", "x"}}); e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(out.String(), "\n")
	a := strings.LastIndex(lines[1], "x")
	b := strings.LastIndex(lines[2], "x")
	if columnWidth(lines[1][:a]) != columnWidth(lines[2][:b]) {
		t.Fatal("Chinese columns misaligned", out.String())
	}
	if got := pythonValue([]any{"\x01", "\u3000", nil}, false); got != `['\x01', '\u3000', None]` {
		t.Fatal(got)
	}
}
func TestYAMLAliasesAndNulls(t *testing.T) {
	m, e := decodeModel([]byte("First name: &first Alice\nLast name: *first\nPhone:\n home: null\n"))
	if e != nil || m["First name"] != "Alice" || m["Last name"] != "Alice" {
		t.Fatal(m, e)
	}
	if m["Phone"].(map[string]any)["home"] != nil {
		t.Fatal("null was not empty")
	}
	if _, e := decodeModel([]byte("Note: &cycle [*cycle]\n")); e == nil {
		t.Fatal("cyclic alias was accepted")
	}
}

func TestUpstreamOptionFormsAndScopes(t *testing.T) {
	o, e := parseArgs([]string{"list", "-rp", "-afamily", "-Fuid,name"})
	if e != nil || !o.Reverse || !o.Parsable || !reflect.DeepEqual(o.Books, []string{"family"}) || o.Fields != "uid,name" {
		t.Fatal(o, e)
	}
	for _, args := range [][]string{{"list", "--parsable=yes"}, {"edit", "--patch=false"}} {
		if _, e := parseArgs(args); e == nil {
			t.Fatal("boolean value accepted", args)
		}
	}
	for _, o := range []Options{{Command: "new", Provided: []string{"-p"}}, {Command: "template", Provided: []string{"-a"}}, {Command: "birthdays", Provided: []string{"-s"}}} {
		if e := validateCommandOptions(o); e == nil {
			t.Fatal("irrelevant option accepted", o)
		}
	}
	if muttRequested([]string{"show", "-c--mutt"}) {
		t.Fatal("attached config value mistaken for protocol")
	}
}

func TestDateOffsetAndPrecisionPreservation(t *testing.T) {
	loc, e := newNativeLocale("C", "C")
	if e != nil {
		t.Fatal(e)
	}
	cfg := Config{Locale: loc}
	for _, tc := range []struct{ raw, iso string }{{"20000102T000000+0200", "2000-01-02T00:00:00+02:00"}, {"20000102T000000.123", "2000-01-02T00:00:00.123"}} {
		c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:precision\nFN:Date\nBDAY:"+tc.raw+"\nEND:VCARD\n")
		if got := isoModelDate(c, "birthday"); got != tc.iso {
			t.Fatal(got)
		}
		before := c.bytes()
		data, e := encodeModel(c.model(cfg))
		if e != nil {
			t.Fatal(e)
		}
		m, e := decodeModel(data)
		if e != nil {
			t.Fatal(e)
		}
		changed, e := c.applyYAML(m, cfg, false)
		if changed || e != nil || !bytes.Equal(before, c.bytes()) {
			t.Fatal("precision round trip changed data", changed, e)
		}
	}
	c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:offset\nFN:Date\nBDAY:20000102T000000+0200\nEND:VCARD\n")
	if got := formatContactDate(c, "birthday", cfg); got != "2000-01-02T00:00:00+02:00" {
		t.Fatal(got)
	}
	cfg.LocalizeDates = true
	if !strings.Contains(formatContactDate(c, "birthday", cfg), "00:00:00") {
		t.Fatal("offset date-time was displayed as a date only")
	}
}

func TestReducedLeapDayIsPreserved(t *testing.T) {
	c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:leap\nFN:Leap Person\nBDAY:--0229\nEND:VCARD\n")
	cfg := Config{}
	if got := formatContactDate(c, "birthday", cfg); got != "--02-29" {
		t.Fatal("valid unknown-year leap date hidden", got)
	}
	m := c.model(cfg)
	if m["Birthday"] != "--02-29" {
		t.Fatal(m["Birthday"])
	}
	before := c.bytes()
	changed, e := c.applyYAML(m, cfg, false)
	if changed || e != nil || !bytes.Equal(before, c.bytes()) {
		t.Fatal("leap date round trip changed data", changed, e)
	}
	if _, e := c.applyModel(Model{"Birthday": "--02-29"}, cfg); e != nil {
		t.Fatal(e)
	}
}

func TestChineseQuotedBookNames(t *testing.T) {
	cfg, path := testConfig(t)
	data, _ := os.ReadFile(path)
	data = bytes.Replace(data, []byte("[[default]]"), []byte(`[["通讯录#1"]] # comment [brackets]`), 1)
	mustWrite(t, path, data)
	loaded, e := loadConfig(path)
	if e != nil || loaded.Books[0].Name != "通讯录#1" || loaded.Books[0].Path != cfg.Books[0].Path {
		t.Fatal(loaded.Books, e)
	}
	depth, name, e := configSection(`[[通讯录#2]] # comment`)
	if e != nil || depth != 2 || name != "通讯录#2" {
		t.Fatal(depth, name, e)
	}
}
