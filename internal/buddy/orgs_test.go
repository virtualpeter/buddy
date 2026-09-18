package buddy

import (
	"strings"
	"testing"

	"buddy/internal/buddy/secrets"

	"gopkg.in/yaml.v3"
)

const nestedOrgsYAML = `
context:
  default: sandbox
orgs:
  acme:
    id: 00000000-0000-0000-0000-000000000000
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
            url: https://example.jamfcloud.com
auths:
  acme-org:
    type: platform
    scope: organization
    contexts: [acme]
    clientId: oid
  env-plat:
    type: platform
    scope: environment
    contexts: [commercial]
    clientId: eid
  sandgod:
    type: oauth
    jamfpro: sandbox
    clientId: cid
`

func TestUnmarshalOrgs_InstanceOnlyUnchanged(t *testing.T) {
	data := []byte(`
context:
  default: sandbox
contexts:
  sandbox:
    jamfpro:
      url: https://example.jamfcloud.com
auths:
  sandgod:
    type: oauth
    jamfpro: sandbox
    clientId: cid
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Orgs) != 0 {
		t.Fatalf("orgs = %+v", cfg.Orgs)
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("sandbox = %+v", cfg.Tenants["sandbox"])
	}
	if cfg.Tenants["sandbox"].Region != "" || cfg.Tenants["sandbox"].EnvironmentID != "" {
		t.Fatalf("instance-only should not grow platform fields: %+v", cfg.Tenants["sandbox"])
	}
	if got := cfg.Tenants["sandbox"].WorkplaceKind(); got != "instance" {
		t.Fatalf("sandbox kind = %q", got)
	}
}

func TestUnmarshalOrgs_ExpandsWorkplaces(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(nestedOrgsYAML), &cfg); err != nil {
		t.Fatal(err)
	}
	org := cfg.Tenants["acme"]
	if !org.IsOrganizationWorkplace() || org.Region != "" || org.URL != "" || org.EnvironmentID != "" {
		t.Fatalf("org workplace=%+v", org)
	}
	if cfg.Orgs["acme"].ID != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("org id stored on orgs tree, not headers: %+v", cfg.Orgs["acme"])
	}
	env := cfg.Tenants["commercial"]
	if env.Org != "acme" || env.Region != "us" || env.EnvironmentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("env workplace=%+v", env)
	}
	if env.URL != "" || env.TenantID != "" {
		t.Fatalf("env should not inherit product tenant: %+v", env)
	}
	ten := cfg.Tenants["sandbox"]
	if ten.Org != "acme" || ten.Region != "us" || ten.EnvironmentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("nested tenant inherit=%+v", ten)
	}
	if ten.TenantID != "22222222-2222-2222-2222-222222222222" || ten.URL != "https://example.jamfcloud.com" {
		t.Fatalf("nested tenant ids=%+v", ten)
	}
	if org.WorkplaceKind() != "organization" || env.WorkplaceKind() != "environment" || ten.WorkplaceKind() != "tenant" {
		t.Fatalf("kinds org=%q env=%q tenant=%q", org.WorkplaceKind(), env.WorkplaceKind(), ten.WorkplaceKind())
	}
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		t.Fatal(err)
	}
	cred := ComposeCredential(org, cfg.Auths["acme-org"], SurfacePlatform)
	if cred.URL != OrgPlatformGatewayURL || cred.Region != OrgPlatformRegion {
		t.Fatalf("org compose=%+v", cred)
	}
	if cred.EnvironmentID != "" || cred.TenantID != "" {
		t.Fatalf("org compose must not send env/tenant headers: %+v", cred)
	}
}

func TestUnmarshalOrgs_OverlaySameName(t *testing.T) {
	data := []byte(`
context:
  default: sandbox
contexts:
  sandbox:
    jamfpro:
      url: https://example.jamfcloud.com
orgs:
  acme:
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	ten := cfg.Tenants["sandbox"]
	if ten.URL != "https://example.jamfcloud.com" {
		t.Fatalf("keep instance url: %+v", ten)
	}
	if ten.TenantID != "22222222-2222-2222-2222-222222222222" || ten.EnvironmentID == "" || ten.Org != "acme" {
		t.Fatalf("overlay platform onto instance: %+v", ten)
	}
}

func TestUnmarshalOrgs_NameCollision(t *testing.T) {
	data := []byte(`
orgs:
  acme:
    environments:
      acme:
        id: 11111111-1111-1111-1111-111111111111
        region: us
`)
	var cfg Config
	err := yaml.Unmarshal(data, &cfg)
	if err == nil || !strings.Contains(err.Error(), "used twice") {
		t.Fatalf("expected name collision, got %v", err)
	}
}

func TestUnmarshalOrgs_OverlayURLConflict(t *testing.T) {
	data := []byte(`
contexts:
  sandbox:
    jamfpro:
      url: https://a.example.jamfcloud.com
orgs:
  acme:
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
            url: https://b.example.jamfcloud.com
`)
	var cfg Config
	err := yaml.Unmarshal(data, &cfg)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected url conflict, got %v", err)
	}
}

