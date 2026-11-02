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

func TestConfigObjQuotedAndMultilineValues(t *testing.T) {
	for _, tc := range []struct {
		input  string
		values []string
		list   bool
	}{{`" a ", "", b,`, []string{" a ", "", "b"}, true}, {`,`, nil, true}, {`'C:\Users\Test # Person', -i, NONE`, []string{`C:\Users\Test # Person`, "-i", "NONE"}, true}, {`""`, []string{""}, false}} {
		got, list, e := configValue(tc.input)
		if e != nil || list != tc.list || !reflect.DeepEqual(got, tc.values) {
			t.Errorf("%s: %#v %t %v", tc.input, got, list, e)
		}
	}
	for _, delimiter := range []string{`"""`, `'''`} {
		scan := bufio.NewScanner(strings.NewReader("  second # literal\n[not a section], still literal\n" + delimiter + " # comment\n"))
		line := 1
		values, list, e := configScannedValue(scan, &line, delimiter+"first")
		if e != nil || list || values[0] != "first\n  second # literal\n[not a section], still literal\n" || line != 4 {
			t.Fatal(values, list, e, line)
		}
	}
	for _, input := range []string{"a,,b", `"""unclosed`, `'''closed''' trailing`, `"unterminated`} {
		scan := bufio.NewScanner(strings.NewReader(""))
		line := 1
		if _, _, e := configScannedValue(scan, &line, input); e == nil {
			t.Errorf("accepted %q", input)
		}
	}
	cfg, path := testConfig(t)
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	data = bytes.Replace(data, []byte("editor = vim, -i, NONE"), []byte("editor = \"\"\"vim\n-i NONE\"\"\" # command"), 1)
	mustWrite(t, path, data)
	cfg, e = loadConfig(path)
	if e != nil || !reflect.DeepEqual(cfg.Editor, []string{"vim", "-i", "NONE"}) {
		t.Fatal(cfg.Editor, e)
	}
}

func TestSourceSearchAndDebug(t *testing.T) {
	cfg, path := testConfig(t)
	data, _ := os.ReadFile(path)
	data = bytes.Replace(data, []byte("search_in_source_files = yes"), []byte("search_in_source_files = no"), 1)
	mustWrite(t, path, data)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "alice.vcf"), []byte(fixture))
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "broken.vcf"), []byte("broken unrelated card"))
	if _, _, e := runTest(t, path, "", false, "list", "-p", "name:Alice"); e == nil {
		t.Fatal("unfiltered search hid malformed card")
	}
	out, err, e := runTest(t, path, "", false, "list", "-p", "-f", "--debug", "name:Alice")
	if e != nil || !strings.Contains(out, "example-one") || !strings.Contains(err, "prefiltered=1 parsed=1 matched=1") {
		t.Fatal(out, err, e)
	}
	os.Remove(filepath.Join(cfg.Books[0].Path, "broken.vcf"))
	plain, err, e := runTest(t, path, "", false, "list", "-p", "name:Alice")
	if e != nil || err != "" || plain != out {
		t.Fatal("debug altered output", plain, err, e)
	}
	data = bytes.Replace(data, []byte("default_action = list"), []byte("default_action = list\ndebug = yes"), 1)
	mustWrite(t, path, data)
	_, err, e = runTest(t, path, "", false, "list", "-p", "name:Alice")
	if e != nil || !strings.Contains(err, "DEBUG khard:") {
		t.Fatal(err, e)
	}
	q := compileSearch([]string{"name:Alice"})
	folded := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:folded\r\nFN:Ali\r\n ce\r\nEND:VCARD\r\n")
	c := mustCard(t, string(folded))
	if !c.matches([]string{"name:Alice"}, cfg) || q.matchesSource(folded) {
		t.Fatal("source mode should preserve upstream's folding false negative")
	}
}

