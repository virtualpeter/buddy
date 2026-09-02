package buddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func setupPolicyCacheCred(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const cred = "cache-test"
	SetActiveConfig(ConfigFromCredentialMap(cred, map[string]CredentialConfig{
		cred: {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "command", RunCommand: "printf Bearer-test"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })
	return cred
}

func writePolicyCache(t *testing.T, cred string, m PolicyCacheManifest, sums []PolicyCacheSummary) string {
	t.Helper()
	dir, err := PolicyCacheTenantDir(cred)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, policyCacheManifestName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	sraw, err := json.Marshal(sums)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, policyCacheSummariesName), sraw, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPolicyCacheMissingStatus(t *testing.T) {
	cred := setupPolicyCacheCred(t)
	m, err := LoadPolicyCacheManifest(cred)
	if err != nil || m != nil {
		t.Fatalf("missing manifest: m=%v err=%v", m, err)
	}
	sums, err := LoadPolicyCacheSummaries(cred)
	if err != nil || len(sums) != 0 {
		t.Fatalf("missing summaries: %v %v", sums, err)
	}
	st, err := GetPolicyCacheStatus(cred)
	if err != nil {
		t.Fatal(err)
	}
	if st.Exists {
		t.Fatal("expected missing cache")
	}
	if got := FormatPolicyCacheLine(st); got != "Policy cache: missing" {
		t.Fatalf("line=%q", got)
	}
}

func TestPolicyCacheRoundTripAndAppScan(t *testing.T) {
	cred := setupPolicyCacheCred(t)
	fetched := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	sums := []PolicyCacheSummary{
		{ID: 1, Name: "Install Santa", UseForSelfService: true, ScriptParams: []string{"install-santa-next"}},
		{ID: 2, Name: "Install Santa Next", Trigger: "install-santa-next", Enabled: true, PackageNames: []string{"Santa-2026.2.pkg"}},
		{ID: 3, Name: "Cache Santa 2026.2", Trigger: "cache-santa-2026.2", Enabled: true, PackageNames: []string{"Santa-2026.2.pkg"}},
	}
	dir := writePolicyCache(t, cred, PolicyCacheManifest{
		FetchedAt: fetched, Tenant: "sandbox", Count: len(sums),
	}, sums)

	got, err := LoadPolicyCacheManifest(cred)
	if err != nil || got == nil || got.Count != 3 || !got.FetchedAt.Equal(fetched) {
		t.Fatalf("manifest=%+v err=%v", got, err)
	}
	loaded, err := LoadPolicyCacheSummaries(cred)
	if err != nil || len(loaded) != 3 {
		t.Fatalf("summaries=%v err=%v", loaded, err)
	}

	st, err := GetPolicyCacheStatus(cred)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Exists || st.Dir != dir || st.Manifest.Count != 3 {
		t.Fatalf("status=%+v dir=%s", st, dir)
	}
	line := FormatPolicyCacheLine(st)
	if !strings.Contains(line, "fetched 2026-08-10T12:00:00Z") || !strings.Contains(line, "3 policies") {
		t.Fatalf("line=%q", line)
	}
	info := st.Info()
	if info.FetchedAt != "2026-08-10T12:00:00Z" || info.Count != 3 || !info.Exists {
		t.Fatalf("info=%+v", info)
	}

	res, err := AppScan(cred, AppScanOpts{Filter: "santa"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "cache" {
		t.Fatalf("source=%q (must not hit live Jamf)", res.Source)
	}
	if len(res.Apps) != 1 || res.Apps[0].Name != "santa" || res.Apps[0].Kind != PolicyAppCache {
		t.Fatalf("apps=%+v", res.Apps)
	}
	if res.Apps[0].PackageName != "Santa-2026.2.pkg" {
		t.Fatalf("package=%q", res.Apps[0].PackageName)
	}
}

func TestPolicyCacheGetFromDisk(t *testing.T) {
	cred := setupPolicyCacheCred(t)
	dir, err := PolicyCacheTenantDir(cred)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const id = 10
	xml := `<policy><general><id>10</id><name>Cache Santa 2026.2</name><enabled>true</enabled><trigger_other>cache-santa-2026.2</trigger_other></general>` +
		`<self_service><use_for_self_service>false</use_for_self_service></self_service>` +
		`<package_configuration><packages><package><id>1</id><name>Santa-2026.2.pkg</name><action>Cache</action></package></packages></package_configuration>` +
		`</policy>`
	if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(id)+".xml"), []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := PolicyCacheGet(cred, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.General.ID != 10 || p.General.Name != "Cache Santa 2026.2" || p.General.TriggerOther != "cache-santa-2026.2" {
		t.Fatalf("policy=%+v", p.General)
	}
	if len(p.PackageConfiguration.Packages) != 1 || p.PackageConfiguration.Packages[0].Name != "Santa-2026.2.pkg" {
		t.Fatalf("packages=%+v", p.PackageConfiguration.Packages)
	}
	if _, err := PolicyCacheGet(cred, 0, nil); err == nil {
		t.Fatal("expected error for id 0")
	}
}

func TestSummaryFromResourcePolicy(t *testing.T) {
	p := &resourcePolicy{}
	p.General.ID = 21
	p.General.Name = "Install Santa [SS]"
	p.General.Enabled = true
	p.General.TriggerOther = ""
	p.SelfService.UseForSelfService = true
	p.PackageConfiguration.Packages = []policySubsetPackageConfigurationPackage{{Name: "Santa-2026.2.pkg"}}
	p.Scripts = []policySubsetScript{{Parameter4: "install-santa-next", Parameter5: "  "}}
	sum := summaryFromResourcePolicy(p)
	if sum.ID != 21 || !sum.UseForSelfService || sum.Trigger != "" {
		t.Fatalf("%+v", sum)
	}
	if len(sum.PackageNames) != 1 || sum.PackageNames[0] != "Santa-2026.2.pkg" {
		t.Fatalf("packages=%v", sum.PackageNames)
	}
	if len(sum.ScriptParams) != 1 || sum.ScriptParams[0] != "install-santa-next" {
		t.Fatalf("params=%v", sum.ScriptParams)
	}
	if summaryFromResourcePolicy(nil).ID != 0 {
		t.Fatal("nil policy")
	}
}

func TestLoadPolicyCacheManifestCorrupt(t *testing.T) {
	cred := setupPolicyCacheCred(t)
	dir, err := PolicyCacheTenantDir(cred)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, policyCacheManifestName), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicyCacheManifest(cred); err == nil {
		t.Fatal("expected parse error")
	}
}