func TestUnmarshalOrgs_EnvIDConflict(t *testing.T) {
	data := []byte(`
contexts:
  sandbox:
    id: 11111111-1111-1111-1111-111111111111
    region: us
    jamfpro:
      url: https://example.jamfcloud.com
orgs:
  acme:
    environments:
      commercial:
        id: aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
`)
	var cfg Config
	err := yaml.Unmarshal(data, &cfg)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected environment id conflict, got %v", err)
	}
}

func TestMarshalOrgs_OmitsImpliedContexts(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(nestedOrgsYAML), &cfg); err != nil {
		t.Fatal(err)
	}
	out, err := marshalConfig(&cfg, []byte(nestedOrgsYAML))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "orgs:") || !strings.Contains(s, "acme:") {
		t.Fatalf("expected orgs tree:\n%s", s)
	}
	if yamlHasTopLevelKey(s, "contexts") {
		t.Fatalf("implied workplaces should be omitted from contexts:\n%s", s)
	}
	var again Config
	if err := yaml.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if !workplaceIdentityEqual(cfg.Tenants["acme"], again.Tenants["acme"]) ||
		!workplaceIdentityEqual(cfg.Tenants["commercial"], again.Tenants["commercial"]) ||
		!workplaceIdentityEqual(cfg.Tenants["sandbox"], again.Tenants["sandbox"]) {
		t.Fatalf("reload not idempotent\nfirst=%+v\nsecond=%+v", cfg.Tenants, again.Tenants)
	}
	out2, err := marshalConfig(&again, out)
	if err != nil {
		t.Fatal(err)
	}
	var third Config
	if err := yaml.Unmarshal(out2, &third); err != nil {
		t.Fatal(err)
	}
	if !workplaceIdentityEqual(again.Tenants["sandbox"], third.Tenants["sandbox"]) {
		t.Fatalf("second reload changed sandbox: %+v vs %+v", again.Tenants["sandbox"], third.Tenants["sandbox"])
	}
}

func TestMarshalOrgs_KeepsLeftoverInstanceContext(t *testing.T) {
	data := []byte(`
context:
  default: sandbox
orgs:
  acme:
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
            url: https://example.jamfcloud.com
contexts:
  other:
    jamfpro:
      url: https://other.example.jamfcloud.com
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	out, err := marshalConfig(&cfg, data)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "orgs:") || !strings.Contains(s, "other:") {
		t.Fatalf("expected orgs + leftover other:\n%s", s)
	}
	if !yamlHasTopLevelKey(s, "contexts") {
		t.Fatalf("expected leftover contexts:\n%s", s)
	}
	var dumped struct {
		Contexts map[string]TenantConfig `yaml:"contexts"`
	}
	if err := yaml.Unmarshal(out, &dumped); err != nil {
		t.Fatal(err)
	}
	if _, ok := dumped.Contexts["sandbox"]; ok {
		t.Fatalf("sandbox should not appear under contexts: %+v", dumped.Contexts)
	}
	if dumped.Contexts["other"].URL != "https://other.example.jamfcloud.com" {
		t.Fatalf("other leftover=%+v", dumped.Contexts["other"])
	}
}

func TestUnmarshalOrgs_InstanceEnvURL(t *testing.T) {
	data := []byte(`
context:
  default: sandbox
orgs:
  default:
    environments:
      sandbox:
        url: https://example.jamfcloud.com
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.Tenants["default"].IsOrganizationWorkplace() {
		t.Fatalf("org=%+v", cfg.Tenants["default"])
	}
	got := cfg.Tenants["sandbox"]
	if got.URL != "https://example.jamfcloud.com" || got.Org != "default" {
		t.Fatalf("sandbox=%+v", got)
	}
	if got.WorkplaceKind() != "instance" {
		t.Fatalf("kind=%q", got.WorkplaceKind())
	}
}

func TestFoldFlatIntoOrgs_InstanceOnly(t *testing.T) {
	cfg := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
	}
	folded, err := foldFlatIntoOrgs(cfg)
	if err != nil || !folded {
		t.Fatalf("folded=%v err=%v", folded, err)
	}
	if cfg.Orgs["default"].Environments["sandbox"].EffectiveURL() != "https://example.jamfcloud.com" {
		t.Fatalf("orgs=%+v", cfg.Orgs)
	}
	if cfg.Tenants["sandbox"].Org != "default" {
		t.Fatalf("sandbox=%+v", cfg.Tenants["sandbox"])
	}
	out, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if yamlHasTopLevelKey(s, "contexts") {
		t.Fatalf("expected orgs-only:\n%s", s)
	}
	folded2, err := foldFlatIntoOrgs(cfg)
	if err != nil || folded2 {
		t.Fatalf("second fold should be no-op: folded=%v err=%v", folded2, err)
	}
}

