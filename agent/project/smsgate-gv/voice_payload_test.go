package main

import (
	"encoding/json"
	"testing"
)

func fixtureThread(records ...string) []json.RawMessage {
	body := `[[["t.+15555550123",1,[`
	for i, r := range records {
		if i > 0 {
			body += ","
		}
		body += r
	}
	body += `]]]]`
	thread, e := voiceThread([]byte(body), "t.+15555550123")
	if e != nil {
		panic(e)
	}
	return thread
}

func TestVoiceNativeIdentity(t *testing.T) {
	a := `["native-a",1793545877539,"+15555550000",[],10,1,null,null,null,"same",null,null,5,1,null]`
	b := `["native-b",1793545877539,"+15555550000",[],11,1,null,null,null,"same",null,null,6,1,null]`
	rows, e := decodeVoiceThread(fixtureThread(a, b), "+15555550000")
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 || rows[0].ID == rows[1].ID || rows[0].Outgoing || !rows[1].Outgoing || rows[0].Timestamp != "2026-11-01T15:11:17.539Z" {
		t.Fatalf("identity/time/direction incorrect: %+v", rows)
	}
	if _, e = decodeVoiceThread(fixtureThread(a, a), "+15555550000"); e == nil {
		t.Fatal("duplicate native ID accepted")
	}
	if _, e = decodeVoiceThread(fixtureThread(a), "+15555559999"); e == nil {
		t.Fatal("wrong account accepted")
	}
	if _, e = voiceThread([]byte(`[[["t.other",1,[]]]]`), "t.+15555550123"); e == nil {
		t.Fatal("wrong conversation accepted")
	}
	for _, bad := range []string{`["",1793545877539,"+15555550000",[],10,1,null,null,null,"same",null,null,5,1,null]`, `["id",0,"+15555550000",[],10,1,null,null,null,"same",null,null,5,1,null]`, `["id",1793545877539,"+15555550000",[],99,1,null,null,null,"same",null,null,5,1,null]`} {
		if _, e = decodeVoiceThread(fixtureThread(bad), "+15555550000"); e == nil {
			t.Fatal("malformed message accepted")
		}
	}
}
