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
		"/api/v4/computers-inventory":          "computers",
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
	var sawPlatGap, sawInstLocked, sawProd bool
	for _, c := range rep.Cells {
		if c.Tenant != c.Context {
			t.Fatalf("coverage cell dual-read: tenant=%q context=%q", c.Tenant, c.Context)
		}
		if c.Tenant == "sandbox" && c.Surface == SurfacePlatform && c.Gap && c.GapReason == "no auth" {
			sawPlatGap = true
		}
		if c.Tenant == "sandbox" && c.Surface == SurfaceInstance && !c.Gap && len(c.Auths) == 1 && c.Auths[0].Secret == SecretPresenceLocked {
			sawInstLocked = true
		}
		if c.Tenant == "prod" && c.Surface == SurfaceInstance && !c.Gap {
			sawProd = true
		}
	}
	if !sawPlatGap || !sawInstLocked || !sawProd {
		t.Fatalf("cells=%+v gaps plat=%v inst=%v prod=%v", rep.Cells, sawPlatGap, sawInstLocked, sawProd)
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
	if !strings.Contains(out, "GAP") || !strings.Contains(out, "sandbox") || !strings.Contains(out, "locked") {
		t.Fatalf("format:\n%s", out)
	}

	unlocked := BuildCoverageReportWith(cfg, CoverageOpts{UnlockSecrets: true})
	for _, c := range unlocked.Cells {
		if c.Tenant == "sandbox" && c.Surface == SurfaceInstance {
			if len(c.Auths) != 1 || c.Auths[0].Secret != SecretPresenceStored {
				t.Fatalf("unlocked SECRET want stored, got %+v", c.Auths)
			}
		}
	}
}

func TestAuthSecretPresenceLockedVsUnlock(t *testing.T) {
	hits := 0
	secrets.SetStore(&countingHasStore{
		mockSecretStore: mockSecretStore{values: map[string]string{
			"vault-oauth/client-secret": "s",
		}},
		onHas: func() { hits++ },
	})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"vault-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}},
			"inline-oauth": {
				Type: "oauth", ClientID: "cid", ClientSecret: "inline",
				Tenants: []string{"sandbox"},
			},
			"missing-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}},
		},
	}

	if got := AuthSecretPresence(cfg, "vault-oauth", false); got != SecretPresenceLocked {
		t.Fatalf("locked: got %q", got)
	}
	if hits != 0 {
		t.Fatalf("locked path must not call Has, hits=%d", hits)
	}
	if got := AuthSecretPresence(cfg, "vault-oauth", true); got != SecretPresenceStored {
		t.Fatalf("unlock stored: got %q", got)
	}
	if hits != 1 {
		t.Fatalf("unlock should Has once, hits=%d", hits)
	}
	if got := AuthSecretPresence(cfg, "inline-oauth", false); got != SecretPresenceInline {
		t.Fatalf("inline without unlock: got %q", got)
	}
	if got := AuthSecretPresence(cfg, "missing-oauth", true); got != SecretPresenceMissing {
		t.Fatalf("missing: got %q", got)
	}

	gapCfg := &Config{
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"missing-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}},
		},
	}
	lockedRep := BuildCoverageReport(gapCfg)
	unlockedRep := BuildCoverageReportWith(gapCfg, CoverageOpts{UnlockSecrets: true})
	var lockedGap, unlockedGap bool
	for _, c := range lockedRep.Cells {
		if c.Tenant == "sandbox" && c.Surface == SurfaceInstance {
			lockedGap = c.Gap
			if c.Auths[0].Secret != SecretPresenceLocked {
				t.Fatalf("want locked, got %q", c.Auths[0].Secret)
			}
		}
	}
	for _, c := range unlockedRep.Cells {
		if c.Tenant == "sandbox" && c.Surface == SurfaceInstance {
			unlockedGap = c.Gap && c.GapReason == "no secret"
			if c.Auths[0].Secret != SecretPresenceMissing {
				t.Fatalf("want missing, got %q", c.Auths[0].Secret)
			}
		}
	}
	if lockedGap {
		t.Fatal("locked SECRET must not create a no-secret gap")
	}
	if !unlockedGap {
		t.Fatal("unlocked missing SECRET should gap")
	}
}

// countingHasStore wraps mockSecretStore and counts Has calls.
type countingHasStore struct {
	mockSecretStore
	onHas func()
}

func (s *countingHasStore) Has(credName string, kind secrets.Kind) (bool, error) {
	if s.onHas != nil {
		s.onHas()
	}
	return s.mockSecretStore.Has(credName, kind)
}

func TestAuthCoversSurface(t *testing.T) {
	inst := AuthEntry{Type: "oauth", Surface: SurfaceInstance}
	if !AuthCoversSurface(inst, SurfaceInstance) || AuthCoversSurface(inst, SurfacePlatform) {
		t.Fatal("instance auth")
	}
	omitted := AuthEntry{Type: "oauth"}
	if !AuthCoversSurface(omitted, SurfaceInstance) || AuthCoversSurface(omitted, SurfacePlatform) {
		t.Fatal("omitted surface is instance-only")
	}
	plat := AuthEntry{Type: AuthTypePlatform}
	if AuthCoversSurface(plat, SurfaceInstance) || !AuthCoversSurface(plat, SurfacePlatform) {
		t.Fatal("platform oauth")
	}
	basic := AuthEntry{Type: "basic"}
	if !AuthCoversSurface(basic, SurfaceInstance) || AuthCoversSurface(basic, SurfacePlatform) {
		t.Fatal("basic cannot cover platform")
	}
}