func TestVisibleGeneralSearchAndFieldBoundaries(t *testing.T) {
	c := mustCard(t, fixture)
	c.Book = "synthetic address book"
	cfg := Config{Private: []string{"SOCIAL-SECURITY"}}
	for _, tc := range []struct {
		term string
		want bool
	}{{"custom;data", false}, {"synthetic-only", true}, {"social-security", true}, {"synthetic address book", true}, {"REV:", false}, {"BEGIN:VCARD", false}, {"3.0", false}, {"name: Alice Zoë", false}, {"full name: Alice Marie Zoë", true}, {"phone", true}, {"miscellaneous", true}, {"voice", true}, {"emails:internet", false}, {"NAME:ALICE", true}, {"notes:second line", true}, {"missing:value", false}} {
		got, e := compileSearch([]string{tc.term}).match(c, cfg)
		if e != nil || got != tc.want {
			t.Errorf("%q: %t %v", tc.term, got, e)
		}
	}
	if c.matches([]string{"synthetic-only"}, Config{}) {
		t.Fatal("unconfigured private value leaked into search")
	}
	empty := mustCard(t, "BEGIN:VCARD\nVERSION:3.0\nUID:empty\nFN:Empty\nEND:VCARD\n")
	if empty.matches([]string{"emails:"}, Config{}) {
		t.Fatal("empty collection matched empty query")
	}
}
func TestDateSearchSemantics(t *testing.T) {
	for _, tc := range []struct {
		property, term string
		want, invalid  bool
	}{
		{"BDAY:19900403", "birthday:1990-04-03", true, false},
		{"BDAY:0001-01-01", "birthday:0001-01-01", true, false},
		{"ANNIVERSARY:bad\nX-ANNIVERSARY:20010102", "anniversary:2001-01-02", true, false}, {"BDAY:1990-04-03", "birthday:1990", false, true}, {"BDAY:1990-04-03", "birthday:", false, true},
		{"BDAY:--0403", "birthday:1900-04-03", true, false}, {"BDAY:--04-03", "birthday:1900-04-03", true, false}, {"BDAY:19900403T120000Z", "birthday:1990-04-03", false, false},
		{"BDAY:19900403T000000+00:00", "birthday:1990-04-03", false, false}, {"BDAY;VALUE=text:spring day", "birthday:spring", true, false}, {"BDAY:invalid", "birthday:invalid", false, false},
		{"X-ANNIVERSARY:20010102", "anniversary:2001-01-02", true, false}, {"", "birthday:", false, false},
	} {
		c := mustCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:date\nFN:Date\n"+tc.property+"\nEND:VCARD\n")
		got, e := compileSearch([]string{tc.term}).match(c, Config{})
		if got != tc.want || (e != nil) != tc.invalid {
			t.Errorf("%s %s: %t %v", tc.property, tc.term, got, e)
		}
	}
}

func TestStage3Executable(t *testing.T) {
	program := os.Getenv("KHARD_XP_CHECK_EXECUTABLE")
	if program == "" {
		t.Skip("set KHARD_XP_CHECK_EXECUTABLE to test release executable")
	}
	cfg, path := testConfig(t)
	data, _ := os.ReadFile(path)
	data = bytes.Replace(data, []byte("search_in_source_files = yes"), []byte("search_in_source_files = no"), 1)
	mustWrite(t, path, data)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "valid.vcf"), []byte(fixture))
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "bad.vcf"), []byte("malformed unrelated"))
	cmd := exec.Command(program, "-c", path, "list", "-p", "-F", "uid", "-f", "--debug", "name:Alice")
	var errors bytes.Buffer
	cmd.Stderr = &errors
	output, e := cmd.Output()
	if e != nil || string(output) != "example-one\n" || !strings.Contains(errors.String(), "prefiltered=1 parsed=1 matched=1") {
		t.Fatal(string(output), errors.String(), e)
	}
	cmd = exec.Command(program, "-c", path, "list", "-p", "-F", "uid", "name:Alice")
	if e := cmd.Run(); e == nil {
		t.Fatal("unfiltered executable hid malformed input")
	}
}
func TestConfigScalarValidationAndMergeEnvironment(t *testing.T) {
	_, path := testConfig(t)
	original, _ := os.ReadFile(path)
	for _, replacement := range []struct{ old, new string }{{"skip_unparsable = no", "skip_unparsable = no, yes"}, {"preferred_version = 3.0", "preferred_version = 3.0, 4.0"}, {"sort = formatted_name", "sort = formatted_name, first_name"}, {"private_objects = SOCIAL-SECURITY,", "private_objects = -bad,"}} {
		mustWrite(t, path, bytes.Replace(original, []byte(replacement.old), []byte(replacement.new), 1))
		if _, e := loadConfig(path); e == nil {
			t.Errorf("accepted %s", replacement.new)
		}
	}
	t.Setenv("MERGE_EDITOR", `helper 'path with spaces'`)
	mustWrite(t, path, bytes.Replace(original, []byte("merge_editor = vimdiff"), nil, 1))
	cfg, e := loadConfig(path)
	if e != nil || !reflect.DeepEqual(cfg.MergeEditor, []string{"helper", "path with spaces"}) {
		t.Fatal(cfg.MergeEditor, e)
	}
	mustWrite(t, path, original)
	cfg, e = loadConfig(path)
	if e != nil || !reflect.DeepEqual(cfg.MergeEditor, []string{"vimdiff"}) {
		t.Fatal("config should override environment", cfg.MergeEditor, e)
	}
}

