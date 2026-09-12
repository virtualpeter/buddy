package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHelperPathFromMacOSExecutable(t *testing.T) {
	root := t.TempDir()
	macos := filepath.Join(root, "Contents", "MacOS")
	helpers := filepath.Join(root, "Contents", "Helpers")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(helpers, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macos, "Buddy")
	want := filepath.Join(helpers, "buddy")
	if err := os.WriteFile(exe, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := helperPath(exe)
	if err != nil {
		t.Fatal(err)
	}
	got, err = filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("helperPath = %q, want %q", got, want)
	}
}
