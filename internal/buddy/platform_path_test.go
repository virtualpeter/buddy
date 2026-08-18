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
	want := "/api/proclassic/tenant/tid-1/policies/id/42"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_ClassicExactPrefix(t *testing.T) {
	got, err := RewriteJamfPath("tid-1", "/JSSResource")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/proclassic/tenant/tid-1/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_ClassicQuery(t *testing.T) {
	got, err := RewriteJamfPath("tid-1", "/JSSResource/policies?foo=1")
	if err != nil {
		t.Fatal(err)
	}
	want := "/api/proclassic/tenant/tid-1/policies?foo=1"
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

func TestRewriteJamfPath_PlatformProducts(t *testing.T) {
	tid := "tid-1"
	cases := []struct {
		in, want string
	}{
		{"/api/devices/v1/devices", "/api/devices/v1/tenant/tid-1/devices"},
		{"/api/device-groups/v1/device-groups", "/api/device-groups/v1/tenant/tid-1/device-groups"},
		{"/api/device-actions/v1/devices/x/restart", "/api/device-actions/v1/tenant/tid-1/devices/x/restart"},
		{"/api/compliance-benchmarks/v1/benchmarks", "/api/compliance-benchmarks/v1/tenant/tid-1/benchmarks"},
		{"/api/ddm/report/v1/devices/x/declarations", "/api/ddm/report/v1/tenant/tid-1/devices/x/declarations"},
		{"/api/devices/v1/devices?page=0", "/api/devices/v1/tenant/tid-1/devices?page=0"},
	}
	for _, tc := range cases {
		got, err := RewriteJamfPath(tid, tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestOpClassForPath_PlatformProducts(t *testing.T) {
	if got := opClassForPath("GET", "/api/devices/v1/devices"); got != OpPlatform {
		t.Fatalf("GET devices → %q", got)
	}
	if got := opClassForPath("POST", "/api/device-actions/v1/devices/x/restart"); got != OpPlatformWrite {
		t.Fatalf("POST device-actions → %q", got)
	}
	if got := opClassForPath("GET", "/api/ddm/report/v1/devices/x/declarations"); got != OpPlatform {
		t.Fatalf("GET ddm/report → %q", got)
	}
}

func TestOpClassForPath_Classic(t *testing.T) {
	if got := opClassForPath("GET", "/JSSResource/policies"); got != OpClassic {
		t.Fatalf("GET Classic → %q", got)
	}
	if got := opClassForPath("POST", "/JSSResource/policies/id/0"); got != OpClassicWrite {
		t.Fatalf("POST Classic → %q want %q", got, OpClassicWrite)
	}
	if got := opClassForPath("PUT", "/api/proclassic/tenant/tid/policies/id/1"); got != OpClassicWrite {
		t.Fatalf("PUT proclassic → %q", got)
	}
	if got := opClassForPath("GET", "/api/proclassic/tenant/tid/JSSResource/policies/id/1"); got != OpClassic {
		t.Fatalf("GET legacy proclassic+JSSResource → %q", got)
	}
}

func TestOpClassForPath_ACS(t *testing.T) {
	if got := opClassForPath("GET", "/api/config-profiles/macos/profile/1"); got != OpACS {
		t.Fatalf("GET ACS → %q want %q", got, OpACS)
	}
	if got := opClassForPath("POST", "/api/config-profiles/plist/can-convert-to-plist"); got != OpACSWrite {
		t.Fatalf("POST ACS plist → %q want %q", got, OpACSWrite)
	}
	if got := opClassForPath("PUT", "/api/config-profiles/macos/abc"); got != OpACSWrite {
		t.Fatalf("PUT ACS → %q want %q", got, OpACSWrite)
	}
}

func TestOpClassForPath_ProPoliciesList(t *testing.T) {
	if got := opClassForPath("GET", "/api/v1/policies"); got != OpProUI {
		t.Fatalf("GET policy list → %q want %q", got, OpProUI)
	}
	if got := opClassForPath("GET", "/api/v1/packages"); got != OpRead {
		t.Fatalf("GET packages must stay published Pro OpRead, got %q", got)
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

func TestRewriteJamfPath_ACSRejected(t *testing.T) {
	_, err := RewriteJamfPath("tid", "/api/config-profiles/plist/can-convert-to-plist")
	if err == nil || !strings.Contains(err.Error(), "ACS is instance-only") {
		t.Fatalf("expected ACS instance-only error, got %v", err)
	}
	got, err := RewriteJamfPath("", "/api/config-profiles/macos/x")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/config-profiles/macos/x" {
		t.Fatalf("instance rewrite should leave ACS path, got %q", got)
	}
}

func TestRewriteJamfPath_ProPoliciesListRejected(t *testing.T) {
	_, err := RewriteJamfPath("tid", "/api/v1/policies")
	if err == nil || !strings.Contains(err.Error(), "undocumented Pro policy list is instance-only") {
		t.Fatalf("expected policy-list instance-only error, got %v", err)
	}
	got, err := RewriteJamfPath("", "/api/v1/policies")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/v1/policies" {
		t.Fatalf("instance rewrite should leave policy list path, got %q", got)
	}
}

func TestDefaultJamfMedia_Proclassic(t *testing.T) {
	ct, accept := DefaultJamfMedia("/api/proclassic/tenant/x/policies")
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
		Tenant: TenantSelection{Default: "gw"},
		Tenants: map[string]TenantConfig{
			"gw": {
				URL:      "https://us.apigw.jamf.com",
				TenantID: "tid",
			},
		},
		Auths: map[string]AuthEntry{
			"gw": {Type: AuthTypePlatform, ClientID: "c", Tenants: []string{"gw"}},
		},
	}
	normalizeConfigForSave(cfg)
	c := cfg.Tenants["gw"]
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