func TestSimpleSearchFastPathMatchesDocument(t *testing.T) {
	for _, text := range []string{fixture, compatibilityCard, "BEGIN:VCARD\nVERSION:4.0\nUID:tags\nFN:Tagged Person\nORG:Example\\, Inc.;Unit\nitem1.TITLE:Captain\nitem1.X-ABLabel:jobtag\nitem2.X-SECRET:private word\nitem2.X-ABLabel:secrettag\nEMAIL;TYPE=work,X-Team:team@example.invalid\nBDAY:20000102\nANNIVERSARY;VALUE=text:Winter Holiday\nEND:VCARD\n"} {
		c := mustCard(t, text)
		c.Book = "search book"
		cfg := Config{Private: []string{"SECRET", "SOCIAL-SECURITY"}}
		view := columnValues{card: c, search: true}
		document := strings.ToLower(searchDocument(c, cfg, &view))
		candidates := strings.Fields(document + " " + text + " absent-token X-UNKNOWN")
		seen := map[string]bool{}
		for _, word := range candidates {
			runes := []rune(strings.ToLower(word))
			for start := range runes {
				for end := start + 1; end <= len(runes) && end <= start+8; end++ {
					term := string(runes[start:end])
					if seen[term] || !simpleSearchAtom(term) {
						continue
					}
					seen[term] = true
					want := strings.Contains(document, term)
					got := quickVisibleMatch(c, cfg, term)
					if got != want {
						t.Fatalf("term %q: fast=%t document=%t", term, got, want)
					}
				}
			}
		}
	}
}

func TestBookPathEnvironment(t *testing.T) {
	t.Setenv("KHARD_XP_TEST_BOOK", t.TempDir())
	for _, input := range []string{"$KHARD_XP_TEST_BOOK/contacts", "${KHARD_XP_TEST_BOOK}/contacts"} {
		path, e := expandBookPath(input)
		if e != nil || path != filepath.Join(os.Getenv("KHARD_XP_TEST_BOOK"), "contacts") {
			t.Fatal(path, e)
		}
	}
	t.Setenv("KHARD_XP_UNDEFINED_TEST", "")
	os.Unsetenv("KHARD_XP_UNDEFINED_TEST")
	path, e := expandBookPath("${KHARD_XP_UNDEFINED_TEST}/contacts")
	if e != nil || !strings.Contains(path, "${KHARD_XP_UNDEFINED_TEST}") {
		t.Fatal("unknown variable changed", path, e)
	}
	cfg, config := testConfig(t)
	data, _ := os.ReadFile(config)
	data = bytes.Replace(data, []byte("path = '"+cfg.Books[0].Path+"'"), []byte(`path = """$KHARD_XP_TEST_BOOK""" # literal config value
type = vdir`), 1)
	mustWrite(t, config, data)
	loaded, e := loadConfig(config)
	if e != nil || loaded.Books[0].Path != os.Getenv("KHARD_XP_TEST_BOOK") {
		t.Fatal(loaded.Books, e)
	}
}
