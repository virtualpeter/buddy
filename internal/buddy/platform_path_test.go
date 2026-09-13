package buddy

import (
	"strings"
	"testing"
)

func TestRewriteJamfPath_Identity(t *testing.T) {
	got, err := RewriteJamfPath(false, "/api/v1/packages")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/v1/packages" {
		t.Fatalf("got %q", got)
	}
}

func TestRewriteJamfPath_Pro(t *testing.T) {
	got, err := RewriteJamfPath(true, "/api/v3/computer-groups/smart-groups")
	if err != nil {
		t.Fatal(err)
	}
	want := "/pro/v3/computer-groups/smart-groups"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_Classic(t *testing.T) {
	got, err := RewriteJamfPath(true, "/JSSResource/policies/id/42")
	if err != nil {
		t.Fatal(err)
	}
	want := "/proclassic/policies/id/42"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_ClassicExactPrefix(t *testing.T) {
	got, err := RewriteJamfPath(true, "/JSSResource")
	if err != nil {
		t.Fatal(err)
	}
	want := "/proclassic/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_ClassicQuery(t *testing.T) {
	got, err := RewriteJamfPath(true, "/JSSResource/policies?foo=1")
	if err != nil {
		t.Fatal(err)
	}
	want := "/proclassic/policies?foo=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_Blueprints(t *testing.T) {
	got, err := RewriteJamfPath(true, "/api/blueprints/v1/blueprints")
	if err != nil {
		t.Fatal(err)
	}
	want := "/blueprints/v1/blueprints"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_PlatformProducts(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/api/devices/v1/devices", "/devices/v1/devices"},
		{"/api/device-groups/v1/device-groups", "/device-groups/v1/device-groups"},
		{"/api/device-actions/v1/devices/x/restart", "/device-actions/v1/devices/x/restart"},
		{"/api/compliance-benchmarks/v1/benchmarks", "/compliance-benchmarks/v1/benchmarks"},
		{"/api/ddm/report/v1/devices/x/declarations", "/ddm/report/v1/devices/x/declarations"},
		{"/api/licensing/v1/licenses", "/licensing/v1/licenses"},
		{"/api/sso/v1/domains", "/sso/v1/domains"},
		{"/api/devices/v1/devices?page=0", "/devices/v1/devices?page=0"},
	}
	for _, tc := range cases {
		got, err := RewriteJamfPath(true, tc.in)
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
	if got := opClassForPath("GET", "/devices/v1/devices"); got != OpPlatform {
		t.Fatalf("GET rewritten devices → %q", got)
	}
	if got := opClassForPath("POST", "/api/device-actions/v1/devices/x/restart"); got != OpPlatformWrite {
		t.Fatalf("POST device-actions → %q", got)
	}
	if got := opClassForPath("GET", "/api/ddm/report/v1/devices/x/declarations"); got != OpPlatform {
		t.Fatalf("GET ddm/report → %q", got)
	}
	if got := opClassForPath("GET", "/pro/v1/buildings"); got != OpPlatform {
		t.Fatalf("GET rewritten pro → %q", got)
	}
	if got := opClassForPath("GET", "/api/licensing/v1/licenses"); got != OpPlatform {
		t.Fatalf("GET licensing → %q", got)
	}
	if got := opClassForPath("GET", "/licensing/v1/licenses"); got != OpPlatform {
		t.Fatalf("GET rewritten licensing → %q", got)
	}
	if got := opClassForPath("GET", "/api/sso/v1/domains"); got != OpPlatform {
		t.Fatalf("GET sso → %q", got)
	}
}

func TestOpClassForPath_Classic(t *testing.T) {
	if got := opClassForPath("GET", "/JSSResource/policies"); got != OpClassic {
		t.Fatalf("GET Classic → %q", got)
	}
	if got := opClassForPath("POST", "/JSSResource/policies/id/0"); got != OpClassicWrite {
		t.Fatalf("POST Classic → %q want %q", got, OpClassicWrite)
	}
	if got := opClassForPath("PUT", "/proclassic/policies/id/1"); got != OpClassicWrite {
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
	got, err := RewriteJamfPath(true, "/api/pro/v1/tenant/tid-1/buildings")
	if err != nil {
		t.Fatal(err)
	}
	want := "/pro/v1/buildings"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_QueryPreserved(t *testing.T) {
	got, err := RewriteJamfPath(true, "/api/v1/packages/1/history?sort=date:desc&page-size=1")
	if err != nil {
		t.Fatal(err)
	}
	want := "/pro/v1/packages/1/history?sort=date:desc&page-size=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteJamfPath_SchemaRejected(t *testing.T) {
	_, err := RewriteJamfPath(true, "/api/schema")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected not supported error, got %v", err)
	}
}

func TestRewriteJamfPath_ACSRejected(t *testing.T) {
	_, err := RewriteJamfPath(true, "/api/config-profiles/plist/can-convert-to-plist")
	if err == nil || !strings.Contains(err.Error(), "ACS is instance-only") {
		t.Fatalf("expected ACS instance-only error, got %v", err)
	}
	got, err := RewriteJamfPath(false, "/api/config-profiles/macos/x")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/config-profiles/macos/x" {
		t.Fatalf("instance rewrite should leave ACS path, got %q", got)
	}
}

func TestRewriteJamfPath_ProPoliciesListRejected(t *testing.T) {
	_, err := RewriteJamfPath(true, "/api/v1/policies")
	if err == nil || !strings.Contains(err.Error(), "undocumented Pro policy list is instance-only") {
		t.Fatalf("expected policy-list instance-only error, got %v", err)
	}
	got, err := RewriteJamfPath(false, "/api/v1/policies")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/v1/policies" {
		t.Fatalf("instance rewrite should leave policy list path, got %q", got)
	}
}

func TestDefaultJamfMedia_Proclassic(t *testing.T) {
	ct, accept := DefaultJamfMedia("/proclassic/policies")
	if ct != "application/xml" || accept != "application/xml" {
		t.Fatalf("got ct=%q accept=%q", ct, accept)
	}
}

func TestCredentialValidate_PlatformRequiresOAuth(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.api.jamfcloud.com",
		TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
		Auth:     AuthConfig{Type: "basic", Username: "u"},
	}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "oauth") {
		t.Fatalf("expected oauth error, got %v", err)
	}
}

