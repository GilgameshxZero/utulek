package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, err := newApp(Config{Poll: 10, Timeout: 5, Theme: "auto"}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	return a
}
func settle(t *testing.T, a *App) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for len(a.Jobs) > 0 && time.Now().Before(until) {
		a.drain()
		time.Sleep(time.Millisecond)
	}
	if len(a.Jobs) > 0 {
		t.Fatal("background operation did not finish")
	}
}
func typeKeys(a *App, text string) {
	for _, r := range text {
		a.key(int(r), 12)
	}
}
func TestComposeModesAndPaste(t *testing.T) {
	a := testApp(t)
	a.openThread("+12025550123")
	a.key(13, 12)
	if a.Phase != "compose" {
		t.Fatal("Enter must open compose")
	}
	typeKeys(a, "qjs")
	a.key(keyShiftEnter, 12)
	typeKeys(a, "second")
	a.key(13, 12)
	if a.Phase != "review" || string(a.Body) != "qjs\nsecond" {
		t.Fatalf("incorrect compose transition: %q %s", a.Body, a.Phase)
	}
	a.key('e', 12)
	a.key(27, 12)
	if a.Phase != "review" {
		t.Fatal("Escape must finish compose")
	}
	a.key('e', 12)
	a.key(keyPasteStart, 12)
	typeKeys(a, "\nnot a send\n")
	a.key(keyPasteEnd, 12)
	if a.Phase != "compose" || len(a.Jobs) != 0 {
		t.Fatal("pasted newline activated a command")
	}
	a.key(keyF4, 12)
	if !strings.HasSuffix(string(a.Body), "\n") {
		t.Fatal("portable newline fallback failed")
	}
}
func TestSearchCancelRestoresExactListPosition(t *testing.T) {
	a := testApp(t)
	messages := []Message{}
	for i := 0; i < 30; i++ {
		number := fmt.Sprintf("+1202555%04d", 1000+i)
		a.Contacts[number] = Contact{fmt.Sprintf("Friend %02d", i), "mobile"}
		messages = append(messages, Message{ID: fmt.Sprint(i), Number: number, Direction: "in", Text: "hi", Timestamp: "2026-11-01T10:00:00Z", State: "Received"})
	}
	if err := a.Cache.put(messages); err != nil {
		t.Fatal(err)
	}
	a.Selected, a.Top = 15, 10
	original := a.capture()
	a.key('/', 12)
	typeKeys(a, "Friend 02")
	a.render(40, 12)
	a.key(27, 12)
	if a.Query != original.Query || a.Selected != original.Selected || a.Top != original.Top || a.entries()[a.Selected-1] != original.Anchor {
		t.Fatalf("cancel changed view: %+v", a.capture())
	}
}
func TestConfirmedSearchAndBackPreserveFilterAndAnchor(t *testing.T) {
	a := testApp(t)
	a.Selected = 2
	before := a.capture()
	a.key('/', 12)
	typeKeys(a, "Jordan")
	a.key(13, 12)
	if a.View != "thread" {
		t.Fatal("Enter should open search result")
	}
	a.key('q', 12)
	if a.View != "index" || a.Query != "Jordan" || a.entries()[a.Selected-1] != "+12025550124" {
		t.Fatal("conversation back lost filtered position")
	}
	a.key('q', 12)
	if a.Query != "" || a.Selected != before.Selected || a.Top != before.Top || !a.Running {
		t.Fatal("filter back did not restore original view")
	}
	a.key('q', 12)
	if a.Running {
		t.Fatal("q at root should quit")
	}
}
func TestBackPinsRecipientAfterListReorders(t *testing.T) {
	a := testApp(t)
	a.Selected = 2
	a.Top = 1
	before := a.capture()
	a.selectEntry()
	if err := a.Cache.put([]Message{{ID: "new", Number: "+12025559999", Direction: "in", Text: "new", Timestamp: "2026-11-02T00:00:00Z", State: "Received"}}); err != nil {
		t.Fatal(err)
	}
	a.key('q', 12)
	if a.entries()[a.Selected-1] != before.Anchor || a.Selected-a.Top != before.Selected-before.Top {
		t.Fatal("selected recipient/physical row changed")
	}
}
func TestHistorySearchCancelRestoresScroll(t *testing.T) {
	a := testApp(t)
	a.openThread("+12025550123")
	a.Scroll = 4
	a.key('/', 12)
	typeKeys(a, "dinner")
	a.render(40, 10)
	a.key(27, 12)
	if a.Scroll != 4 || a.Query != "" || a.View != "thread" {
		t.Fatal("history search cancellation lost scroll")
	}
}
func TestNestedNewConversationAndUnknownNumber(t *testing.T) {
	a := testApp(t)
	a.Selected = 2
	a.key('n', 12)
	a.key('/', 12)
	typeKeys(a, "Sam")
	a.key(13, 12)
	if a.Number != "+12025550125" {
		t.Fatal("new contact selection failed")
	}
	a.key('q', 12)
	if a.View != "contacts" || a.Query != "Sam" {
		t.Fatal("contact picker cursor lost")
	}
	a.key('q', 12)
	a.key('q', 12)
	if a.View != "index" || a.Selected != 2 {
		t.Fatal("original index selection lost")
	}
	a.key('n', 12)
	a.Prompt = func(title, initial string, secret bool) (string, bool) { return "+1 (202) 555-0199", true }
	a.Selected = 0
	a.selectEntry()
	if a.Number != "+12025550199" {
		t.Fatal("manual number failed")
	}
}
func TestFooterOnlyNoTitleOrRefreshing(t *testing.T) {
	a := testApp(t)
	a.Jobs["sync"] = true
	f := a.render(40, 10)
	if len(f.Lines) == 0 || f.Lines[0].Y != 0 || !strings.Contains(f.Lines[0].Text, "New conversation") {
		t.Fatal("title is consuming content row")
	}
	for _, line := range f.Lines {
		if strings.Contains(strings.ToLower(line.Text), "refresh") {
			t.Fatal("refresh status displayed")
		}
	}
	a.key('/', 10)
	typeKeys(a, "Alex")
	f = a.render(60, 10)
	footers := 0
	for _, line := range f.Lines {
		if line.Y == 9 {
			footers++
			if !strings.Contains(line.Text, "/Alex") {
				t.Fatal("search missing from shared footer")
			}
		}
		if line.Y != 9 && strings.Contains(line.Text, "/Alex") {
			t.Fatal("separate search row")
		}
	}
	if footers != 1 {
		t.Fatal("more than one footer")
	}
}
func TestCacheCompatiblePrivateAndNoDrafts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	config := Config{Server: "http://example.invalid", Username: "u", Password: "p", Cache: true}
	cache, err := openCache(config, false)
	if err != nil {
		t.Fatal(err)
	}
	message := Message{ID: "one", Number: "+12025550123", Direction: "out", Text: "unchanged", State: "Uncertain", Media: []Attachment{{Name: "photo.png", ContentType: "image/png", PartID: 4}}}
	if err = cache.put([]Message{message}); err != nil {
		t.Fatal(err)
	}
	path := cache.path
	cache.close()
	cache, err = openCache(config, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.close()
	if !reflect.DeepEqual(cache.records[message.key()], message) {
		t.Fatal("cache changed message or attachment metadata")
	}
	if !isPrivateFile(path) {
		t.Fatal("cache permissions")
	}
	if filepath.Dir(path) != filepath.Join(root, "smsgatec") {
		t.Fatal("cache outside XDG directory")
	}
}
func TestRecipientValidationDoesNotGuess(t *testing.T) {
	for _, raw := range []string{"2025550123", "+1+2025550123", "+1202oops5550123"} {
		if _, err := recipient(raw); err == nil {
			t.Fatal("unsafe normalization", raw)
		}
	}
	if value, err := recipient("+1 (202) 555-0123"); err != nil || value != "+12025550123" {
		t.Fatal(value, err)
	}
}
func TestAttachmentPickerOpenSaveAndBack(t *testing.T) {
	a := testApp(t)
	a.openThread("+12025550123")
	_ = a.Cache.put([]Message{{ID: "mms:1", Number: a.Number, Direction: "in", Text: "Picture", State: "Received", Type: "MMS_DOWNLOADED", Media: []Attachment{{PartID: 2, Name: "photo.png", ContentType: "image/png"}}}})
	a.Scroll = 3
	a.key('o', 12)
	if a.View != "picker" || a.Picker.Action != "open" || len(a.Picker.Choices) != 1 {
		t.Fatal("open attachment picker failed")
	}
	a.key('s', 12)
	if a.Picker.Action != "save" {
		t.Fatal("s should select save")
	}
	a.key('q', 12)
	if a.View != "thread" || a.Scroll != 3 {
		t.Fatal("attachment back changed conversation")
	}
	a.key('s', 12)
	if a.Picker.Action != "save" {
		t.Fatal("s in conversation should choose save picker")
	}
}
func TestSafeAttachmentSaveCannotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := saveExclusive(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := saveExclusive(path, []byte("replacement")); err == nil {
		t.Fatal("overwrote an existing file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("file changed")
	}
	name := safeName("../../evil.txt", "text/plain")
	if name != "evil.txt" {
		t.Fatal(name)
	}
}
func TestFilenamePickerAddsThenRemovesAttachment(t *testing.T) {
	a := testApp(t)
	a.openThread("+12025550123")
	root := t.TempDir()
	path := filepath.Join(root, "test.txt")
	_ = os.WriteFile(path, []byte("test"), 0600)
	a.LastFileDir = root
	a.key('a', 12)
	if a.View != "picker" {
		t.Fatal("file picker not opened")
	}
	for i, item := range a.Picker.Files {
		if item.Path == path {
			a.Picker.Selected = i
		}
	}
	a.key(13, 12)
	if a.View != "thread" || len(a.DraftFiles) != 1 || a.Phase != "review" {
		t.Fatal("attachment not selected")
	}
	a.key('d', 12)
	if len(a.DraftFiles) != 0 {
		t.Fatal("attachment removal failed")
	}
}
