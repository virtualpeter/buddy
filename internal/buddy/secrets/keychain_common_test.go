package secrets

import (
	"strings"
	"testing"
)

func TestSecurityQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
	}
	for _, tc := range cases {
		if got := securityQuote(tc.in); got != tc.want {
			t.Errorf("securityQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSecurityAddInteractiveCmdOmitsSecretFromArgs(t *testing.T) {
	secret := "super-secret-value-xyz"
	cmd, err := securityAddInteractiveCmd(keychainServiceCLI, "sandbox-admin/password", secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "security" || cmd.Args[1] != "-i" {
		t.Fatalf("args = %q", cmd.Args)
	}
	for _, a := range cmd.Args {
		if strings.Contains(a, secret) {
			t.Fatalf("secret leaked in argv: %q", cmd.Args)
		}
	}
}

func TestSecurityAddInteractiveLineTooLarge(t *testing.T) {
	big := strings.Repeat("x", securityInteractiveMax)
	_, err := securityAddInteractiveLine(keychainServiceCLI, "a/password", big)
	if err == nil {
		t.Fatal("expected error for oversized secret")
	}
}

func TestBuddyKeychainServices(t *testing.T) {
	got := BuddyKeychainServices()
	if len(got) != 2 || got[0] != keychainServiceCLI || got[1] != keychainServiceSigned {
		t.Fatalf("BuddyKeychainServices = %q", got)
	}
}

func TestKeychainSignedServiceAndLabel(t *testing.T) {
	if keychainServiceSigned != "virtualpete.buddy.cli" {
		t.Fatalf("keychainServiceSigned = %q", keychainServiceSigned)
	}
	if keychainLabelSigned != "Buddy" {
		t.Fatalf("keychainLabelSigned = %q", keychainLabelSigned)
	}
}

func TestParseKeychainDumpForService(t *testing.T) {
	dump := `
class: "genp"
attributes:
    "svce"<blob>="virtualpete.buddy.cli"
    "acct"<blob>="rel-admin/password"
class: "genp"
attributes:
    "svce"<blob>="buddy"
    "acct"<blob>="sandbox-admin/password"
`
	got := parseKeychainDumpForService(dump, keychainServiceSigned)
	if len(got) != 1 || got[0].AuthName != "rel-admin" || got[0].Kind != KindPassword {
		t.Fatalf("got %+v", got)
	}
}
