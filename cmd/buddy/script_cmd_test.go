package main

import (
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestScriptUploadAndUpdateAreTheSameCommand(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"upload", "update", "upl", "upd"} {
		err := runJamfScript("", buddy.Opts{}, []string{sub}, nil)
		if err == nil {
			t.Fatalf("%s: expected a missing-argument error", sub)
		}
		msg := err.Error()
		if strings.Contains(msg, "unknown script subcommand") || strings.Contains(msg, "Did you mean") {
			t.Fatalf("%s: %s", sub, msg)
		}
		if !strings.Contains(msg, "script-id or file") {
			t.Fatalf("%s: %s", sub, msg)
		}
	}
}
