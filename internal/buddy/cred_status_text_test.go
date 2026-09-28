package buddy

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"buddy/internal/buddy/secrets"
)

func TestCredStatusHeaderFileStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "file")
	t.Setenv("BUDDY_USE_AGENT", "0")

	var b strings.Builder
	if err := WriteCredStatusHeader(&b, &Config{}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	root := filepath.Join(dir, "buddy")
	for _, want := range []string{
		filepath.Join(root, "config.yaml"),
		filepath.Join(root, "secrets.enc"),
		filepath.Join(root, "dek"),
		filepath.Join(root, "tokens"),
		"secrets: files",
		"recovery:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if runtime.GOOS != "windows" && !strings.Contains(got, "mode 0600") {
		t.Fatalf("missing file mode:\n%s", got)
	}
	if strings.Contains(got, "Buddy Agent") {
		t.Fatalf("file store should not describe the Agent:\n%s", got)
	}
}

func TestCredStatusHeaderPassStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "pass")
	t.Setenv("BUDDY_USE_AGENT", "0")

	var b strings.Builder
	if err := WriteCredStatusHeader(&b, &Config{}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.Contains(got, "secrets: pass") || !strings.Contains(got, "pass show "+secrets.PassDEKEntry()) {
		t.Fatalf("pass store:\n%s", got)
	}
}

func TestCredStatusHeaderKeychain(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain backend is macOS")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "")
	t.Setenv("BUDDY_USE_AGENT", "0")

	var b strings.Builder
	if err := WriteCredStatusHeader(&b, &Config{}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	item, ok := secrets.KeychainDEKItem()
	if !ok {
		t.Fatal("expected a keychain item")
	}
	if !strings.Contains(got, item.Service) || !strings.Contains(got, item.Account) {
		t.Fatalf("missing keychain item in:\n%s", got)
	}
	switch item.Access {
	case "cli":
		if !strings.Contains(got, "unsigned binary") || !strings.Contains(got, "/usr/bin/security") {
			t.Fatalf("unsigned keychain:\n%s", got)
		}
	case "framework":
		if !strings.Contains(got, "Security.framework") {
			t.Fatalf("signed keychain:\n%s", got)
		}
	default:
		t.Fatalf("access %q:\n%s", item.Access, got)
	}
}

func TestCredStatusHeaderAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "file")
	t.Setenv("BUDDY_USE_AGENT", "1")

	cfg := &Config{
		agent: &Config{
			Tenants: map[string]TenantConfig{"sandbox": {}},
			Auths:   map[string]AuthEntry{"sandbox-agent": {}},
		},
		Auths: map[string]AuthEntry{"sandbox-agent": {}},
	}
	var b strings.Builder
	if err := WriteCredStatusHeader(&b, cfg); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"agent-config.json",
		"virtualpete.buddy.agent",
		"agent.sock",
		"not written",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secrets.enc") {
		t.Fatalf("agent-only config should not describe the local vault:\n%s", got)
	}
}

func TestCredStatusHeaderAgentPlusLocalVault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "file")
	t.Setenv("BUDDY_USE_AGENT", "1")

	cfg := &Config{
		agent: &Config{
			Auths: map[string]AuthEntry{"sandbox-agent": {}},
		},
		Auths: map[string]AuthEntry{
			"sandbox-agent": {},
			"local":         {},
		},
	}
	var b strings.Builder
	if err := WriteCredStatusHeader(&b, cfg); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.Contains(got, "virtualpete.buddy.agent") || !strings.Contains(got, "secrets.enc") || !strings.Contains(got, "auths in config.yaml") {
		t.Fatalf("mixed stores:\n%s", got)
	}
}
