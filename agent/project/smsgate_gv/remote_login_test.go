package main

import (
	"strings"
	"testing"
)

func TestRemoteLoginArgs(t *testing.T) {
	args := remoteLoginArgs("/private/profile", 9222, "https://voice.google.com/u/0/settings")
	joined := strings.Join(args, " ")
	for _, want := range []string{"--headless=new", "--remote-debugging-address=127.0.0.1", "--remote-debugging-port=9222", "--user-data-dir=/private/profile", "https://voice.google.com/u/0/settings"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
}

func TestFreeLoopbackPort(t *testing.T) {
	port, e := freeLoopbackPort()
	if e != nil || port < 1 || port > 65535 {
		t.Fatalf("port=%d err=%v", port, e)
	}
}
