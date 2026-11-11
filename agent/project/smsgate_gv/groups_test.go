package main

import "testing"

func TestGroupThreadDecoding(t *testing.T) {
	media := `["Paid venmos","",null,[["+15551110000","+15551110000"]],"+15551110000",["+15551110000","+15555550000"]]`
	incoming := `["gm-1",1695421453842,"+15555550000",["Group Message.x","Group Message.x"],10,1,null,null,null,"hello group",null,null,5,1,` + media + `,"Group Message.x",0,null,[null,1],null,null,null,null,null,null,null,null,null,null,"g.Group Message.x"]`
	label := `["gm-2",1695421453843,"+15555550000",["Group Message.x","Group Message.x"],10,1,null,null,null,"+15551110000 - Paid venmos",null,null,5,1,` + media + `,"Group Message.x",0,null,[null,1],null,null,null,null,null,null,null,null,null,null,"g.Group Message.x"]`
	outgoing := `["gm-3",1695421453844,"+15555550000",["Group Message.x","Group Message.x"],11,1,null,null,null,"outgoing text",null,null,5,1,` + media + `,"Group Message.x",0,null,[null,1],null,null,null,null,null,null,null,null,null,null,"g.Group Message.x"]`
	body := `[[["g.Group Message.x",1,[` + incoming + `,` + label + `,` + outgoing + `]]]]`
	rows, e := decodeHistory([]byte(body), Config{Number: "+15555550000", MaxThreads: 1000})
	if e != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), e)
	}
	for _, m := range rows {
		if m.GroupID != "g.Group Message.x" || len(m.Participants) != 2 || m.Type != "MMS" {
			t.Fatalf("group metadata missing: %+v", m)
		}
	}
	in := rows[0]
	if in.Sender != "+15551110000" || in.Preview != "hello group" {
		t.Fatalf("incoming group message: %+v", in)
	}
	out := rows[1]
	if out.Sender != "" || out.Body == nil || out.Body.Text != "outgoing text" || len(out.Recipients) != 1 || out.Recipients[0].Number != "+15551110000" {
		t.Fatalf("outgoing group message: %+v", out)
	}
}
