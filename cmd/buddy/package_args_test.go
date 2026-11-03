package main

import (
	"testing"
	"time"
)

func TestParsePackageUploadWaitReadyDefault(t *testing.T) {
	t.Parallel()
	_, _, opts, err := parsePackageUploadArgs([]string{"Chrome-121.pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.WaitReady {
		t.Fatal("omitted --wait-ready should wait")
	}
	if opts.WaitReadyTimeout != 0 {
		t.Fatalf("timeout = %v, want 0 (default)", opts.WaitReadyTimeout)
	}

	_, _, opts, err = parsePackageUploadArgs([]string{"Chrome-121.pkg", "--wait-ready=false"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.WaitReady {
		t.Fatal("--wait-ready=false should not poll after upload")
	}

	_, _, opts, err = parsePackageUploadArgs([]string{"--wait-ready=90", "Chrome-121.pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.WaitReady || opts.WaitReadyTimeout != 90*time.Second {
		t.Fatalf("opts = %+v", opts)
	}
}
