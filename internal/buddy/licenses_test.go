package buddy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func orgLicensesTestConfig() *Config {
	return &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {},
		},
		Auths: map[string]AuthEntry{
			"sandbox": {
				Type: AuthTypePlatform, Scope: PlatformScopeOrganization,
				ClientID: "cid", ClientSecret: "sec",
				Tenants: []string{"sandbox"}, Access: AccessFull,
			},
		},
	}
}

func TestLicenseList_OrgGatewayNoScopeHeaders(t *testing.T) {
	var sawPath string
	var sawEnv, sawTenant string
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   900,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawEnv = r.Header.Get(HeaderEnvironmentID)
		sawTenant = r.Header.Get(HeaderTenantID)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"totalCount": 1,
			"results": []map[string]interface{}{
				{
					"productName":    "Jamf Pro",
					"sku":            "JPRO",
					"licenseType":    "SUBSCRIPTION",
					"purchasedSeats": 25,
					"startDate":      "2026-01-01",
					"endDate":        "2027-01-01",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Cleanup(SetOrgPlatformGatewayURLForTest(srv.URL))
	SetActiveConfig(orgLicensesTestConfig())
	SetActiveSelection("sandbox", "sandbox")
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	items, err := LicenseListValues("sandbox", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["productName"] != "Jamf Pro" {
		t.Fatalf("items = %+v", items)
	}
	if sawPath != "/licensing/v1/licenses" {
		t.Fatalf("path = %q want /licensing/v1/licenses", sawPath)
	}
	if sawEnv != "" || sawTenant != "" {
		t.Fatalf("scope headers env=%q tenant=%q", sawEnv, sawTenant)
	}
}

func TestLicenseList_EmptyResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   900,
		})
	})
	mux.HandleFunc("/licensing/v1/licenses", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"totalCount":0,"results":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Cleanup(SetOrgPlatformGatewayURLForTest(srv.URL))
	SetActiveConfig(orgLicensesTestConfig())
	SetActiveSelection("sandbox", "sandbox")
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	items, err := LicenseListValues("sandbox", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
}

func TestLicenseList_RejectsEnvironmentWorkplace(t *testing.T) {
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
	_, err := LicenseListValues("gw", slog.Default())
	if err == nil || !strings.Contains(err.Error(), "organization") {
		t.Fatalf("expected organization-scoped error, got %v", err)
	}
}

func TestLicenseList_RejectsInstance(t *testing.T) {
	SetActiveConfig(ConfigFromCredentialMap("inst", map[string]CredentialConfig{
		"inst": {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "oauth", ClientID: "c", ClientSecret: "s"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })
	_, err := LicenseListValues("inst", slog.Default())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLicenseField(t *testing.T) {
	row := map[string]interface{}{"productName": "Jamf Pro", "title": "Other"}
	if got := licenseField(row, "productName", "title"); got != "Jamf Pro" {
		t.Fatalf("got %q", got)
	}
	if got := licenseField(row, "missing", "title"); got != "Other" {
		t.Fatalf("fallback = %q", got)
	}
}
