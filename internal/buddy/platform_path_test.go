package buddy

import (
	"strings"
	"testing"
)

func TestRewriteJamfPath_Identity(t *testing.T) {
	got, err := RewriteJamfPath("", "/api/v1/packages")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/v1/packages" {
		t.Fatalf("got %q", got)
	}
}

func TestRewriteJamfPath_Pro(t *testing.T) {
	tid := "e77c1408-10c8-4007-b177-abc9157fbcaa"
	got, err := RewriteJamfPath(tid, "/api/v3/computer-groups/smart-groups")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/pro/v3/tenant/" + tid + "/computer-groups/smart-groups"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_Classic(t *testing.T) {
	tid := "tid-1"
	got, err := RewriteJamfPath(tid, "/JSSResource/policies/id/42")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/proclassic/tenant/tid-1/JSSResource/policies/id/42"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_Blueprints(t *testing.T) {
	tid := "tid-1"
	got, err := RewriteJamfPath(tid, "/api/blueprints/v1/blueprints")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/blueprints/v1/tenant/tid-1/blueprints"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_AlreadyScoped(t *testing.T) {
	path := "/api/pro/v1/tenant/tid-1/buildings"
	got, err := RewriteJamfPath("tid-1", path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q", got)
	}
}

func TestRewriteJamfPath_QueryPreserved(t *testing.T) {
	got, err := RewriteJamfPath("tid", "/api/v1/packages/1/history?sort=date:desc&page-size=1")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/pro/v1/tenant/tid/packages/1/history?sort=date:desc&page-size=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_SchemaRejected(t *testing.T) {
	_, err := RewriteJamfPath("tid", "/api/schema")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected not supported error, got %v", err)
	}
}

func TestDefaultJamfMedia_Proclassic(t *testing.T) {
	ct, accept := DefaultJamfMedia("/api/proclassic/tenant/x/JSSResource/policies")
	if ct != "application/xml" || accept != "application/xml" {
		t.Fatalf("got ct=%q accept=%q", ct, accept)
	}
}

func TestCredentialValidate_PlatformRequiresOAuth(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.apigw.jamf.com",
		TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
		Auth:     AuthConfig{Type: "basic", Username: "u"},
	}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "oauth") {
		t.Fatalf("expected oauth error, got %v", err)
	}
}

func TestCredentialValidate_GatewayURLRequiresTenantID(t *testing.T) {
	cred := CredentialConfig{
		URL:  "https://us.apigw.jamf.com",
		Auth: AuthConfig{Type: "oauth", ClientID: "c"},
	}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "tenantId") {
		t.Fatalf("expected tenantId error, got %v", err)
	}
}

func TestCredentialValidate_PlatformOAuthOK(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.apigw.jamf.com",
		TenantID: "example-opaque-tenant", // opaque string, not necessarily UUID
		Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialValidate_RegionWithoutURL(t *testing.T) {
	cred := CredentialConfig{
		Region:   "us",
		TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
		Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := cred.EffectiveBaseURL(); got != "https://us.apigw.jamf.com" {
		t.Fatalf("EffectiveBaseURL = %q", got)
	}
}

func TestCredentialValidate_InstanceRequiresURL(t *testing.T) {
	cred := CredentialConfig{Auth: AuthConfig{Type: "oauth", ClientID: "c"}}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Fatalf("expected url required, got %v", err)
	}
}

func TestNormalizeConfigForSave_InfersRegionDropsDefaultURL(t *testing.T) {
	cfg := &Config{
		Cred: "gw",
		Credentials: map[string]CredentialConfig{
			"gw": {
				URL:      "https://us.apigw.jamf.com",
				TenantID: "tid",
				Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
			},
		},
	}
	normalizeConfigForSave(cfg)
	c := cfg.Credentials["gw"]
	if c.Region != "us" {
		t.Fatalf("Region = %q", c.Region)
	}
	if c.URL != "" {
		t.Fatalf("default gateway URL should be omitted, got %q", c.URL)
	}
}

func TestCredentialValidate_AcceptsNonUUIDTenant(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.apigw.jamf.com",
		TenantID: "my-tenant-slug",
		Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatalf("opaque tenantId should be allowed: %v", err)
	}
}

func TestCredentialIsPlatform(t *testing.T) {
	if (CredentialConfig{}).IsPlatform() {
		t.Fatal("empty should not be platform")
	}
	if !(CredentialConfig{TenantID: "x"}).IsPlatform() {
		t.Fatal("tenantId should enable platform")
	}
	if !(CredentialConfig{Region: "us"}).IsPlatform() {
		t.Fatal("region should enable platform")
	}
}

func TestLooksLikePlatformGateway(t *testing.T) {
	if !LooksLikePlatformGateway("https://us.apigw.jamf.com") {
		t.Fatal("expected true")
	}
	if LooksLikePlatformGateway("https://example.jamfcloud.com") {
		t.Fatal("expected false")
	}
}

func TestUsesPlatformAuth(t *testing.T) {
	gwOnly := CredentialConfig{URL: "https://apac.apigw.jamf.com", Auth: AuthConfig{Type: "oauth"}}
	if !gwOnly.UsesPlatformAuth() {
		t.Fatal("apigw URL should use platform auth")
	}
	inst := CredentialConfig{URL: "https://x.jamfcloud.com", Auth: AuthConfig{Type: "oauth"}}
	if inst.UsesPlatformAuth() {
		t.Fatal("instance URL should not use platform auth")
	}
}

func TestEffectiveTenantID_FromAuthMisplaced(t *testing.T) {
	cred := CredentialConfig{
		URL: "https://us.apigw.jamf.com",
		Auth: AuthConfig{
			Type:     "oauth",
			ClientID: "c",
			TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
		},
	}
	if !cred.IsPlatform() {
		t.Fatal("misplaced auth.tenantId should enable platform")
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := cred.EffectiveTenantID(); got != "e77c1408-10c8-4007-b177-abc9157fbcaa" {
		t.Fatalf("got %q", got)
	}
}
