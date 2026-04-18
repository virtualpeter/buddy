package script

import (
	"os"
	"path/filepath"
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
	ok, _, _ := wval.(*starlark.Dict).Get(starlark.String("ok"))
	if ok != starlark.True {
		t.Fatalf("file_write: %v", wval)
	}

	rval, err := starlark.Call(th, readFn, starlark.Tuple{starlark.String(p)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rd := rval.(*starlark.Dict)
	ok, _, _ = rd.Get(starlark.String("ok"))
	if ok != starlark.True {
		t.Fatalf("file_read: %v", rval)
	}
	v, _, _ := rd.Get(starlark.String("value"))
	if string(v.(starlark.String)) != "hello 世界" {
		t.Fatalf("got %v", v)
	}

	bval, err := starlark.Call(th, readBFn, starlark.Tuple{starlark.String(p)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bd := bval.(*starlark.Dict)
	v2, _, _ := bd.Get(starlark.String("value"))
	if string(v2.(starlark.Bytes)) != "hello 世界" {
		t.Fatalf("bytes: got %q", v2)
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
	rval, err := starlark.Call(th, readFn, starlark.Tuple{starlark.String(p)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rd := rval.(*starlark.Dict)
	ok, _, _ := rd.Get(starlark.String("ok"))
	if ok != starlark.False {
		t.Fatalf("expected ok false, got %v", rval)
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
	ok, _, _ := wval.(*starlark.Dict).Get(starlark.String("ok"))
	if ok != starlark.True {
		t.Fatalf("file_write: %v", wval)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %v want %v", got, []byte(payload))
	}
}
