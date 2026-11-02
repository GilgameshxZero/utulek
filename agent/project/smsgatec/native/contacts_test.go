package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVCards(t *testing.T) {
	data := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Kai " + "\r\n Example\r\nN:Example;Kai;;;\r\nNICKNAME:K\\,K\r\nNOTE:NYC\\nFriend\r\nitem1.TEL;TYPE=\"cell,voice\":tel:+1-203-555-0123\r\nTEL:020 1234 5678\r\nEND:VCARD\r\n"
	got, err := parseVCards(context.Background(), data, "nyc k,k")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["+12035550123"].Name != "Kai Example" || got["02012345678"].Kind != "" {
		t.Fatalf("wrong contacts: %#v", got)
	}
	got, err = parseVCards(context.Background(), data, "missing")
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if vcardText(`a\\nb\nc\,d\;e`) != "a\\nb\nc,d;e" {
		t.Fatal("escape decoding")
	}
}
func TestContactsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.VCF"), []byte("BEGIN:VCARD\nN:Example;Jane;;;\nTEL:+12025550123\nEND:VCARD\nBEGIN:VCARD\nFN:Other\nTEL:+12025550124\nEND:VCARD\n"), 0600)
	got, err := readContacts(context.Background(), dir, "")
	if err != nil || len(got) != 2 || got["+12025550123"].Name != "Jane Example" {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = readContacts(ctx, dir, ""); err == nil {
		t.Fatal("cancel ignored")
	}
}
