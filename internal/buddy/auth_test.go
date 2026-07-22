package buddy

import (
	"os"
	"testing"

	"buddy/internal/buddy/tokenauth"
)

func TestDirectAuthOptsUsesDirectAuth(t *testing.T) {
	empty := DirectAuthOpts{}
	if empty.UsesDirectAuth() {
		t.Fatal("empty opts should not use direct auth")
	}
	withURL := DirectAuthOpts{BaseURL: "https://example.jamfcloud.com"}
	if !withURL.UsesDirectAuth() {
		t.Fatal("url should trigger direct auth")
	}
	withRegion := DirectAuthOpts{Region: "us"}
	if !withRegion.UsesDirectAuth() {
		t.Fatal("region should trigger direct auth")
	}
}

func TestValidateDirectAuth(t *testing.T) {
	err := validateDirectAuth(tokenauth.Config{
		BaseURL:  "https://x.jamfcloud.com",
		Username: "u",
		Password: "p",
	})
	if err != nil {
		t.Fatalf("basic auth: %v", err)
	}
	err = validateDirectAuth(tokenauth.Config{
		BaseURL:      "https://x.jamfcloud.com",
		Username:     "u",
		Password:     "p",
		ClientId:     "id",
		ClientSecret: "sec",
	})
	if err == nil {
		t.Fatal("expected error for multiple auth methods")
	}
}

func TestLoadOAuth2CredsFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/creds.json"
	if err := os.WriteFile(path, []byte(`{"client_id":"abc","client_secret":"xyz"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	creds, err := LoadOAuth2CredsFile(path)
	if err != nil {
		t.Fatalf("LoadOAuth2CredsFile: %v", err)
	}
	if creds.ClientID != "abc" || creds.ClientSecret != "xyz" {
		t.Fatalf("got %+v", creds)
	}
}

func TestCompactAuthInfo(t *testing.T) {
	info := map[string]interface{}{
		"authenticationType": "CLIENT_CREDENTIALS",
		"account": map[string]interface{}{
			"username":     "buddy-cli",
			"accessLevel":  "FullAccess",
			"privilegeSet": "CUSTOM",
			"privilegesBySite": map[string]interface{}{
				"-1": []interface{}{"Read Computers", "Update Policies"},
			},
		},
		"sites": []interface{}{map[string]interface{}{"id": "-1", "name": "NONE"}},
	}
	s := CompactAuthInfo(info)
	if s["username"] != "buddy-cli" || s["privilegeCount"] != 2 {
		t.Fatalf("%v", s)
	}
	if _, ok := s["account"]; ok {
		t.Fatal("compact form should not include full account")
	}
}
