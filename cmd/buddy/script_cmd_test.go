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
		if !strings.Contains(msg, "file") {
			t.Fatalf("%s: %s", sub, msg)
		}
	}
}

func TestScriptCreateFileMustBeDownloadName(t *testing.T) {
	t.Parallel()
	err := runJamfScript("", buddy.Opts{}, []string{"create", "My Script.sh"}, nil)
	if err == nil || !strings.Contains(err.Error(), "not a download name") {
		t.Fatalf("create: %v", err)
	}
}

func TestScriptCheckAndRenameAreCommands(t *testing.T) {
	t.Parallel()
	err := runJamfScript("", buddy.Opts{}, []string{"check"}, nil)
	if err == nil || !strings.Contains(err.Error(), "file") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("check: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"rename"}, nil)
	if err == nil || !strings.Contains(err.Error(), "script or new name") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("rename: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"c"}, nil)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("c: %v", err)
	}
}

func TestScriptPutCloneCopyMigrateAndPageAreCommands(t *testing.T) {
	t.Parallel()
	err := runJamfScript("", buddy.Opts{}, []string{"put"}, nil)
	if err == nil || !strings.Contains(err.Error(), "script or json file") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("put: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"clone"}, nil)
	if err == nil || !strings.Contains(err.Error(), "script or name") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("clone: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"copy", "7"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--name=") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("copy: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"migrate"}, nil)
	if err == nil || !strings.Contains(err.Error(), "script") || strings.Contains(err.Error(), "unknown script subcommand") {
		t.Fatalf("migrate: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"list", "--page=nope"}, nil)
	if err == nil || !strings.Contains(err.Error(), "--page") || strings.Contains(err.Error(), "unknown") {
		t.Fatalf("page: %v", err)
	}
	err = runJamfScript("", buddy.Opts{}, []string{"create", "--os-requirements=10.15", "--parameter4=p4"}, nil)
	if err == nil || !strings.Contains(err.Error(), "name or file") {
		t.Fatalf("create flags: %v", err)
	}
}
