package script

import (
	"os"
	"path/filepath"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestJamfAuthTokenPath(t *testing.T) {
	blocked := []string{
		"/api/v1/auth/token",
		"/api/v1/auth/keep-alive",
		"/api/oauth/token",
		"/auth/token",
		"/oauth2/token",
		"https://example.jamfcloud.com/api/v1/auth/token?x=1",
	}
	for _, p := range blocked {
		if !jamfAuthTokenPath(p) {
			t.Errorf("expected blocked: %q", p)
		}
	}
	if jamfAuthTokenPath("/api/v1/packages") {
		t.Error("packages path should not be blocked")
	}
}

func TestBuddySecretPathAndMCPGuards(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	cfgDir := filepath.Join(xdg, "buddy")
	if err := os.MkdirAll(filepath.Join(cfgDir, "tokens"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "secrets"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.yaml")
	tok := filepath.Join(cfgDir, "tokens", "x.json")
	sec := filepath.Join(cfgDir, "secrets", "a.json")
	if err := os.WriteFile(cfg, []byte("tenant:\n  default: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tok, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sec, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{cfg, tok, sec} {
		if !buddySecretPath(p) {
			t.Errorf("expected secret path blocked: %q", p)
		}
	}
	other := filepath.Join(t.TempDir(), "ok.txt")
	if err := os.WriteFile(other, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if buddySecretPath(other) {
		t.Errorf("temp file should not be blocked: %q", other)
	}

	prev := buddy.ActiveChannel()
	buddy.SetActiveChannel(buddy.ChannelMCP)
	t.Cleanup(func() { buddy.SetActiveChannel(prev) })

	dict := makeBuiltins("", nil, false)
	th := &starlark.Thread{Name: "t"}
	readFn := dict["file_read"].(*starlark.Builtin)
	rval, err := starlark.Call(th, readFn, starlark.Tuple{starlark.String(cfg)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, _, _ := rval.(*starlark.Dict).Get(starlark.String("ok"))
	if ok != starlark.False {
		t.Fatalf("expected file_read blocked on MCP, got %v", rval)
	}

	v, err := doJamfRequest(th, "", nil, "GET", "/api/v1/auth/token", starlark.None, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	ok, _, _ = v.(*starlark.Dict).Get(starlark.String("ok"))
	if ok != starlark.False {
		t.Fatalf("expected jamf token path blocked, got %v", v)
	}
}
