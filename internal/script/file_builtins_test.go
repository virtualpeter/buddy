package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestFileReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")

	dict := makeBuiltins("", nil, false)
	writeFn := dict["file_write"].(*starlark.Builtin)
	readFn := dict["file_read"].(*starlark.Builtin)
	readBFn := dict["file_read_bytes"].(*starlark.Builtin)

	th := &starlark.Thread{Name: "t"}
	wval, err := starlark.Call(th, writeFn, starlark.Tuple{starlark.String(p), starlark.String("hello 世界")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wval != starlark.None {
		t.Fatalf("file_write want None, got %v", wval)
	}

	rval, err := starlark.Call(th, readFn, starlark.Tuple{starlark.String(p)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(rval.(starlark.String)) != "hello 世界" {
		t.Fatalf("got %v", rval)
	}

	bval, err := starlark.Call(th, readBFn, starlark.Tuple{starlark.String(p)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(bval.(starlark.Bytes)) != "hello 世界" {
		t.Fatalf("bytes: got %q", bval)
	}
}

func TestFileReadRejectsNonUTF8(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin")
	if err := os.WriteFile(p, []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}

	dict := makeBuiltins("", nil, false)
	readFn := dict["file_read"].(*starlark.Builtin)
	th := &starlark.Thread{Name: "t"}
	_, err := starlark.Call(th, readFn, starlark.Tuple{starlark.String(p)}, nil)
	if err == nil {
		t.Fatal("expected error for non-UTF-8")
	}
	if !strings.Contains(err.Error(), "utf-8") && !strings.Contains(err.Error(), "UTF-8") && !strings.Contains(strings.ToLower(err.Error()), "utf") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileWriteBytes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.bin")

	dict := makeBuiltins("", nil, false)
	writeFn := dict["file_write"].(*starlark.Builtin)
	th := &starlark.Thread{Name: "t"}
	payload := starlark.Bytes([]byte{0, 1, 255})
	wval, err := starlark.Call(th, writeFn, starlark.Tuple{starlark.String(p), payload}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wval != starlark.None {
		t.Fatalf("file_write want None, got %v", wval)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %v want %v", got, []byte(payload))
	}
}
