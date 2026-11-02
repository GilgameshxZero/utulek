package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func operationBooks(t *testing.T) (Config, string, *Card, Book) {
	t.Helper()
	cfg, path := testConfig(t)
	dest := Book{Name: "destination", Path: filepath.Join(filepath.Dir(path), "Destination 雪 with spaces")}
	if e := os.Mkdir(dest.Path, 0700); e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	// Add the second subsection inside addressbooks, before general.
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	mustWrite(t, path, []byte(strings.Replace(string(data), "[general]", "[[destination]]\npath = '"+dest.Path+"'\n[general]", 1)))
	cfg, e = loadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	sourcePath := filepath.Join(cfg.Books[0].Path, "source.vcf")
	mustWrite(t, sourcePath, []byte(fixture))
	cards, e := loadCards(cfg, []string{"default"})
	if e != nil {
		t.Fatal(e)
	}
	return cfg, path, cards[0], dest
}
func TestCopyMoveLifecycle(t *testing.T) {
	for _, command := range []string{"copy", "move"} {
		t.Run(command, func(t *testing.T) {
			cfg, path, source, dest := operationBooks(t)
			if _, _, e := runTest(t, path, "", false, command, "-a", "default", "-A", dest.Name, "uid:example-one"); e != nil {
				t.Fatal(e)
			}
			cards, e := loadCards(cfg, []string{dest.Name})
			if e != nil || len(cards) != 1 {
				t.Fatal(cards, e)
			}
			c := cards[0]
			if command == "copy" {
				if c.first("UID") == source.first("UID") {
					t.Fatal("copy retained UID")
				}
				data, e := os.ReadFile(source.Path)
				if e != nil || !bytes.Equal(data, source.Original) {
					t.Fatal("copy altered source", e)
				}
			} else {
				if c.first("UID") != source.first("UID") {
					t.Fatal("move changed UID")
				}
				if _, e := os.Stat(source.Path); !os.IsNotExist(e) {
					t.Fatal("move retained source", e)
				}
			}
			if c.first("REV") == "" {
				t.Fatal("missing REV")
			}
			for _, raw := range []string{"X-EXTRA;X-PARAM=preserve:custom\\;data", "item1.TEL;TYPE=home;X-KEEP=one:", "PHOTO;ENCODING=b;TYPE=JPEG:YWJj\r\n ZGVm"} {
				if !bytes.Contains(c.Original, []byte(raw)) {
					t.Fatalf("lost %q", raw)
				}
			}
		})
	}
}
func TestTransferFailures(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		_, _, c, b := operationBooks(t)
		mustWrite(t, c.Path, []byte(fixture+"\n"))
		if e := transferCard(c, b, false); e == nil {
			t.Fatal("accepted stale source")
		}
		files, _ := os.ReadDir(b.Path)
		if len(files) != 0 {
			t.Fatal("stale copy wrote destination")
		}
	})
	t.Run("collision", func(t *testing.T) {
		_, _, c, b := operationBooks(t)
		dest := filepath.Join(b.Path, c.first("UID")+".vcf")
		mustWrite(t, dest, []byte("do not clobber"))
		if e := transferCard(c, b, true); e == nil {
			t.Fatal("clobbered")
		}
		data, _ := os.ReadFile(dest)
		if string(data) != "do not clobber" {
			t.Fatal("destination modified")
		}
		if e := sameOriginal(c); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("unsafe UID", func(t *testing.T) {
		_, _, c, b := operationBooks(t)
		c.replace("UID", []Property{newProp("UID", "../escape")})
		c.Original = c.bytes()
		mustWrite(t, c.Path, c.Original)
		if e := transferCard(c, b, true); e == nil {
			t.Fatal("accepted path UID")
		}
	})
	t.Run("locked", func(t *testing.T) {
		_, _, c, b := operationBooks(t)
		mustWrite(t, c.Path+".khard-win.lock", nil)
		if e := transferCard(c, b, false); e == nil {
			t.Fatal("ignored lock")
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		_, path, c, b := operationBooks(t)
		mustWrite(t, filepath.Join(b.Path, "duplicate.vcf"), c.Original)
		if _, _, e := runTest(t, path, "", false, "copy", "-a", "default", "-A", b.Name); e == nil {
			t.Fatal("duplicate requires decision")
		}
		if e := sameOriginal(c); e != nil {
			t.Fatal(e)
		}
	})
}
func mergeRunner(input string) (Runner, *bytes.Buffer) {
	var out, errs bytes.Buffer
	return Runner{In: bufio.NewReader(strings.NewReader(input)), Out: &out, Err: &errs, Interactive: true}, &errs
}
func TestMergeLifecycleAndRecovery(t *testing.T) {
	for _, mode := range []string{"success", "unchanged", "invalid", "cancel", "stale-source", "stale-target"} {
		t.Run(mode, func(t *testing.T) {
			cfg, _, source, b := operationBooks(t)
			target := cloneCard(source)
			target.replace("UID", []Property{newProp("UID", "target-id")})
			target.Path = filepath.Join(b.Path, "target.vcf")
			target.Book = b.Name
			target.Original = target.bytes()
			mustWrite(t, target.Path, target.Original)
			exe, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			cfg.MergeEditor = []string{exe, "-test.run=^TestEditorHelper$", "--"}
			t.Setenv("KHARD_XP_EDITOR_HELPER", "1")
			text := "Note: merged note 雪\n"
			if mode == "unchanged" {
				data, e := encodeModel(target.model(cfg))
				if e != nil {
					t.Fatal(e)
				}
				text = string(data)
			}
			if mode == "invalid" {
				text = "Note: [bad"
			}
			t.Setenv("KHARD_XP_EDITOR_TEXT", text)
			input := "y\n"
			if mode == "cancel" {
				input = "n\n"
			}
			if mode == "stale-source" {
				mustWrite(t, source.Path, []byte(fixture+"\n"))
			}
			if mode == "stale-target" {
				mustWrite(t, target.Path, append(target.Original, '\n'))
			}
			r, errs := mergeRunner(input)
			e = r.mergeContacts(source, target, cfg, true)
			if mode == "success" {
				if e != nil {
					t.Fatal(e)
				}
				if _, e := os.Stat(source.Path); !os.IsNotExist(e) {
					t.Fatal("source retained", e)
				}
				data, _ := os.ReadFile(target.Path)
				if !bytes.Contains(data, []byte("NOTE:merged note")) || !bytes.Contains(data, []byte("UID:target-id")) || !bytes.Contains(data, []byte("X-EXTRA;X-PARAM=preserve:")) {
					t.Fatal("merge lost data")
				}
			} else {
				if mode == "unchanged" {
					if e != nil {
						t.Fatal(e)
					}
				} else {
					if e == nil {
						t.Fatal("expected error")
					}
					marker := "Your merge input is preserved at "
					if !strings.Contains(errs.String(), marker) {
						t.Fatal("missing recovery", errs.String())
					}
					recovery := strings.TrimSpace(strings.Split(errs.String(), marker)[1])
					if _, e := os.Stat(recovery); e != nil {
						t.Fatal(e)
					}
					os.Remove(recovery)
				}
				if _, e := os.Stat(source.Path); e != nil {
					t.Fatal("lost source", e)
				}
				data, _ := os.ReadFile(target.Path)
				if mode != "stale-target" && !bytes.Equal(data, target.Original) {
					t.Fatal("target changed on failure")
				}
			}
		})
	}
}
func TestMailHeaderParsing(t *testing.T) {
	data := []byte("From: =?UTF-8?Q?Zo=C3=AB?= <zoe@example.invalid>\r\nTo: Friends: Alice <alice@example.invalid>,\r\n Bob <bob@example.invalid>;\r\nReply-To: Reply <reply@example.invalid>\r\nSubject: not an address\r\n\r\nFrom: body@example.invalid\n")
	for _, tc := range []struct {
		headers string
		count   int
	}{{"", 1}, {"to", 2}, {"from,to", 3}, {"reply-to", 1}, {"all", 4}} {
		a, e := readMailAddresses(data, tc.headers)
		if e != nil || len(a) != tc.count {
			t.Fatal(tc, a, e)
		}
	}
	a, e := readMailAddresses(data, "")
	if e != nil || a[0].Name != "Zoë" {
		t.Fatal(a, e)
	}
	if _, e := readMailAddresses([]byte("From: broken <\n"), ""); e == nil {
		t.Fatal("accepted malformed address")
	}
	if _, e := readMailAddresses([]byte("Subject: empty\n"), ""); e == nil {
		t.Fatal("accepted no address")
	}
}
func TestAddEmailLifecycle(t *testing.T) {
	cfg, path, source, _ := operationBooks(t)
	mailPath := filepath.Join(filepath.Dir(path), "mail header.eml")
	mustWrite(t, mailPath, []byte("From: Alice Zoë <new@example.invalid>\n"))
	if _, _, e := runTest(t, path, "y\nyes\nwork\n", true, "add-email", "-a", "default", "-i", mailPath); e != nil {
		t.Fatal(e)
	}
	cards, e := loadCards(cfg, []string{"default"})
	if e != nil {
		t.Fatal(e)
	}
	c := cards[0]
	if len(c.properties("EMAIL")) != 2 || !strings.Contains(string(c.Original), "EMAIL;TYPE=work:new@example.invalid") {
		t.Fatal("missing email", string(c.Original))
	}
	if !strings.Contains(string(c.Original), "item2.X-ABLabel:office") || !strings.Contains(string(c.Original), "X-EXTRA;X-PARAM=preserve:") {
		t.Fatal("lost metadata")
	}
	before := c.Original
	if _, _, e := runTest(t, path, "", false, "add-email", "--skip-already-added", "-a", "default", "-i", mailPath); e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(source.Path)
	if !bytes.Equal(before, data) {
		t.Fatal("skip rewrote contact")
	}
	mustWrite(t, mailPath, []byte("From: New Person <person@example.invalid>\n"))
	if _, _, e := runTest(t, path, "y\ncreate\n1\n\n\n\ny\n\n", true, "add-email", "-a", "default", "-i", mailPath); e != nil {
		t.Fatal(e)
	}
	cards, e = loadCards(cfg, []string{"default"})
	if e != nil || len(cards) != 2 {
		t.Fatal(cards, e)
	}
}

func TestEmailCustomLabelsAndPreferences(t *testing.T) {
	for _, v := range []string{"3.0", "4.0"} {
		t.Run(v, func(t *testing.T) {
			c := mustCard(t, strings.Replace(fixture, "VERSION:3.0", "VERSION:"+v, 1))
			if e := c.addEmailAddress("work,Team,pref=3", "team@example.invalid"); e != nil {
				t.Fatal(e)
			}
			var added Property
			for _, p := range c.properties("EMAIL") {
				if p.Value == "team@example.invalid" {
					added = p
				}
			}
			if added.Group != "itememail1" || !strings.Contains(added.Head, "TYPE=work,X-Team") {
				t.Fatal(added)
			}
			if !strings.Contains(c.label(added), "Team") {
				t.Fatal("missing custom label")
			}
			if v == "4.0" && !strings.Contains(added.Head, "PREF=3") {
				t.Fatal(added)
			}
			if v == "3.0" && !strings.Contains(added.Head, ",pref") {
				t.Fatal(added)
			}
			if e := c.addEmailAddress("Team", "another@example.invalid"); e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(string(c.bytes()), "itememail2.EMAIL") {
				t.Fatal("group collision")
			}
			before := c.bytes()
			if e := c.addEmailAddress("Team,Other", "bad@example.invalid"); e == nil {
				t.Fatal("accepted multiple custom labels")
			}
			if !bytes.Equal(before, c.bytes()) {
				t.Fatal("invalid addition modified contact")
			}
		})
	}
}

func TestMergeEditorFailureAndSingleEditorFallback(t *testing.T) {
	for _, mode := range []string{"failure", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			cfg, _, source, b := operationBooks(t)
			target := cloneCard(source)
			target.Path = filepath.Join(b.Path, "target.vcf")
			target.Book = b.Name
			target.replace("UID", []Property{newProp("UID", "target")})
			target.Original = target.bytes()
			mustWrite(t, target.Path, target.Original)
			exe, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			editor := []string{exe, "-test.run=^TestEditorHelper$", "--"}
			t.Setenv("KHARD_XP_EDITOR_HELPER", "1")
			t.Setenv("KHARD_XP_EDITOR_TEXT", "Note: merged\n")
			if mode == "failure" {
				cfg.MergeEditor = []string{filepath.Join(t.TempDir(), "missing editor")}
			} else {
				cfg.MergeEditor = nil
				cfg.Editor = editor
			}
			r, errs := mergeRunner("y\n")
			e = r.mergeContacts(source, target, cfg, false)
			if mode == "failure" {
				if e == nil {
					t.Fatal("accepted editor failure")
				}
				marker := "Your merge input is preserved at "
				if !strings.Contains(errs.String(), marker) {
					t.Fatal(errs.String())
				}
				os.Remove(strings.TrimSpace(strings.Split(errs.String(), marker)[1]))
				if e := sameOriginal(target); e != nil {
					t.Fatal(e)
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				data, _ := os.ReadFile(target.Path)
				if !bytes.Contains(data, []byte("NOTE:merged")) {
					t.Fatal("fallback failed")
				}
			}
			if e := sameOriginal(source); e != nil {
				t.Fatal("source changed", e)
			}
		})
	}
}
