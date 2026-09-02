package buddy

import (
	"strings"
	"testing"
)

func TestResolveCredential_NilCredentials(t *testing.T) {
	cfg := &Config{Credentials: nil}
	_, err := cfg.ResolveCredential("default")
	if err == nil {
		t.Fatal("expected error when Credentials is nil")
	}
	if !strings.Contains(err.Error(), "no credentials configured") {
		t.Errorf("error = %v", err)
	}
}

func TestResolveCredential_Missing(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]CredentialConfig{
			"default": {URL: "https://jamf.example.com", Auth: AuthConfig{Type: "basic"}},
		},
	}
	_, err := cfg.ResolveCredential("missing")
	if err == nil {
		t.Fatal("expected error when credential name not found")
	}
	if !strings.Contains(err.Error(), "credential \"missing\" not found") {
		t.Errorf("error = %v", err)
	}
}

func TestResolveCredential_Found(t *testing.T) {
	want := CredentialConfig{
		URL: "https://jamf.example.com",
		Auth: AuthConfig{
			Type:     "basic",
			Username: "user",
			Password: "secret",
		},
	}
	cfg := &Config{
		Credentials: map[string]CredentialConfig{
			"default": want,
		},
	}
	got, err := cfg.ResolveCredential("default")
	if err != nil {
		t.Fatalf("ResolveCredential: %v", err)
	}
	if got.URL != want.URL || got.Auth.Username != want.Auth.Username || got.Auth.Password != want.Auth.Password {
		t.Errorf("got URL=%q Username=%q Password=%q, want URL=%q Username=%q Password=%q",
			got.URL, got.Auth.Username, got.Auth.Password,
			want.URL, want.Auth.Username, want.Auth.Password)
	}
}
