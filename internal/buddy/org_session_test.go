package buddy

import (
	"strings"
	"testing"
)

func TestOrganizationWorkplaceFor(t *testing.T) {
	cfg := orgChildLicensesTestConfig()
	if got := organizationWorkplaceFor(cfg, "sandbox", cfg.Tenants["sandbox"]); got != "sandbox" {
		t.Fatalf("org self = %q", got)
	}
	if got := organizationWorkplaceFor(cfg, "prod", cfg.Tenants["prod"]); got != "sandbox" {
		t.Fatalf("env parent = %q", got)
	}
	if got := organizationWorkplaceFor(cfg, "prod-pro", cfg.Tenants["prod-pro"]); got != "sandbox" {
		t.Fatalf("product-tenant parent = %q", got)
	}
	orphan := TenantConfig{Region: "us", EnvironmentID: "11111111-1111-1111-1111-111111111111"}
	if got := organizationWorkplaceFor(cfg, "orphan", orphan); got != "" {
		t.Fatalf("orphan = %q", got)
	}
}

func TestPickOrgAccountSession_FromEnvironment(t *testing.T) {
	SetActiveConfig(orgChildLicensesTestConfig())
	SetActiveSelection("prod", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, err := pickOrgAccountSession("")
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "sandbox" || auth != "sandbox-org" {
		t.Fatalf("got tenant=%q auth=%q", tenant, auth)
	}
}

func TestPickOrgAccountSession_FromProductTenant(t *testing.T) {
	SetActiveConfig(orgChildLicensesTestConfig())
	SetActiveSelection("prod-pro", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, err := pickOrgAccountSession("")
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "sandbox" || auth != "sandbox-org" {
		t.Fatalf("got tenant=%q auth=%q", tenant, auth)
	}
}

func TestPickOrgAccountSession_AlreadyOrg(t *testing.T) {
	SetActiveConfig(orgLicensesTestConfig())
	SetActiveSelection("sandbox", "sandbox")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, err := pickOrgAccountSession("sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "sandbox" || auth != "sandbox" {
		t.Fatalf("got tenant=%q auth=%q", tenant, auth)
	}
}

func TestPickOrgAccountSession_IgnoresEnvironmentPin(t *testing.T) {
	SetActiveConfig(orgChildLicensesTestConfig())
	SetActiveSelection("prod", "prod-plat")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, err := pickOrgAccountSession("prod-plat")
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "sandbox" || auth != "sandbox-org" {
		t.Fatalf("env pin should not be reused: tenant=%q auth=%q", tenant, auth)
	}
}

func TestPickOrgAccountSession_ChildWithoutAuths(t *testing.T) {
	cfg := orgChildLicensesTestConfig()
	delete(cfg.Auths, "prod-plat")
	delete(cfg.Auths, "prod-admin")
	SetActiveConfig(cfg)
	SetActiveSelection("prod", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, err := pickOrgAccountSession("")
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "sandbox" || auth != "sandbox-org" {
		t.Fatalf("got tenant=%q auth=%q", tenant, auth)
	}
}

func TestPickOrgAccountSession_NoParent(t *testing.T) {
	SetActiveConfig(ConfigFromCredentialMap("gw", map[string]CredentialConfig{
		"gw": {
			Region:        "us",
			EnvironmentID: "11111111-1111-1111-1111-111111111111",
			Auth: AuthConfig{
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "sec",
			},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })

	_, _, err := pickOrgAccountSession("gw")
	if err == nil || !strings.Contains(err.Error(), "organization parent") {
		t.Fatalf("expected no organization parent, got %v", err)
	}
}
