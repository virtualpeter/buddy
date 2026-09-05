package main

import (
	"reflect"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/buddy/secrets"
	"buddy/internal/clispec"
)

type mockSecretStore struct {
	values map[string]string
}

func (m *mockSecretStore) key(credName string, kind secrets.Kind) string {
	return credName + "/" + kind.String()
}

func (m *mockSecretStore) Get(credName string, kind secrets.Kind) (string, error) {
	if v, ok := m.values[m.key(credName, kind)]; ok && v != "" {
		return v, nil
	}
	return "", secrets.ErrNotFound
}

func (m *mockSecretStore) Set(credName string, kind secrets.Kind, value string) error {
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[m.key(credName, kind)] = value
	return nil
}

func (m *mockSecretStore) Delete(credName string, kind secrets.Kind) error {
	delete(m.values, m.key(credName, kind))
	return nil
}

func (m *mockSecretStore) Has(credName string, kind secrets.Kind) (bool, error) {
	v, ok := m.values[m.key(credName, kind)]
	return ok && v != "", nil
}

func (m *mockSecretStore) List() ([]secrets.Entry, error) {
	var out []secrets.Entry
	for k := range m.values {
		parts := strings.SplitN(k, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind, err := secrets.ParseKind(parts[1])
		if err != nil {
			continue
		}
		out = append(out, secrets.Entry{AuthName: parts[0], Kind: kind})
	}
	return out, nil
}

func (m *mockSecretStore) Description() string { return "mock" }

func TestParseInt_Valid(t *testing.T) {
	got, err := parseInt("42", "id")
	if err != nil {
		t.Fatalf("parseInt(\"42\", \"id\"): %v", err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	got, err = parseInt("  1  ", "id")
	if err != nil {
		t.Fatalf("parseInt(\"  1  \", \"id\"): %v", err)
	}
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestParseInt_Invalid(t *testing.T) {
	tests := []struct {
		in   string
		name string
	}{
		{"", "id"},
		{"0", "id"},
		{"-1", "id"},
		{"x", "id"},
	}
	for _, tt := range tests {
		_, err := parseInt(tt.in, tt.name)
		if err == nil {
			t.Errorf("parseInt(%q, %q): expected error", tt.in, tt.name)
		}
		if !strings.Contains(err.Error(), "must be positive integer") {
			t.Errorf("parseInt(%q): error = %v", tt.in, err)
		}
	}
}

func TestParseParamVal_Valid(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"4", []int{4}},
		{"4,5,6", []int{4, 5, 6}},
		{"4, 5, 6", []int{4, 5, 6}},
		{"11", []int{11}},
	}
	for _, tt := range tests {
		got, err := parseParamVal(tt.in)
		if err != nil {
			t.Fatalf("parseParamVal(%q): %v", tt.in, err)
		}
		if len(got) != len(tt.want) {
			t.Errorf("parseParamVal(%q): got %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseParamVal(%q): got %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

func TestParseParamVal_Invalid(t *testing.T) {
	tests := []string{"3", "12", "x", ""}
	for _, in := range tests {
		_, err := parseParamVal(in)
		if err == nil {
			t.Errorf("parseParamVal(%q): expected error", in)
		}
		if err != nil && !strings.Contains(err.Error(), "4-11") {
			t.Errorf("parseParamVal(%q): error = %v", in, err)
		}
	}
}

func TestParseParamArgs_NoFlags(t *testing.T) {
	args := []string{"a", "b"}
	indices, filtered, err := parseParamArgs(args)
	if err != nil {
		t.Fatalf("parseParamArgs: %v", err)
	}
	if indices[0] != 4 || indices[1] != 5 || indices[2] != 6 {
		t.Errorf("got default param indices %v, want [4 5 6]", indices)
	}
	if len(filtered) != 2 || filtered[0] != "a" || filtered[1] != "b" {
		t.Errorf("got filtered %v, want [a b]", filtered)
	}
}

func TestParseParamArgs_ParamEquals(t *testing.T) {
	args := []string{"a", "--param=5,6,7", "b"}
	indices, filtered, err := parseParamArgs(args)
	if err != nil {
		t.Fatalf("parseParamArgs: %v", err)
	}
	if len(indices) != 3 || indices[0] != 5 || indices[1] != 6 || indices[2] != 7 {
		t.Errorf("got param indices %v, want [5 6 7]", indices)
	}
	if len(filtered) != 2 || filtered[0] != "a" || filtered[1] != "b" {
		t.Errorf("got filtered %v, want [a b]", filtered)
	}
}

func TestParseParamArgs_ParamSpace(t *testing.T) {
	// --param 5,6 parses correctly; positional "x" is retained in filtered
	args := []string{"x", "--param", "5,6"}
	indices, filtered, err := parseParamArgs(args)
	if err != nil {
		t.Fatalf("parseParamArgs: %v", err)
	}
	if len(indices) != 2 || indices[0] != 5 || indices[1] != 6 {
		t.Errorf("got param indices %v, want [5 6]", indices)
	}
	if len(filtered) != 1 || filtered[0] != "x" {
		t.Errorf("got filtered %v, want [x]", filtered)
	}
}

func TestParseParamArgs_ParamNoValue(t *testing.T) {
	args := []string{"--param"}
	_, _, err := parseParamArgs(args)
	if err == nil {
		t.Error("expected error when --param has no value")
	}
	if !strings.Contains(err.Error(), "requires a value") {
		t.Errorf("error = %v", err)
	}
}

func TestParsePolicySSArgs_Empty(t *testing.T) {
	_, _, err := parsePolicySSArgs(nil)
	if err == nil {
		t.Error("expected error for empty args")
	}
	if !strings.Contains(err.Error(), "missing policy-id") {
		t.Errorf("error = %v", err)
	}
}

func TestParsePolicySSArgs_OnlyPolicyID(t *testing.T) {
	_, _, err := parsePolicySSArgs([]string{"123"})
	if err == nil {
		t.Error("expected error when only policy-id (no version or flags)")
	}
	if !strings.Contains(err.Error(), "missing version or flags") {
		t.Errorf("error = %v", err)
	}
}

func TestParsePolicySSArgs_PolicyIDAndVersion(t *testing.T) {
	opts, policyID, err := parsePolicySSArgs([]string{"456", "1.0.0"})
	if err != nil {
		t.Fatalf("parsePolicySSArgs: %v", err)
	}
	if policyID != 456 {
		t.Errorf("policyID = %d, want 456", policyID)
	}
	if opts.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", opts.Version)
	}
}

func TestParsePolicySSArgs_WithDisplayNameAndGroupIDs(t *testing.T) {
	opts, policyID, err := parsePolicySSArgs([]string{"--display-name=foo", "789", "2.0.0", "--group-ids=1,2"})
	if err != nil {
		t.Fatalf("parsePolicySSArgs: %v", err)
	}
	if policyID != 789 {
		t.Errorf("policyID = %d, want 789", policyID)
	}
	if opts.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", opts.Version)
	}
	if opts.DisplayName == nil || *opts.DisplayName != "foo" {
		t.Errorf("DisplayName = %v, want ptr to \"foo\"", opts.DisplayName)
	}
	if len(opts.GroupIDs) != 2 || opts.GroupIDs[0] != 1 || opts.GroupIDs[1] != 2 {
		t.Errorf("GroupIDs = %v, want [1 2]", opts.GroupIDs)
	}
}

func TestParsePolicySSArgs_InvalidPolicyID(t *testing.T) {
	_, _, err := parsePolicySSArgs([]string{"x", "1.0.0"})
	if err == nil {
		t.Error("expected error for invalid policy-id")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error = %v", err)
	}
}

func TestExtractGlobalFlags(t *testing.T) {
	argv := []string{"buddy", "cred", "secret", "path", "-v", "-t"}
	g, rest := extractGlobalFlags(argv)
	if !g.verbose || !g.trace {
		t.Fatalf("verbose=%v trace=%v, want both true", g.verbose, g.trace)
	}
	want := []string{"buddy", "cred", "secret", "path"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}

	argv = []string{"buddy", "auth", "token", "show", "--raw", "--user=default"}
	g, rest = extractGlobalFlags(argv)
	if g.user != "default" {
		t.Fatalf("user = %q, want default", g.user)
	}
	want = []string{"buddy", "auth", "token", "show", "--raw"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}

	argv = []string{"buddy", "auth", "token", "show", "--raw", "--cred=default"}
	g, rest = extractGlobalFlags(argv)
	if !g.rejectCred {
		t.Fatal("expected rejectCred for removed --cred")
	}
	want = []string{"buddy", "auth", "token", "show", "--raw"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}

	argv = []string{"buddy", "--user", "dev", "auth", "test"}
	g, rest = extractGlobalFlags(argv)
	if g.user != "dev" {
		t.Fatalf("user = %q, want dev", g.user)
	}

	argv = []string{"buddy", "-e", "1+1", "--", "foo"}
	g, rest = extractGlobalFlags(argv)
	if g.evalSource != "1+1" || g.evalDup || g.evalMissing {
		t.Fatalf("eval: source=%q dup=%v missing=%v", g.evalSource, g.evalDup, g.evalMissing)
	}
	want = []string{"buddy", "--", "foo"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}

	argv = []string{"buddy", "--eval=print(len(args))", "bar"}
	g, rest = extractGlobalFlags(argv)
	if g.evalSource != "print(len(args))" {
		t.Fatalf("evalSource = %q", g.evalSource)
	}
	want = []string{"buddy", "bar"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}

	argv = []string{"buddy", "--eval", "x", "-e", "y"}
	g, rest = extractGlobalFlags(argv)
	if !g.evalDup || g.evalSource != "x" {
		t.Fatalf("want dup with first source, got dup=%v source=%q", g.evalDup, g.evalSource)
	}

	argv = []string{"buddy", "-e"}
	g, _ = extractGlobalFlags(argv)
	if !g.evalMissing {
		t.Fatal("expected evalMissing for bare -e")
	}
}

func TestRunAuthRefreshHelp(t *testing.T) {
	// extractGlobalFlags does not rewrite "auth refresh" to "auth token refresh".
	argv := []string{"buddy", "auth", "refresh", "help"}
	g, rest := extractGlobalFlags(argv)
	if g.showVersion {
		t.Fatal("unexpected version flag")
	}
	want := []string{"buddy", "auth", "refresh", "help"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest = %v, want %v", rest, want)
	}
}

func TestAuthTokenBareShowsHelp(t *testing.T) {
	err := runAuthToken("", nil, nil)
	if err != nil {
		t.Fatalf("bare token should print help, got %v", err)
	}
}

func TestAuthTokenRawFlagHintsShow(t *testing.T) {
	err := runAuthToken("", []string{"--raw"}, nil)
	if err == nil || !strings.Contains(err.Error(), "auth token show") {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthTokenShowPrintsToken(t *testing.T) {
	err := runAuthToken("", []string{"show"}, nil)
	if err == nil {
		t.Fatal("expected session resolve error")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("show parsed as flag: %v", err)
	}
	if strings.Contains(err.Error(), "unknown") {
		t.Fatalf("show not recognised: %v", err)
	}
}

func TestAuthTokenRejectsMovedDirectFlags(t *testing.T) {
	err := runAuthToken("", []string{"--url=https://example.jamfcloud.com", "--username=admin", "--password=secret"}, nil)
	if err == nil || !strings.Contains(err.Error(), "buddy token direct") {
		t.Fatalf("err = %v", err)
	}
}

func TestTokenDirectRequiresFlags(t *testing.T) {
	err := runTokenDirect(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "buddy token help") {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthTokenStatusAndDropAreSubcommands(t *testing.T) {
	err := runAuthToken("", []string{"status"}, nil)
	if err == nil {
		t.Fatal("expected session resolve error")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("status parsed as flag: %v", err)
	}

	err = runAuthToken("", []string{"drop"}, nil)
	if err == nil {
		t.Fatal("expected session resolve error")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("drop parsed as flag: %v", err)
	}
}

func TestAuthRefreshMovedToTokenRefresh(t *testing.T) {
	err := runAuth("", buddy.Opts{}, []string{clispec.SubRefresh}, nil)
	if err == nil || !strings.Contains(err.Error(), "auth token refresh") {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthCapsMovedToAuthorization(t *testing.T) {
	err := runAuth("", buddy.Opts{}, []string{"caps"}, nil)
	if err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthTokenRefreshDoesNotRequirePastedToken(t *testing.T) {
	err := runAuthTokenRefresh("", nil, nil)
	if err == nil {
		t.Fatal("expected session resolve error")
	}
	if strings.Contains(err.Error(), "token is required") {
		t.Fatalf("refresh still requires pasted token: %v", err)
	}

	err = runAuthTokenRefresh("", []string{"eyJhbGciOiJIUzI1NiJ9.aaa"}, nil)
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("want unexpected argument, got %v", err)
	}
}

func TestStaticGroupSplitFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFlags []string
		wantPos   []string
	}{
		{
			name:      "positional before long flag with equals",
			args:      []string{"5", "--computer-ids=1"},
			wantFlags: []string{"--computer-ids=1"},
			wantPos:   []string{"5"},
		},
		{
			name:      "flag before positional",
			args:      []string{"--computer-ids=1", "5"},
			wantFlags: []string{"--computer-ids=1"},
			wantPos:   []string{"5"},
		},
		{
			name:      "positional before two-token flag",
			args:      []string{"5", "--computer-ids", "1"},
			wantFlags: []string{"--computer-ids", "1"},
			wantPos:   []string{"5"},
		},
		{
			name:      "members id then filter",
			args:      []string{"42", "--filter=lab"},
			wantFlags: []string{"--filter=lab"},
			wantPos:   []string{"42"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotF, gotP := staticGroupSplitFlags(tt.args)
			if !reflect.DeepEqual(gotF, tt.wantFlags) || !reflect.DeepEqual(gotP, tt.wantPos) {
				t.Errorf("staticGroupSplitFlags(%v) = flags %#v pos %#v; want flags %#v pos %#v",
					tt.args, gotF, gotP, tt.wantFlags, tt.wantPos)
			}
		})
	}
}

func TestParseCredSetTenantArgs(t *testing.T) {
	tid := "e77c1408-10c8-4007-b177-abc9157fbcaa"
	got, err := parseCredSetTenantArgs([]string{
		"gw", "--region=us", "--tenant-id=" + tid,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "gw" || got.Tenant.Region != "us" || got.Tenant.TenantID != tid {
		t.Fatalf("%+v", got)
	}
	if err := got.Tenant.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseCredSetAuthArgs(t *testing.T) {
	got, err := parseCredSetAuthArgs([]string{
		"ro", "--type=oauth", "--client-id=cid", "--tenants=sandbox,prod", "--access=readonly", "--surface=platform",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ro" || got.Auth.ClientID != "cid" || got.Auth.EffectiveAccess() != buddy.AccessReadonly {
		t.Fatalf("%+v", got)
	}
	if got.Auth.Type != buddy.AuthTypePlatform {
		t.Fatalf("oauth+surface=platform should fold to type platform, got %q", got.Auth.Type)
	}
	if len(got.Auth.Tenants) != 2 {
		t.Fatalf("tenants=%v", got.Auth.Tenants)
	}
}

func TestParseCredSetAuthArgs_RejectsExtraPositionals(t *testing.T) {
	_, err := parseCredSetAuthArgs([]string{"a", "b", "--type=oauth", "--client-id=c", "--tenants=t"}, false)
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("got %v", err)
	}
}

func TestSecretSelectionForAuthType(t *testing.T) {
	wantPw, wantCs, err := secretSelectionForAuthType("basic", "c")
	if err != nil || !wantPw || wantCs {
		t.Fatalf("basic => pw=%v cs=%v err=%v", wantPw, wantCs, err)
	}
	wantPw, wantCs, err = secretSelectionForAuthType("platform", "c")
	if err != nil || wantPw || !wantCs {
		t.Fatalf("platform => pw=%v cs=%v err=%v", wantPw, wantCs, err)
	}
	if _, _, err = secretSelectionForAuthType("command", "c"); err == nil {
		t.Fatal("command auth should have no stored secret")
	}
}

func TestRunCredMigrate_MovesInlineSecret(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mock := &mockSecretStore{values: map[string]string{}}
	secrets.SetStore(mock)
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &buddy.Config{
		Tenant: buddy.TenantSelection{Default: "sandbox"},
		Tenants: map[string]buddy.TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]buddy.AuthEntry{
			"sandbox": {
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "inline-secret",
				Tenants:      []string{"sandbox"},
			},
		},
	}
	if err := runCredMigrate(cfg, buddy.Opts{SkipConfirm: true}, nil); err != nil {
		t.Fatalf("runCredMigrate: %v", err)
	}
	if got := cfg.Auths["sandbox"].ClientSecret; got != "" {
		t.Fatalf("inline clientSecret not cleared: %q", got)
	}
	if v, err := mock.Get("sandbox", secrets.KindClientSecret); err != nil || v != "inline-secret" {
		t.Fatalf("store client-secret = %q, err %v", v, err)
	}
}

func TestRunCredEraseSecrets_WipesStore(t *testing.T) {
	mock := &mockSecretStore{values: map[string]string{
		"sandbox/password":      "pw",
		"sandbox/client-secret": "cs",
	}}
	secrets.SetStore(mock)
	t.Cleanup(func() { secrets.SetStore(nil) })

	if err := runCredEraseSecrets(buddy.Opts{SkipConfirm: true}); err != nil {
		t.Fatalf("runCredEraseSecrets: %v", err)
	}
	left, err := mock.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("leftover %+v", left)
	}
}

func TestParseCredSetTenant_GatewayRequiresTenantID(t *testing.T) {
	got, err := parseCredSetTenantArgs([]string{"gw", "--region=us"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Tenant.Validate(); err == nil {
		t.Fatal("Validate should reject platform without tenantId")
	} else if !strings.Contains(err.Error(), "tenantId") {
		t.Fatalf("Validate error = %v", err)
	}
}
