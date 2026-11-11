package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDefaultEditorRequiresConfirmation(t *testing.T) {
	// A pipe must never launch an unattended GUI or save before confirmation.
	t.Setenv("EDITOR", "missing-editor.exe")
	for _, format := range []string{"yaml", "vcard"} {
		var output bytes.Buffer
		r := Runner{Out: &output, Err: &output}
		original := []byte("private draft")
		_, path, err := r.editor(original, Config{}, format)
		if path != "" {
			defer os.Remove(path)
		}
		if err == nil || !strings.Contains(err.Error(), "requires interactive input") {
			t.Fatalf("expected confirmation requirement, got %v", err)
		}
		if !strings.HasSuffix(path, ".txt") {
			t.Fatalf("unsafe file association: %s", path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(data, original) {
			t.Fatalf("lost recovery draft: %v", readErr)
		}
	}
}

func TestDefaultEditorReportsOpenFailure(t *testing.T) {
	if err := openDefaultEditor("invalid\x00path"); err == nil {
		t.Fatal("accepted invalid Windows path")
	}
}
