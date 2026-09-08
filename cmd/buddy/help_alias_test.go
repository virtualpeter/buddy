package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintCommandHelpResolvesEAAlias(t *testing.T) {
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	printCommandHelp([]string{"ea"})
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	out, _ := io.ReadAll(rOut)
	errb, _ := io.ReadAll(rErr)
	if strings.Contains(string(errb), "No help") || strings.Contains(string(errb), "Did you mean") {
		t.Fatalf("help ea should resolve: stderr=%s", errb)
	}
	if !strings.Contains(string(out), "extension-attribute") {
		t.Fatalf("help ea stdout missing extension-attribute:\n%s", out)
	}
	if !strings.Contains(string(out), "Aliases: ea, eas") {
		t.Fatalf("help ea stdout missing aliases:\n%s", out)
	}
}
