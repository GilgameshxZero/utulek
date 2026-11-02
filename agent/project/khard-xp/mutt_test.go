package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Optional subprocess checks exercise main's std.obj/stderr/exit-code contract
// against the release executable, rather than only Runner's returned errors.
func TestMuttExecutableProtocol(t *testing.T) {
	program := os.Getenv("KHARD_XP_CHECK_EXECUTABLE")
	if program == "" {
		t.Skip("set KHARD_XP_CHECK_EXECUTABLE to check a release executable")
	}
	cfg, path := testConfig(t)
	name := `Zoë O'Connor "quoted" $(touch sentinel) & | ; % ! ^ < >`
	card := "BEGIN:VCARD\nVERSION:3.0\nUID:literal\nFN:" + escape(name) + "\nEMAIL;TYPE=work:literal@example.invalid\nEND:VCARD\n"
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "literal.vcf"), []byte(card))
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "alice.vcf"), []byte(fixture))
	for _, tc := range []struct {
		name    string
		args    []string
		want    string
		failure bool
	}{
		{"zero", []string{"email", "--mutt", "--", "no-such-match"}, "\n", false},
		{"single", []string{"email", "--mutt", "--", "Alice Zoë"}, "\nalice@example.invalid\tAlice Zoë\toffice, work\n", false},
		{"literal", []string{"email", "--mutt", "--", name}, "\nliteral@example.invalid\t" + name + "\twork\n", false},
		{"multiple", []string{"email", "--mutt", "--", "example.invalid"}, "\nalice@example.invalid\tAlice Zoë\toffice, work\nliteral@example.invalid\t" + name + "\twork\n", false},
		{"parsable", []string{"email", "--parsable", "--", "Alice Zoë"}, "alice@example.invalid\tAlice Zoë\toffice, work\n", false},
		{"missing-config", []string{"email", "--mutt", "-c", filepath.Join(t.TempDir(), "missing.conf")}, "", true},
		{"unknown-book", []string{"email", "--mutt", "-a", "missing-book"}, "", true},
		{"invalid-option", []string{"email", "--bad-option", "--mutt"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-c", path}, tc.args...)
			cmd := exec.Command(program, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if tc.failure {
				exit, ok := err.(*exec.ExitError)
				if !ok || exit.ExitCode() != 1 || !strings.HasPrefix(stdout.String(), "khard: ") || strings.Count(stdout.String(), "\n") != 1 || !strings.HasPrefix(stderr.String(), "khard: ") {
					t.Fatalf("invalid failure protocol: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
				}
			} else if err != nil || stdout.String() != tc.want || stderr.Len() != 0 {
				t.Fatalf("invalid success protocol: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestMuttQueryProtocol(t *testing.T) {
	cfg, path := testConfig(t)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "one.vcf"), []byte(fixture))
	second := strings.NewReplacer("example-one", "example-two", "Alice", "Bob", "alice@", "bob@").Replace(fixture)
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "two.vcf"), []byte(second))
	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"zero", "missing", nil},
		{"one", "Alice Zoë", []string{"alice@example.invalid"}},
		{"multiple", "example.invalid", []string{"alice@example.invalid", "bob@example.invalid"}},
		{"unicode", "Zoë", []string{"alice@example.invalid", "bob@example.invalid"}},
		{"all", "", []string{"alice@example.invalid", "bob@example.invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runTest(t, path, "", false, "email", "--mutt", "-d", "formatted_name", "--", tc.query)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(out, "\n")
			if lines[0] != "" || lines[len(lines)-1] != "" {
				t.Fatalf("missing newline-terminated status/rows: %q", out)
			}
			rows := lines[1 : len(lines)-1]
			if len(rows) != len(tc.want) {
				t.Fatalf("wrong row count (first result must survive): %q", out)
			}
			for i, row := range rows {
				fields := strings.Split(row, "\t")
				if len(fields) != 3 || fields[0] != tc.want[i] {
					t.Fatalf("invalid row: %q", row)
				}
			}
		})
	}
	before, _, err := runTest(t, path, "", false, "email", "--parsable", "--", "Alice Zoë")
	if err != nil {
		t.Fatal(err)
	}
	if before != "alice@example.invalid\tAlice Zoë\toffice, work\n" {
		t.Fatalf("parsable output changed: %q", before)
	}
	out, _, err := runTest(t, path, "", false, "email", "--parsable", "--mutt", "--", "Alice Zoë")
	if err != nil || out != "\n"+before {
		t.Fatalf("combined flags: %q %v", out, err)
	}
}

func TestMuttLiteralQueryAndSanitizedFields(t *testing.T) {
	cfg, path := testConfig(t)
	name := `Zoë O'Connor "quoted" $(touch sentinel) & | ; % ! ^ < >`
	card := "BEGIN:VCARD\nVERSION:3.0\nUID:literal\nFN:" + escape(name) + "\nEMAIL;TYPE=work:literal@example.invalid\nEND:VCARD\n"
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "literal.vcf"), []byte(card))
	out, _, err := runTest(t, path, "", false, "email", "--mutt", "--", name)
	if err != nil || out != "\nliteral@example.invalid\t"+name+"\twork\n" {
		t.Fatalf("literal query changed or evaluated: %q %v", out, err)
	}
	card = "BEGIN:VCARD\nVERSION:3.0\nUID:controls\nFN:Name\\nSecond\tColumn\nEMAIL;TYPE=work:control@example.invalid\nEND:VCARD\n"
	mustWrite(t, filepath.Join(cfg.Books[0].Path, "control.vcf"), []byte(card))
	out, _, err = runTest(t, path, "", false, "email", "--mutt", "--", "uid:controls")
	if err != nil || out != "\ncontrol@example.invalid\tName Second Column\twork\n" {
		t.Fatalf("row controls not sanitized: %q %v", out, err)
	}
}

func TestMuttErrorsHaveStatusAndNoRows(t *testing.T) {
	cfg, path := testConfig(t)
	for _, args := range [][]string{
		{"email", "--mutt", "-c", filepath.Join(t.TempDir(), "missing.conf")},
		{"email", "--unknown", "--mutt"},
		{"email", "--mutt", "-a", "missing-book"},
		{"list", "--mutt"},
		{"email", "--mutt=false"},
	} {
		out, _, err := runTest(t, path, "", false, args...)
		if err == nil || !strings.HasPrefix(out, "khard: ") || strings.Count(out, "\n") != 1 || strings.Contains(out, "\t") {
			t.Fatalf("invalid error status for %v: %q %v", args, out, err)
		}
	}
	if err := os.Remove(cfg.Books[0].Path); err != nil {
		t.Fatal(err)
	}
	out, _, err := runTest(t, path, "", false, "email", "--mutt")
	if err == nil || !strings.HasPrefix(out, "khard: ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("unreadable book: %q %v", out, err)
	}
}

func TestMuttFlagDetectionRespectsDataArguments(t *testing.T) {
	for _, args := range [][]string{{"email", "--", "--mutt"}, {"email", "-c", "--mutt"}, {"email", "--config=--mutt"}, {"merge", "-t", "--mutt"}, {"copy", "-A", "--mutt"}, {"add-email", "-H", "--mutt"}} {
		if muttRequested(args) {
			t.Fatalf("query or config value mistaken for flag: %v", args)
		}
	}
	if !muttRequested([]string{"email", "--bad-option", "--mutt"}) {
		t.Fatal("protocol missing after parse error")
	}
}
