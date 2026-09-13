package buddy

import (
	"strings"
	"testing"
)

const instanceClientJSON = `{"client_name":"buddy-cli","client_id":"11111111-1111-1111-1111-111111111111","client_secret":"instance-secret","grant_type":"client_credentials","content_type":"application/x-www-form-urlencoded"}`

const platformOrgCurl = `curl --request POST \
--url https://us.api.jamfcloud.com/auth/token \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data 'grant_type=client_credentials' --data 'client_id=22222222-2222-2222-2222-222222222222' \
--data 'client_secret=org-secret'`

const platformOrgAliasCurl = `curl --request POST \
--url https://api.jamfcloud.com/auth/token \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data 'grant_type=client_credentials' --data 'client_id=22222222-2222-2222-2222-222222222222' \
--data 'client_secret=org-secret'`

const platformEnvCurl = `curl --request POST \
--url https://us.api.jamfcloud.com/auth/token \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data 'grant_type=client_credentials' \
--data 'client_id=33333333-3333-3333-3333-333333333333' \
--data 'client_secret=env-secret'`

func TestParsePastedClientJSONInstance(t *testing.T) {
	got, err := ParsePastedClientCreds(instanceClientJSON)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != AuthTypeOAuth || got.ClientID != "11111111-1111-1111-1111-111111111111" || got.ClientSecret != "instance-secret" || got.ClientName != "buddy-cli" {
		t.Fatalf("%+v", got)
	}
	if got.Region != "" || got.InstanceURL != "" {
		t.Fatalf("instance JSON should not set platform/url: %+v", got)
	}
}

func TestParsePastedClientJSONFence(t *testing.T) {
	raw := "```json\n" + instanceClientJSON + "\n```"
	got, err := ParsePastedClientCreds(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientName != "buddy-cli" {
		t.Fatalf("%+v", got)
	}
}

func TestParsePastedClientCurlPlatform(t *testing.T) {
	org, err := ParsePastedClientCreds(platformOrgCurl)
	if err != nil {
		t.Fatal(err)
	}
	if org.Type != AuthTypePlatform || org.Scope != "" || org.Region != "us" || org.ClientID != "22222222-2222-2222-2222-222222222222" || org.ClientSecret != "org-secret" {
		t.Fatalf("us org-admin curl is region us until cred add (env vs org): %+v", org)
	}
	alias, err := ParsePastedClientCreds(platformOrgAliasCurl)
	if err != nil {
		t.Fatal(err)
	}
	if alias.Type != AuthTypePlatform || alias.Scope != PlatformScopeOrganization || alias.Region != "" {
		t.Fatalf("api.jamfcloud.com sample (NXDOMAIN) maps to org: %+v", alias)
	}
	env, err := ParsePastedClientCreds(platformEnvCurl)
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != AuthTypePlatform || env.Scope != "" || env.Region != "us" || env.ClientID != "33333333-3333-3333-3333-333333333333" || env.ClientSecret != "env-secret" {
		t.Fatalf("env curl: %+v", env)
	}
}

func TestParsePastedClientCurlCombinedData(t *testing.T) {
	raw := `curl --request POST --url https://eu.api.jamfcloud.com/oauth2/token --data 'grant_type=client_credentials&client_id=cid&client_secret=s3cret'`
	got, err := ParsePastedClientCreds(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != AuthTypePlatform || got.Region != "eu" || got.ClientID != "cid" || got.ClientSecret != "s3cret" {
		t.Fatalf("%+v", got)
	}
}

func TestParsePastedClientCurlInstanceToken(t *testing.T) {
	raw := `curl -X POST --url https://example.jamfcloud.com/api/oauth/token -d 'grant_type=client_credentials' -d 'client_id=cid' -d 'client_secret=s3cret'`
	got, err := ParsePastedClientCreds(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != AuthTypeOAuth || got.InstanceURL != "https://example.jamfcloud.com" || got.ClientID != "cid" {
		t.Fatalf("%+v", got)
	}
}

func TestParsePastedClientCredsErrors(t *testing.T) {
	if _, err := ParsePastedClientCreds("   "); err == nil {
		t.Fatal("empty")
	}
	if _, err := ParsePastedClientCreds(`{"client_id":"cid"}`); err == nil || !strings.Contains(err.Error(), "client_secret") {
		t.Fatalf("json missing secret: %v", err)
	}
	if _, err := ParsePastedClientCreds("curl --url https://example.com"); err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Fatalf("curl missing ids: %v", err)
	}
}

func TestApplyPastedClientCredsDoesNotOverwrite(t *testing.T) {
	opts := CredAddOpts{ClientID: "keep", Type: AuthTypeOAuth}
	applyPastedClientCreds(&opts, &PastedClientCreds{ClientID: "new", ClientSecret: "s", Type: AuthTypePlatform, Region: "us"})
	if opts.ClientID != "keep" || opts.ClientSecret != "s" || opts.Type != AuthTypeOAuth || opts.Region != "us" {
		t.Fatalf("%+v", opts)
	}
}
