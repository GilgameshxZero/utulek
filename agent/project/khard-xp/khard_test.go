package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:example-one\r\nN:Zoë;Alice;Marie;;\r\nFN:Alice Zoë\r\nTEL:+15550100001\r\nitem1.TEL;TYPE=home;X-KEEP=one:+15550100002\r\nitem2.EMAIL;TYPE=work:alice@example.invalid\r\nitem2.X-ABLabel:office\r\nADR;TYPE=home:;;12 Test Street;Test City;;10000;Test Country\r\nNOTE:Old\\nsecond line\r\nORG:Example;Engineering\r\nCATEGORIES:Friends,Test\\,One\r\nBDAY:1990-04-03\r\nX-EXTRA;X-PARAM=preserve:custom\\;data\r\nPHOTO;ENCODING=b;TYPE=JPEG:YWJj\r\n ZGVm\r\nX-SOCIAL-SECURITY:synthetic-only\r\nEND:VCARD\r\n"

func mustWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func testConfig(t *testing.T) (Config, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "Address Book with spaces — 雪")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "khard.conf")
	s := fmt.Sprintf("# same ConfigObj syntax on both platforms\n[addressbooks]\n[[default]]\npath = '%s'\n[general]\ndefault_action = list\neditor = vim, -i, NONE\nmerge_editor = vimdiff\n[contact table]\ndisplay = formatted_name\nsort = formatted_name\nshow_uids = no\nreverse = false\ngroup_by_addressbook = no\npreferred_phone_number_type = home, pref\n[vcard]\nprivate_objects = SOCIAL-SECURITY,\npreferred_version = 3.0\nsearch_in_source_files = yes\nskip_unparsable = no\n", dir)
	mustWrite(t, path, []byte(s))
	cfg, e := loadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	return cfg, path
}
func mustCard(t *testing.T, s string) *Card {
	t.Helper()
	c, e := parseCard([]byte(s))
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func runTest(t *testing.T, config, input string, interactive bool, args ...string) (string, string, error) {
	t.Helper()
	var out, errs bytes.Buffer
	r := Runner{In: bufio.NewReader(strings.NewReader(input)), Out: &out, Err: &errs, Interactive: interactive}
	args = append([]string{"-c", config}, args...)
	e := r.run(args)
	return out.String(), errs.String(), e
}

func TestConfigInterchange(t *testing.T) {
	cfg, path := testConfig(t)
	if len(cfg.Editor) != 3 || cfg.Editor[2] != "NONE" || len(cfg.Private) != 1 || cfg.Private[0] != "SOCIAL-SECURITY" {
		t.Fatalf("wrong config: %#v", cfg)
	}
	t.Setenv("KHARD_CONFIG", path)
	a, e := loadConfig("")
	if e != nil || a.Path != path {
		t.Fatalf("KHARD_CONFIG: %v", e)
	}
	t.Setenv("KHARD_CONFIG", "")
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	p, e := defaultConfigPath()
	if e != nil || p != filepath.Join(root, "khard", "khard.conf") {
		t.Fatal(p, e)
	}
	values, list, e := configValue(`"C:\Users\Test Person\contacts", '-i', NONE # comment`)
	if e != nil || !list || values[0] != `C:\Users\Test Person\contacts` || len(values) != 3 {
		t.Fatalf("Windows list parsing: %q %v", values, e)
	}
	values, list, e = configValue(`"~/a,#b/" # trailing comment`)
	if e != nil || list || values[0] != "~/a,#b/" {
		t.Fatal(values, e)
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	p, e = expandPath("~/contacts/file.vcf")
	if e != nil || p != filepath.Join(home, "contacts", "file.vcf") {
		t.Fatal(p, e)
	}
}

func TestConfigRejectsMalformed(t *testing.T) {
	_, path := testConfig(t)
	for _, s := range []string{"[addressbooks]\n[[x]]\npath = 'unclosed\n", "[addressbooks]\n[[x]]\npath=foo\npath=bar\n", "[addressbooks]\n[[x]]\n", "[addressbooks]\n[[x]]\npath=foo\n[vcard]\npreferred_version=5.0\n"} {
		mustWrite(t, path, []byte(s))
		if _, e := loadConfig(path); e == nil {
			t.Fatal("accepted malformed config", s)
		}
	}
}

func TestYAMLRoundTripPreservesOriginal(t *testing.T) {
	cfg, _ := testConfig(t)
	c := mustCard(t, fixture)
	data, e := encodeModel(c.model(cfg))
	if e != nil {
		t.Fatal(e)
	}
	m, e := decodeModel(data)
	if e != nil {
		t.Fatal(e)
	}
	changed, e := c.applyModel(m, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if changed || string(c.bytes()) != fixture {
		t.Fatalf("no-op changed original: %v\n%s", changed, c.bytes())
	}
	if !c.matches([]string{"name:alice", "emails:example.invalid", "phone_numbers:0002", "note:second"}, cfg) {
		t.Fatal("field search failed")
	}
	if c.matches([]string{"name:example.invalid"}, cfg) {
		t.Fatal("field query leaked into all fields")
	}
}

func TestEditingPreservesUnknownPropertiesAndLabels(t *testing.T) {
	cfg, _ := testConfig(t)
	c := mustCard(t, fixture)
	changed, e := c.applyModel(Model{"Note": "A new note\n雪;comma,slash\\n"}, cfg)
	if e != nil || !changed {
		t.Fatal(changed, e)
	}
	for _, key := range []string{"TEL", "EMAIL", "X-ABLABEL", "X-EXTRA", "PHOTO", "UID", "X-SOCIAL-SECURITY"} {
		orig := mustCard(t, fixture).properties(key)
		now := c.properties(key)
		for i := range orig {
			if orig[i].Raw != now[i].Raw {
				t.Fatalf("%s changed", key)
			}
		}
	}
	if c.first("NOTE") != "A new note\n雪;comma,slash\\n" {
		t.Fatal("escaping damaged note", c.first("NOTE"))
	}
	m := c.model(cfg)
	m["Phone"] = map[string]any{"": "+15550100003", "home": "+15550100004"}
	if _, e = c.applyModel(m, cfg); e != nil {
		t.Fatal(e)
	}
	pp := c.properties("TEL")
	if len(pp) != 2 || pp[0].Head != "TEL" || pp[1].Head != "item1.TEL;TYPE=home;X-KEEP=one" {
		t.Fatalf("phone metadata changed: %#v", pp)
	}
}

func TestVCard4PhonesAndUnicodeFolding(t *testing.T) {
	cfg, _ := testConfig(t)
	c := mustCard(t, "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:v4\r\nFN:雪 Test\r\nTEL;VALUE=uri;PREF=1;TYPE=home:tel:+15550100001;ext=42\r\nEND:VCARD\r\n")
	data, e := encodeModel(c.model(cfg))
	if e != nil {
		t.Fatal(e)
	}
	m, e := decodeModel(data)
	if e != nil {
		t.Fatal(e)
	}
	changed, e := c.applyModel(m, cfg)
	if changed || e != nil {
		t.Fatal(changed, e)
	}
	label := c.label(c.properties("TEL")[0])
	_, e = c.applyModel(Model{"Phone": map[string]any{label: "+15550100002;ext=43"}}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	p := c.properties("TEL")[0]
	if p.Value != "tel:+15550100002;ext=43" {
		t.Fatalf("invalid URI: %s", p.Value)
	}
	value := strings.Repeat("雪", 90) + "\\,;\n"
	_, e = c.applyModel(Model{"Note": value}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	again := mustCard(t, string(c.bytes()))
	if again.first("NOTE") != value {
		t.Fatal("folding damaged Unicode")
	}
	for _, line := range strings.Split(c.properties("NOTE")[0].Raw, "\r\n") {
		if len(line) > 75 {
			t.Fatal("line longer than 75 octets")
		}
	}
}

func TestStorageConflictLockAndSafeLifecycle(t *testing.T) {
	cfg, _ := testConfig(t)
	book := cfg.Books[0]
	path := filepath.Join(book.Path, "existing.vcf")
	mustWrite(t, path, []byte(fixture))
	c := mustCard(t, fixture)
	c.Path = path
	if _, e := c.applyModel(Model{"Note": "changed"}, cfg); e != nil {
		t.Fatal(e)
	}
	mustWrite(t, path, []byte(strings.Replace(fixture, "Old", "external", 1)))
	if e := saveCard(c); e == nil || !strings.Contains(e.Error(), "changed on disk") {
		t.Fatal("did not detect external change", e)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "external") {
		t.Fatal("overwrote external change")
	}
	mustWrite(t, path, []byte(fixture))
	mustWrite(t, path+".khard-win.lock", nil)
	if e := saveCard(c); e == nil {
		t.Fatal("ignored existing lock")
	}
	os.Remove(path + ".khard-win.lock")
	if e := saveCard(c); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(path)
	if !strings.Contains(string(b), "NOTE:changed") || !strings.Contains(string(b), "REV:") {
		t.Fatal("save failed")
	}
	if e := removeCard(c); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("remove failed")
	}
	c = mustCard(t, fixture)
	if e := createCard(c, book); e != nil {
		t.Fatal(e)
	}
	if c.first("UID") == "example-one" {
		t.Fatal("new card reused UID")
	}
	b, _ = os.ReadFile(c.Path)
	if !bytes.Equal(b, c.bytes()) {
		t.Fatal("new contact differs")
	}
	entries, e := os.ReadDir(book.Path)
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}

func TestCLILifecycleAndSelection(t *testing.T) {
	cfg, path := testConfig(t)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "one.vcf"), []byte(fixture))
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "-p", "alice"}, "example-one\tAlice Zoë\tdefault"},
		{[]string{"phone", "-p", "name:alice"}, "+15550100001\tAlice Zoë\t"},
		{[]string{"email", "-p"}, "alice@example.invalid"},
		{[]string{"show", "--format", "vcard", "uid:example-one"}, fixture},
		{[]string{"show", "--format", "yaml", "alice"}, "Formatted name: Alice Zoë"},
		{[]string{"birthdays"}, "1990-04-03"},
		{[]string{"postaddress"}, "12 Test Street"},
		{[]string{"filename"}, "one.vcf"},
		{[]string{"addressbooks"}, "default\t"},
	} {
		out, _, e := runTest(t, path, "", false, test.args...)
		if e != nil || !strings.Contains(out, test.want) {
			t.Fatalf("%v: %v\n%s", test.args, e, out)
		}
	}
	input := "First name: Bob\nLast name: Example\nPhone:\n    \"\": '+15550100003'\nEmail:\n    work: bob@example.invalid\nNote: |\n    Hello\n    雪\n"
	if out, _, e := runTest(t, path, input, false, "new"); e != nil || !strings.Contains(out, "Created Bob Example") {
		t.Fatal(out, e)
	}
	if _, _, e := runTest(t, path, "", false, "remove", "--force"); e == nil || !strings.Contains(e.Error(), "2 contacts match") {
		t.Fatal("ambiguous remove should fail", e)
	}
	if _, _, e := runTest(t, path, "", false, "remove", "bob"); e == nil {
		t.Fatal("noninteractive removal should require force")
	}
	if out, _, e := runTest(t, path, "Note: Edited safely\n", false, "edit", "bob"); e != nil || !strings.Contains(out, "Updated") {
		t.Fatal(out, e)
	}
	if _, _, e := runTest(t, path, "", false, "remove", "--force", "bob"); e != nil {
		t.Fatal(e)
	}
	cards, e := loadCards(cfg, nil)
	if e != nil || len(cards) != 1 {
		t.Fatal(len(cards), e)
	}
	if _, _, e := runTest(t, path, "", false, "merge"); e != nil {
		t.Fatal("merging a contact with itself should be a no-op", e)
	}
}

