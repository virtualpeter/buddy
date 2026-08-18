package buddy

import (
	"strings"
	"testing"
	"time"

	"buddy/internal/buddy/secrets"
)

func TestCapsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	restore := SetCapsCacheDirForTest(dir)
	t.Cleanup(restore)

	rec := CapsRecord{
		Auth:        "sandbox-oauth",
		Tenant:      "sandbox",
		Surface:     SurfaceInstance,
		Live:        LiveTokenOK,
		TokenOk:     true,
		ProbeOk:     false,
		Probe:       "auth",
		ProbeStatus: 403,
		Privileges:  []string{"Read Computers"},
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := SaveCaps(rec); err != nil {
		t.Fatal(err)
	}
	got := LoadCaps("sandbox-oauth", "sandbox", SurfaceInstance, "")
	if got == nil {
		t.Fatal("expected record")
	}
	if got.Live != LiveTokenOK || !got.TokenOk || got.ProbeOk || len(got.Privileges) != 1 {
		t.Fatalf("%+v", got)
	}
	InvalidateCaps("sandbox-oauth", "sandbox", SurfaceInstance, "")
	if LoadCaps("sandbox-oauth", "sandbox", SurfaceInstance, "") != nil {
		t.Fatal("expected deleted")
	}
}

func TestCapsExpiredNotLoaded(t *testing.T) {
	dir := t.TempDir()
	restore := SetCapsCacheDirForTest(dir)
	t.Cleanup(restore)

	rec := CapsRecord{
		Auth:      "sandbox-oauth",
		Tenant:    "sandbox",
		Surface:   SurfaceInstance,
		Live:      LiveOK,
		TokenOk:   true,
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	if err := SaveCaps(rec); err != nil {
		t.Fatal(err)
	}
	// SaveCaps overwrites zero/past expiry? It keeps ExpiresAt if set.
	if LoadCaps("sandbox-oauth", "sandbox", SurfaceInstance, "") != nil {
		t.Fatal("expired record should not load")
	}
}

func TestCapsCacheKeyPlatform(t *testing.T) {
	k := CapsCacheKey("plat", "sandbox", SurfacePlatform, "tid-1")
	if k != "plat@sandbox@platform:tid-1" {
		t.Fatalf("key=%q", k)
	}
	k2 := CapsCacheKey("plat", "sandbox", SurfaceInstance, "tid-1")
	if k2 != "plat@sandbox" {
		t.Fatalf("instance key=%q", k2)
	}
}

func TestResourceClassForPath(t *testing.T) {
	cases := map[string]string{
		"/api/v1/packages":                     "packages",
		"/api/v1/scripts":                      "scripts",
		"/api/v3/computers-inventory":          "computers",
		"/api/v3/computer-groups/smart-groups": "groups",
		"/JSSResource/policies/id/1":           "policies",
		"/api/v2/categories":                   "categories",
		"/api/v2/os-x-configuration-profiles":  "profiles",
		"/api/v1/auth":                         "",
	}
	for path, want := range cases {
		if got := resourceClassForPath(path); got != want {
			t.Errorf("%s: got %q want %q", path, got, want)
		}
	}
}

func TestCapsHasResourcePrivilege(t *testing.T) {
	rec := &CapsRecord{Privileges: []string{"Read policies in Jamf Pro API"}, PrivilegeSet: "CUSTOM"}
	known, has := capsHasResourcePrivilege(rec, "policies")
	if !known || !has {
		t.Fatalf("policies known=%v has=%v", known, has)
	}
	known, has = capsHasResourcePrivilege(rec, "computers")
	if !known || has {
		t.Fatalf("computers known=%v has=%v", known, has)
	}
	admin := &CapsRecord{PrivilegeSet: "ADMINISTRATOR"}
	known, has = capsHasResourcePrivilege(admin, "computers")
	if !known || !has {
		t.Fatalf("admin known=%v has=%v", known, has)
	}
	empty := &CapsRecord{Live: LiveOK, TokenOk: true}
	known, has = capsHasResourcePrivilege(empty, "policies")
	if known {
		t.Fatal("empty privilege list should fail open")
	}
}

func TestBuildCoverageReportGaps(t *testing.T) {
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-oauth/client-secret": "s",
		"cli-only/client-secret":      "s",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://example.jamfcloud.com",
				Region:   "us",
				TenantID: "11111111-1111-1111-1111-111111111111",
			},
			"prod": {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-oauth": {
				Type: "oauth", ClientID: "cid",
				Tenants: []string{"sandbox"}, Surface: SurfaceInstance,
			},
			"orphan": {
				Type: "oauth", ClientID: "c2", Tenants: []string{},
			},
			"cli-only": {
				Type: "oauth", ClientID: "c3",
				Tenants: []string{"prod"}, Channels: []string{ChannelCLI},
			},
		},
	}
	rep := BuildCoverageReport(cfg)
	var sawPlatGap, sawInstCovered, sawProd bool
	for _, c := range rep.Cells {
		if c.Tenant == "sandbox" && c.Surface == SurfacePlatform && c.Gap && c.GapReason == "no auth" {
			sawPlatGap = true
		}
		if c.Tenant == "sandbox" && c.Surface == SurfaceInstance && !c.Gap && len(c.Auths) == 1 && c.Auths[0].Secret == SecretPresenceStored {
			sawInstCovered = true
		}
		if c.Tenant == "prod" && c.Surface == SurfaceInstance && !c.Gap {
			sawProd = true
		}
	}
	if !sawPlatGap || !sawInstCovered || !sawProd {
		t.Fatalf("cells=%+v gaps plat=%v inst=%v prod=%v", rep.Cells, sawPlatGap, sawInstCovered, sawProd)
	}
	if !rep.HasGaps {
		t.Fatal("expected HasGaps")
	}
	if len(rep.UnlinkedAuths) != 1 || rep.UnlinkedAuths[0] != "orphan" {
		t.Fatalf("unlinked=%v", rep.UnlinkedAuths)
	}
	if len(rep.ChannelHidden) == 0 || !strings.Contains(strings.Join(rep.ChannelHidden, " "), "cli-only") {
		t.Fatalf("channel hidden=%v", rep.ChannelHidden)
	}

	var b strings.Builder
	FormatCoverageReport(rep, &b)
	out := b.String()
	if !strings.Contains(out, "GAP") || !strings.Contains(out, "sandbox") {
		t.Fatalf("format:\n%s", out)
	}
}

func TestAuthCoversSurface(t *testing.T) {
	inst := AuthEntry{Type: "oauth", Surface: SurfaceInstance}
	if !AuthCoversSurface(inst, SurfaceInstance) || AuthCoversSurface(inst, SurfacePlatform) {
		t.Fatal("instance auth")
	}
	anyA := AuthEntry{Type: "oauth", Surface: SurfaceAny}
	if !AuthCoversSurface(anyA, SurfaceInstance) || !AuthCoversSurface(anyA, SurfacePlatform) {
		t.Fatal("any oauth should cover both")
	}
	basic := AuthEntry{Type: "basic", Surface: SurfaceAny}
	if AuthCoversSurface(basic, SurfacePlatform) {
		t.Fatal("basic cannot cover platform")
	}
}