func TestCredentialValidate_GatewayURLOrgOK(t *testing.T) {
	cred := CredentialConfig{
		URL:  "https://us.api.jamfcloud.com",
		Auth: AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatalf("org-scope platform cred should be valid: %v", err)
	}
}

func TestCredentialValidate_RetiredGatewayRefused(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.apigw.jamf.com",
		TenantID: "tid",
		Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
	}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("expected retired gateway error, got %v", err)
	}
}

func TestCredentialValidate_BothScopeIDsRefused(t *testing.T) {
	cred := CredentialConfig{
		Region:        "us",
		EnvironmentID: "env-1",
		TenantID:      "tid-1",
		Auth:          AuthConfig{Type: "oauth", ClientID: "c"},
	}
	err := cred.Validate()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestTenantValidate_OrganizationWorkplace(t *testing.T) {
	tcfg := TenantConfig{}
	if err := tcfg.Validate(); err != nil {
		t.Fatalf("empty context is an org workplace: %v", err)
	}
	if !tcfg.IsOrganizationWorkplace() || !tcfg.HasPlatform() {
		t.Fatal("org workplace should HasPlatform with no region")
	}
	if tcfg.GatewayBaseURL() != OrgPlatformGatewayURL {
		t.Fatalf("GatewayBaseURL = %q", tcfg.GatewayBaseURL())
	}
}

func TestTenantValidate_OrgOK(t *testing.T) {
	tcfg := TenantConfig{Region: "us"}
	if err := tcfg.Validate(); err != nil {
		t.Fatalf("org-scope tenant should be valid: %v", err)
	}
	if !tcfg.HasPlatform() {
		t.Fatal("region-only tenant should HasPlatform")
	}
	if tcfg.HasScopeID() {
		t.Fatal("org tenant should have no scope id")
	}
}

func TestTenantValidate_HybridNeedsScopeID(t *testing.T) {
	tcfg := TenantConfig{URL: "https://example.jamfcloud.com", Region: "us"}
	err := tcfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "hybrid") {
		t.Fatalf("expected hybrid scope error, got %v", err)
	}
}