func TestEditorHelper(t *testing.T) {
	if os.Getenv("KHARD_XP_EDITOR_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	if e := os.WriteFile(path, []byte(os.Getenv("KHARD_XP_EDITOR_TEXT")), 0600); e != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestExternalEditorAndRecovery(t *testing.T) {
	cfg, _ := testConfig(t)
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Editor = []string{exe, "-test.run=^TestEditorHelper$", "--"}
	t.Setenv("KHARD_XP_EDITOR_HELPER", "1")
	t.Setenv("KHARD_XP_EDITOR_TEXT", "Formatted name: Edited 雪\nPhone:\n  \"\": '+15550100005'\n")
	var out, errs bytes.Buffer
	r := Runner{In: bufio.NewReader(strings.NewReader("")), Out: &out, Err: &errs, Interactive: true}
	if e := r.writeContact(nil, cfg, Options{}); e != nil {
		t.Fatal(e)
	}
	cards, e := loadCards(cfg, nil)
	if e != nil || len(cards) != 1 || cards[0].first("FN") != "Edited 雪" {
		t.Fatal(cards, e)
	}
	if cards[0].properties("TEL")[0].Head != "TEL" {
		t.Fatal("added an inferred phone label")
	}
	t.Setenv("KHARD_XP_EDITOR_TEXT", "Bad field: invalid\n")
	e = r.writeContact(cards[0], cfg, Options{})
	if e == nil {
		t.Fatal("accepted invalid edit")
	}
	prefix := "Your editor input is preserved at "
	text := errs.String()
	idx := strings.Index(text, prefix)
	if idx < 0 {
		t.Fatal("no recovery path", text)
	}
	path := strings.TrimSpace(text[idx+len(prefix):])
	defer os.Remove(path)
	if b, e := os.ReadFile(path); e != nil || string(b) != "Bad field: invalid\n" {
		t.Fatal("editor recovery failed", e)
	}
	b, e := os.ReadFile(cards[0].Path)
	if e != nil || !bytes.Equal(b, cards[0].Original) {
		t.Fatal("invalid input changed real file", e)
	}
}

func TestRejectsMalformedContactInput(t *testing.T) {
	for _, s := range []string{"[a, b]", "Note: a\nNote: b\n", "Note: &a [*a]\n", "Note: a\n---\nNote: b\n", "Unknown: field\n"} {
		if _, e := decodeModel([]byte(s)); e == nil {
			t.Fatal("accepted invalid YAML", s)
		}
	}
	for _, s := range []string{"BEGIN:VCARD\nVERSION:3.0\n", fixture + fixture, "BEGIN:VCARD\nVERSION:3.0\nVERSION:4.0\nEND:VCARD\n"} {
		if _, e := parseCard([]byte(s)); e == nil {
			t.Fatal("accepted invalid vCard")
		}
	}
	cfg, _ := testConfig(t)
	for _, m := range []Model{{"Phone": "missing label"}, {"First name": map[string]any{"bad": "map"}}, {"Private": map[string]any{"UNCONFIGURED": "value"}}} {
		c := mustCard(t, fixture)
		if _, e := c.applyModel(m, cfg); e == nil {
			t.Fatal("accepted invalid model", m)
		}
	}
}

func TestAuditAddressBookReadOnly(t *testing.T) {
	path := os.Getenv("KHARD_XP_AUDIT_CONFIG")
	if path == "" {
		t.Skip("set KHARD_XP_AUDIT_CONFIG for opt-in read-only compatibility audit")
	}
	cfg, e := loadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	cards, e := loadCards(cfg, nil)
	if e != nil {
		t.Fatal(e)
	}
	for i, c := range cards {
		data, e := encodeModel(c.model(cfg))
		if e != nil {
			t.Fatalf("contact #%d YAML export: %v", i+1, e)
		}
		m, e := decodeModel(data)
		if e != nil {
			t.Fatalf("contact #%d YAML import: %v", i+1, e)
		}
		changed, e := c.applyModel(m, cfg)
		if e != nil || changed || !bytes.Equal(c.bytes(), c.Original) {
			t.Fatalf("contact #%d would change on an unedited YAML round trip: changed=%v error=%v", i+1, changed, e)
		}
	}
	t.Logf("%d contacts audited without writing any contact files", len(cards))
}

func TestSelectionAndNewFileNoClobber(t *testing.T) {
	cfg, _ := testConfig(t)
	a, b := mustCard(t, fixture), mustCard(t, strings.Replace(fixture, "Alice", "Bob", 1))
	r := Runner{In: bufio.NewReader(strings.NewReader("2\n")), Out: io.Discard, Err: io.Discard, Interactive: true}
	selected, e := r.choose([]*Card{a, b}, cfg)
	if e != nil || selected != b {
		t.Fatal("selection failed", e)
	}
	root := t.TempDir()
	from, to := filepath.Join(root, "from"), filepath.Join(root, "to")
	mustWrite(t, from, []byte("new"))
	mustWrite(t, to, []byte("existing"))
	if e := installNewFile(from, to); e == nil {
		t.Fatal("new file overwrote an existing destination")
	}
	data, e := os.ReadFile(to)
	if e != nil || string(data) != "existing" {
		t.Fatal("destination changed", e)
	}
}
