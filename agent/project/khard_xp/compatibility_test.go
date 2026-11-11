package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const compatibilityCard = "BEGIN:VCARD\nVERSION:3.0\nUID:compatibility\nN:Zoë;Alice;Marie;Dr.;Jr.\nFN:Alice Zoë\nNICKNAME:Ally\nTEL;TYPE=home:+49 (30) 123-4567\nTEL;TYPE=work:030 765-4321\nEMAIL;TYPE=work:alice@example.invalid\nADR;TYPE=home:;;12 Test Street;Test City;;10000;Test Country\nORG:Example;Engineering\nTITLE:Engineer\nROLE:Developer\nEND:VCARD\n"

func TestUpstreamPhoneSearchRules(t *testing.T) {
	c, err := parseCard([]byte(compatibilityCard))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		term string
		want bool
	}{
		{"+49 30 1234567", true}, {"49301234567", true}, {"0301234567", true},
		{"+49307654321", true}, {"home", true}, {"work", true}, {"999999", false},
		{"invalid", false}, {"", true}, {"030", true}, {"012", true},
	} {
		if got := c.matches([]string{"phone_numbers:" + tc.term}, Config{}); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.term, got, tc.want)
		}
	}
}

func TestUpstreamNamesAndPublicFields(t *testing.T) {
	c, err := parseCard([]byte(compatibilityCard))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.name("first_name"); got != "Alice Marie Zoë" {
		t.Fatal(got)
	}
	if got := c.name("last_name"); got != "Zoë, Alice Marie" {
		t.Fatal(got)
	}
	for _, query := range []string{"name:Zoë, Alice Marie", "name:Ally", "organisations:Engineering", "post_addresses:Test City", "roles:Developer", "titles:Engineer", "version:3.0", "kind:individual"} {
		if !c.matches([]string{query}, Config{}) {
			t.Errorf("missing match %q", query)
		}
	}
}

func TestCustomListFields(t *testing.T) {
	cfg, path := testConfig(t)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "compat.vcf"), []byte(compatibilityCard))
	fields := "INDEX,uid,name,first_name,last_name,emails.work.0,post_addresses.home.0.city,organisations.0.1,roles.0,emails.work.99"
	got, _, err := runTest(t, path, "", false, "list", "-p", "-F", fields)
	if err != nil {
		t.Fatal(err)
	}
	want := "1\tcompatibility\tAlice Zoë\tAlice\tZoë\talice@example.invalid\tTest City\tEngineering\tDeveloper\t\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, _, err := runTest(t, path, "", false, "list", "-p", "-F", "emails.work.-1,invalid"); err == nil {
		t.Fatal("accepted unknown field")
	}
	c, err := parseCard([]byte(compatibilityCard))
	if err != nil {
		t.Fatal(err)
	}
	view := columnValues{card: c}
	if got := view.nested("emails.work.-1"); got != "" {
		t.Fatalf("negative index: %q", got)
	}
	if got := view.nested("organisations"); got != "[['Example', 'Engineering']]" {
		t.Fatal(got)
	}
	if got := view.nested("emails"); got != "{'work': ['alice@example.invalid']}" {
		t.Fatal(got)
	}
}

func TestCustomFieldsExecutable(t *testing.T) {
	program := os.Getenv("KHARD_XP_CHECK_EXECUTABLE")
	if program == "" {
		t.Skip("set KHARD_XP_CHECK_EXECUTABLE to check a release executable")
	}
	cfg, path := testConfig(t)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "compat.vcf"), []byte(compatibilityCard))
	cmd := exec.Command(program, "-c", path, "list", "-p", "-F", "uid,roles.0")
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "compatibility\tDeveloper\n" {
		t.Fatalf("custom columns: %q, %v", output, err)
	}
	cmd = exec.Command(program, "-c", path, "list", "-p", "name:no-such-match")
	output, err = cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || len(output) != 0 {
		t.Fatalf("empty-list contract: %q, %v", output, err)
	}
}

func TestCollectionColumnEscaping(t *testing.T) {
	got := pythonValue([]any{"O'Connor", "both ' and \"", "line\nnext"}, false)
	if !strings.Contains(got, `"O'Connor"`) || !strings.Contains(got, `line\nnext`) || strings.Contains(got, "\n") {
		t.Fatalf("invalid collection escaping: %q", got)
	}
}
