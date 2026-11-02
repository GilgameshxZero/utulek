package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestHistoryAccountIdentityAndDurability(t *testing.T) {
	raw := fixtureThread(`["native-a",1793545877539,"+15555550000",[],10,1,null,null,null,"same",null,null,5,1,null]`, `["native-b",1793545877539,"+15555550000",[],11,1,null,null,null,"same",null,null,6,1,null]`)
	body, e := json.Marshal([]any{[]any{raw}})
	if e != nil {
		t.Fatal(e)
	}
	c := Config{Number: "+15555550000", MaxThreads: 30, SyncNumbers: []string{"+15555550123"}}
	rows, e := decodeHistory(body, c)
	if e != nil || len(rows) != 2 {
		t.Fatalf("decode count=%d err=%v", len(rows), e)
	}
	if rows[0].ID == rows[1].ID || nativeBridgeID(c.Number, "native-a") == nativeBridgeID("+15555559999", "native-a") {
		t.Fatal("identity collision")
	}
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	// A confirmed locally submitted message retains the client-assigned ID.
	sent := rows[1]
	sent.ID = "client-assigned"
	if e = store.put(sent); e != nil {
		t.Fatal(e)
	}
	if e = store.mergeImports(rows); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(store.path)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.mergeImports(rows); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(store.path)
	if !reflect.DeepEqual(before, after) || len(store.rows) != 2 {
		t.Fatal("duplicate refresh changed store")
	}
	if _, ok := store.get("client-assigned"); !ok {
		t.Fatal("client identity lost")
	}
	reopened, e := openStore(filepath.Dir(store.path))
	if e != nil || len(reopened.rows) != 2 {
		t.Fatal("restart lost history", e)
	}
	// Failure to commit a refresh must leave memory unchanged too.
	original := store.rows
	store.path = filepath.Join(t.TempDir(), "missing", "messages.json")
	changed := rows[0]
	changed.Preview = "changed"
	if e = store.mergeImports([]Message{changed}); e == nil {
		t.Fatal("write failure ignored")
	}
	if !reflect.DeepEqual(original, store.rows) {
		t.Fatal("failed commit changed memory")
	}
}

func TestHistoryFiltersAndFractionalOrdering(t *testing.T) {
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = store.mergeImports([]Message{{ID: "whole", Sender: "+15555550123", Created: "2026-11-01T15:00:00Z"}, {ID: "fraction", Sender: "+15555550123", Created: "2026-11-01T15:00:00.100Z"}}); e != nil {
		t.Fatal(e)
	}
	rows := store.list(true)
	if rows[0].ID != "fraction" {
		t.Fatal("fractional ordering failed")
	}
	s := &Bridge{config: Config{Username: "test", Password: "private"}, store: store, lastSync: time.Now().UTC().Format(time.RFC3339)}
	req := httptest.NewRequest("GET", "/inbox?from="+url.QueryEscape("2026-11-01T15:00:00.100Z")+"&limit=1", nil)
	req.SetBasicAuth("test", "private")
	r := httptest.NewRecorder()
	s.ServeHTTP(r, req)
	if r.Code != 200 || r.Header().Get("X-Total-Count") != "1" {
		t.Fatal("filtered count", r.Code)
	}
	if e = json.Unmarshal(r.Body.Bytes(), &rows); e != nil || len(rows) != 1 || rows[0].ID != "fraction" {
		t.Fatal("date filter failed", e)
	}
	req = httptest.NewRequest("GET", "/inbox?from=invalid", nil)
	req.SetBasicAuth("test", "private")
	r = httptest.NewRecorder()
	s.ServeHTTP(r, req)
	if r.Code != 400 {
		t.Fatal("invalid date accepted")
	}
}

func TestImportsKeepAccountsDistinct(t *testing.T) {
	store, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	rows := []Message{}
	for _, account := range []string{"+15555550000", "+15555559999"} {
		rows = append(rows, Message{ID: nativeBridgeID(account, "same-native-id"), Account: account, Source: "same-native-id", Sender: "+15555550123", Created: "2026-11-02T00:00:00Z", Preview: "same text"})
	}
	if e = store.mergeImports(rows[:1]); e != nil {
		t.Fatal(e)
	}
	if e = store.mergeImports(rows[1:]); e != nil {
		t.Fatal(e)
	}
	if len(store.rows) != 2 {
		t.Fatal("messages from different GV numbers were merged")
	}
	for _, row := range rows {
		actual, ok := store.get(row.ID)
		if !ok || actual.Account != row.Account {
			t.Fatal("account identity changed")
		}
	}
}