func TestFoldFlatIntoOrgs_SeveralInstances(t *testing.T) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://sandbox.example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
	}
	if _, err := foldFlatIntoOrgs(cfg); err != nil {
		t.Fatal(err)
	}
	org := cfg.Orgs["default"]
	if org.Environments["sandbox"].EffectiveURL() == "" || org.Environments["prod"].EffectiveURL() == "" {
		t.Fatalf("expected one env per instance: %+v", org)
	}
}

func TestFoldFlatIntoOrgs_SharedEnvironmentID(t *testing.T) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://sandbox.example.jamfcloud.com", Region: "us", EnvironmentID: "e1", TenantID: "t1"},
			"prod":    {URL: "https://prod.example.jamfcloud.com", Region: "us", EnvironmentID: "e1", TenantID: "t2"},
		},
	}
	if _, err := foldFlatIntoOrgs(cfg); err != nil {
		t.Fatal(err)
	}
	org := cfg.Orgs["default"]
	var envName string
	for n, e := range org.Environments {
		if e.EffectiveID() == "e1" {
			envName = n
			if len(e.Tenants) != 2 {
				t.Fatalf("want 2 tenants under shared env: %+v", e)
			}
		}
	}
	if envName == "" {
		t.Fatalf("missing shared env: %+v", org)
	}
}

func TestDeleteWorkplace_BlocksParentWithChildren(t *testing.T) {
	cfg := &Config{}
	if _, err := SetTenantMetadata(cfg, "sandbox", SetTenantOpts{URL: "https://example.jamfcloud.com", Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	_, err := DeleteTenantMetadata(cfg, "default")
	if err == nil || !strings.Contains(err.Error(), "cannot delete") {
		t.Fatalf("expected parent blocked, got %v", err)
	}
	if _, err := DeleteTenantMetadata(cfg, "sandbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteTenantMetadata(cfg, "default"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Orgs) != 0 {
		t.Fatalf("orgs leftover %+v", cfg.Orgs)
	}
}

func TestRenameWorkplaceAndAuth(t *testing.T) {
	secrets.SetStore(&mockSecretStore{values: map[string]string{"sandgod/client-secret": "s3cret"}})
	t.Cleanup(func() { secrets.SetStore(nil) })
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Auths: map[string]AuthEntry{
			"sandgod": {Type: AuthTypeOAuth, ClientID: "cid", Tenants: []string{"sandbox"}, JamfPro: "sandbox"},
		},
	}
	if _, err := SetTenantMetadata(cfg, "sandbox", SetTenantOpts{URL: "https://example.jamfcloud.com", Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	if err := RenameWorkplace(cfg, "default", "acme"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Orgs["acme"]; !ok {
		t.Fatalf("orgs=%+v", cfg.Orgs)
	}
	if cfg.Tenants["sandbox"].Org != "acme" {
		t.Fatalf("sandbox=%+v", cfg.Tenants["sandbox"])
	}
	if err := RenameWorkplace(cfg, "sandbox", "lab"); err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "lab" {
		t.Fatalf("default=%q", cfg.Tenant.Default)
	}
	if cfg.Auths["sandgod"].JamfPro != "lab" {
		t.Fatalf("auth=%+v", cfg.Auths["sandgod"])
	}
	if err := RenameAuth(cfg, "sandgod", "labgod"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Auths["labgod"]; !ok {
		t.Fatalf("auth missing: %+v", cfg.Auths)
	}
	ok, err := HasCredentialSecret("labgod", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("secret moved? %v %v", ok, err)
	}
	hasOld, _ := HasCredentialSecret("sandgod", secrets.KindClientSecret)
	if hasOld {
		t.Fatal("old secret still present")
	}
}

func yamlHasTopLevelKey(doc, key string) bool {
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, key+":") {
			return true
		}
	}
	return false
}

func TestUnmarshalOrgs_EnvironmentIdAlias(t *testing.T) {
	data := []byte(`
orgs:
  acme:
    environments:
      commercial:
        environmentId: 11111111-1111-1111-1111-111111111111
        region: eu
        tenants:
          sandbox:
            jamfpro:
              tenantId: 22222222-2222-2222-2222-222222222222
              url: https://example.jamfcloud.com
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	env := cfg.Tenants["commercial"]
	if env.EnvironmentID != "11111111-1111-1111-1111-111111111111" || env.Region != "eu" {
		t.Fatalf("env=%+v", env)
	}
	ten := cfg.Tenants["sandbox"]
	if ten.TenantID != "22222222-2222-2222-2222-222222222222" || ten.URL != "https://example.jamfcloud.com" {
		t.Fatalf("tenant=%+v", ten)
	}
}