func TestTenantValidate_RetiredURLRefused(t *testing.T) {
	tcfg := TenantConfig{URL: "https://eu.apigw.jamf.com", TenantID: "tid"}
	err := tcfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("expected retired gateway error, got %v", err)
	}
}

func TestCredentialValidate_PlatformOAuthOK(t *testing.T) {
	cred := CredentialConfig{
		URL:      "https://us.api.jamfcloud.com",
		TenantID: "example-opaque-tenant",
		Auth:     AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialValidate_RegionWithoutURL(t *testing.T) {
	cred := CredentialConfig{
		Region:        "us",
		EnvironmentID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
		Auth:          AuthConfig{Type: "oauth", ClientID: "c"},
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := cred.EffectiveBaseURL(); got != "https://us.api.jamfcloud.com" {
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
				URL:           "https://us.api.jamfcloud.com",
				EnvironmentID: "tid",
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
		URL:      "https://us.api.jamfcloud.com",
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
	if !(CredentialConfig{EnvironmentID: "e"}).IsPlatform() {
		t.Fatal("environmentId should enable platform")
	}
	if !(CredentialConfig{Region: "us"}).IsPlatform() {
		t.Fatal("region should enable platform")
	}
}

func TestLooksLikePlatformGateway(t *testing.T) {
	if !LooksLikePlatformGateway("https://us.api.jamfcloud.com") {
		t.Fatal("expected true for GA regional host")
	}
	if !LooksLikePlatformGateway("https://api.jamfcloud.com") {
		t.Fatal("expected true for org host")
	}
	if LooksLikePlatformGateway("https://us.apigw.jamf.com") {
		t.Fatal("retired host is not the GA gateway")
	}
	if !LooksLikeRetiredPlatformGateway("https://us.apigw.jamf.com") {
		t.Fatal("expected retired")
	}
	if LooksLikePlatformGateway("https://example.jamfcloud.com") {
		t.Fatal("expected false for instance")
	}
}

func TestComposeCredential_OrganizationUsesUSGateway(t *testing.T) {
	tenant := TenantConfig{
		URL:           "https://example.jamfcloud.com",
		Region:        "apac",
		EnvironmentID: "11111111-1111-1111-1111-111111111111",
	}
	auth := AuthEntry{Type: AuthTypePlatform, Scope: PlatformScopeOrganization, ClientID: "c", Tenants: []string{"sandbox"}}
	cred := ComposeCredential(tenant, auth, SurfacePlatform)
	if cred.URL != OrgPlatformGatewayURL || cred.Region != OrgPlatformRegion || cred.EnvironmentID != "" || cred.TenantID != "" {
		t.Fatalf("org compose should use us.api.jamfcloud.com with no env/tenant headers: %+v", cred)
	}
	if err := cred.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestUsesPlatformAuth(t *testing.T) {
	gwOnly := CredentialConfig{URL: "https://apac.api.jamfcloud.com", Auth: AuthConfig{Type: "oauth"}}
	if !gwOnly.UsesPlatformAuth() {
		t.Fatal("GA gateway URL should use platform auth")
	}
	inst := CredentialConfig{URL: "https://x.jamfcloud.com", Auth: AuthConfig{Type: "oauth"}}
	if inst.UsesPlatformAuth() {
		t.Fatal("instance URL should not use platform auth")
	}
}

func TestEffectiveTenantID_FromAuthMisplaced(t *testing.T) {
	cred := CredentialConfig{
		URL: "https://us.api.jamfcloud.com",
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

func TestPlatformAuthTestHint(t *testing.T) {
	hint := platformAuthTestHint(`{"errors":[{"code":"REQUEST_CONTEXT_NOT_PROVIDED"}]}`, CredentialConfig{})
	if hint == "" || !strings.Contains(hint, "organization") {
		t.Fatalf("got %q", hint)
	}
	hint = platformAuthTestHint(`OWNERSHIP_FORBIDDEN`, CredentialConfig{EnvironmentID: "env-1"})
	if !strings.Contains(hint, "X-Environment-Id") {
		t.Fatalf("got %q", hint)
	}
}
