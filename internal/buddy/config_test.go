package buddy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestMarshalConfig_OmitsEmptyAuthFields(t *testing.T) {
	cfg := &Config{
		Cred: "oauth1",
		Credentials: map[string]CredentialConfig{
			"oauth1": {
				URL: "https://example.jamfcloud.com",
				Auth: AuthConfig{
					Type:     "oauth",
					ClientID: "cid",
				},
			},
			"basic1": {
				URL: "https://example.jamfcloud.com",
				Auth: AuthConfig{
					Type:     "basic",
					Username: "admin",
				},
			},
		},
	}
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, bad := range []string{"runCommand", "password:", "clientSecret:", `username: ""`} {
		if strings.Contains(s, bad) {
			t.Fatalf("marshal should omit %q, got:\n%s", bad, s)
		}
	}
	if !strings.Contains(s, "clientId: cid") {
		t.Fatalf("missing clientId:\n%s", s)
	}
	if !strings.Contains(s, "username: admin") {
		t.Fatalf("missing username:\n%s", s)
	}

	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Credentials["oauth1"].Auth.ClientID != "cid" || got.Credentials["oauth1"].Auth.Type != "oauth" {
		t.Fatalf("round-trip oauth1 = %+v", got.Credentials["oauth1"])
	}
	if got.Credentials["basic1"].Auth.Username != "admin" {
		t.Fatalf("round-trip basic1 = %+v", got.Credentials["basic1"])
	}
}

func TestMarshalConfig_PreservesCredentialOrder(t *testing.T) {
	existing := []byte(`cred: z
credentials:
    zebra:
        url: https://z.example.com
        auth:
            type: oauth
            clientId: z
    alpha:
        url: https://a.example.com
        auth:
            type: oauth
            clientId: a
`)
	cfg := &Config{
		Cred: "zebra",
		Credentials: map[string]CredentialConfig{
			"alpha": {URL: "https://a.example.com", Auth: AuthConfig{Type: "oauth", ClientID: "a"}},
			"zebra": {URL: "https://z.example.com", Auth: AuthConfig{Type: "oauth", ClientID: "z"}},
			"beta":  {URL: "https://b.example.com", Auth: AuthConfig{Type: "oauth", ClientID: "b"}},
		},
	}
	data, err := marshalConfig(cfg, existing)
	if err != nil {
		t.Fatal(err)
	}
	order := credentialNamesInYAMLOrder(data)
	want := []string{"zebra", "alpha", "beta"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v\n%s", order, want, data)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v\n%s", order, want, data)
		}
	}
}

func TestMarshalConfig_MigratesAuthTenantID(t *testing.T) {
	cfg := &Config{
		Cred: "gw",
		Credentials: map[string]CredentialConfig{
			"gw": {
				URL: "https://us.apigw.jamf.com",
				Auth: AuthConfig{
					Type:     "oauth",
					ClientID: "cid",
					TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
				},
			},
		},
	}
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "tenantId: e77c1408-10c8-4007-b177-abc9157fbcaa") {
		t.Fatalf("expected root tenantId:\n%s", s)
	}
	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Credentials["gw"].TenantID != "e77c1408-10c8-4007-b177-abc9157fbcaa" {
		t.Fatalf("root TenantID = %q", got.Credentials["gw"].TenantID)
	}
	if got.Credentials["gw"].Auth.TenantID != "" {
		t.Fatalf("auth.TenantID should be empty after save, got %q", got.Credentials["gw"].Auth.TenantID)
	}
}
