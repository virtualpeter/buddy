package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveIDEPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BUDDY_IDE_ROOT", dir)
	sub := filepath.Join(dir, "examples")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(sub, "a.buddy")
	if err := os.WriteFile(f, []byte("x=1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIDEPath("examples/a.buddy")
	if err != nil {
		t.Fatal(err)
	}
	if got != f {
		t.Fatalf("got %q want %q", got, f)
	}

	if _, err := resolveIDEPath("../outside.buddy"); err == nil {
		t.Fatal("expected outside path rejected")
	}
}
