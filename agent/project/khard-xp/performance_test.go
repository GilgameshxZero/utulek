package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadCards(b *testing.B) {
	dir := b.TempDir()
	for i := 0; i < 754; i++ {
		data := []byte(fmt.Sprintf("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:%d\r\nN:Contact%04d;Example;;;\r\nFN:Example Contact%04d\r\nEMAIL:example%d@example.invalid\r\nEND:VCARD\r\n", i, (i*317)%754, i, i))
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d.vcf", i)), data, 0600); err != nil {
			b.Fatal(err)
		}
	}
	cfg := Config{Books: []Book{{Name: "benchmark", Path: dir}}, Sort: "first_name"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loadCards(cfg, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatches(b *testing.B) {
	c, err := parseCard([]byte(fixture))
	if err != nil {
		b.Fatal(err)
	}
	for _, terms := range [][]string{nil, {"name:Alice"}, {"Alice", "Engineering"}} {
		b.Run(fmt.Sprint(terms), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !c.matches(terms, Config{}) {
					b.Fatal("missing fixture match")
				}
			}
		})
	}
}

func TestSearchPreservesFieldsAndLiteralTerms(t *testing.T) {
	c, err := parseCard([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Private: []string{"SOCIAL-SECURITY"}}
	for _, tc := range []struct {
		terms []string
		want  bool
	}{
		{nil, true},
		{[]string{"name:ALICE", "name:Zoë"}, true},
		{[]string{"uid:example-one"}, true},
		{[]string{"formatted_name:Alice"}, true},
		{[]string{"first_name:Alice", "last_name:Zoë"}, true},
		{[]string{"emails:alice@example.invalid", "phone_numbers:+15550100002"}, true},
		{[]string{"addresses:Test City", "notes:second line"}, true},
		{[]string{"private:synthetic-only"}, true},
		{[]string{"custom;data"}, false},
		{[]string{"Engineering", "Alice"}, true},
		{[]string{"Country", "Alice"}, true},
		{[]string{"Country", "missing"}, false},
		{[]string{"PHOTO:YWJj"}, false},
		{[]string{"missing:value"}, false},
		{[]string{"name:missing"}, false},
		{[]string{"Alice", "missing"}, false},
	} {
		if got := c.matches(tc.terms, cfg); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.terms, got, tc.want)
		}
	}
}

func TestFilteredLoadOrderingAndMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	for file, text := range map[string]string{
		"a.vcf": "BEGIN:VCARD\nVERSION:3.0\nFN:Zed\nEND:VCARD\n",
		"b.vcf": "BEGIN:VCARD\nVERSION:3.0\nFN:alice\nEND:VCARD\n",
		"c.vcf": "BEGIN:VCARD\nVERSION:3.0\nFN:Alice\nEND:VCARD\n",
	} {
		mustWrite(t, filepath.Join(dir, file), []byte(text))
	}
	cfg := Config{Books: []Book{{Name: "test", Path: dir}}, Sort: "formatted_name", Reverse: true}
	cards, err := loadMatchingCards(cfg, nil, []string{"name:alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || filepath.Base(cards[0].Path) != "b.vcf" || filepath.Base(cards[1].Path) != "c.vcf" {
		t.Fatalf("filtered order: %v", cards)
	}
	cards, err = loadCards(cfg, nil)
	if err != nil || len(cards) != 3 || cards[0].first("FN") != "Zed" {
		t.Fatalf("reverse order: %v, %v", cards, err)
	}
	mustWrite(t, filepath.Join(dir, "bad.vcf"), []byte("invalid"))
	if _, err = loadMatchingCards(cfg, nil, []string{"no matches"}); err == nil {
		t.Fatal("filter hid malformed contact")
	}
	cfg.Skip = true
	if cards, err = loadMatchingCards(cfg, nil, []string{"no matches"}); err != nil || len(cards) != 0 {
		t.Fatalf("skip malformed: %v, %v", cards, err)
	}
}
