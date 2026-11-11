package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed scripts/group_mms.py
var groupMMSScript string

func (b *Browser) sendGroupMMS(m Message, store *Store) (result Message) {
	result = m
	result.Submitted = true
	result.AttemptedAt = now()
	setState(&result, "Uncertain", "group MMS handed to helper")
	if e := store.put(result); e != nil {
		setState(&result, "Failed", "cannot persist group MMS journal; no send attempted")
		return
	}
	files, cleanup, e := writeOutboxFiles(b.home, m)
	if e != nil {
		setState(&result, "Failed", "cannot prepare group attachments; no send attempted")
		return
	}
	defer cleanup()
	script, e := os.CreateTemp("", "smsgate_gv-group_mms-*.py")
	if e != nil {
		setState(&result, "Failed", "cannot create group MMS helper")
		return
	}
	defer os.Remove(script.Name())
	if _, e = script.WriteString(groupMMSScript); e == nil {
		e = script.Chmod(0600)
	}
	if ce := script.Close(); e == nil {
		e = ce
	}
	if e != nil {
		setState(&result, "Failed", "cannot write group MMS helper")
		return
	}
	recipients := make([]string, 0, len(m.Recipients))
	for _, r := range m.Recipients {
		recipients = append(recipients, r.Number)
	}
	text := ""
	if m.Body != nil {
		text = m.Body.Text
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", script.Name(),
		"--home", b.home,
		"--recipients", strings.Join(recipients, ","),
		"--text", text,
		"--files", strings.Join(files, ","))
	output, e := cmd.Output()
	if e != nil {
		setState(&result, "Failed", "group MMS helper failed")
		_ = store.put(result)
		return
	}
	var response struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		Thread string `json:"thread"`
		Source string `json:"source"`
	}
	if json.Unmarshal(output, &response) != nil || !response.OK {
		setState(&result, "Failed", "group MMS helper reported failure")
		_ = store.put(result)
		return
	}
	result.Created = now()
	result.Source = response.Source
	if strings.HasPrefix(response.Thread, "g.") {
		result.GroupID = response.Thread
	}
	setState(&result, "Sent", "")
	if e := store.put(result); e != nil {
		result.Error = fmt.Sprintf("sent but journal update failed: %v", e)
	}
	return
}
