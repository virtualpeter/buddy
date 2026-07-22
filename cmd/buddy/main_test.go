package main

import (
	"reflect"
	"strings"
	"testing"
)

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

	argv = []string{"buddy", "auth", "token", "--raw", "--cred=default"}
	g, rest = extractGlobalFlags(argv)
	if !g.usedCred || g.cred != "default" {
		t.Fatalf("cred = %q usedCred=%v", g.cred, g.usedCred)
	}
	want = []string{"buddy", "auth", "token", "--raw"}
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
	// runAuth treats "auth refresh help" via subcommand help check in token/refresh branches.
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

func TestParseCredAddArgs_TenantIDEqualsAndSpace(t *testing.T) {
	tid := "e77c1408-10c8-4007-b177-abc9157fbcaa"
	for _, args := range [][]string{
		{"--name=gw", "--url=https://us.apigw.jamf.com", "--type=oauth", "--client-id=cid", "--tenant-id=" + tid},
		{"--name", "gw", "--url", "https://us.apigw.jamf.com", "--type", "oauth", "--client-id", "cid", "--tenant-id", tid},
	} {
		got, err := parseCredAddArgs(args, false)
		if err != nil {
			t.Fatalf("parseCredAddArgs(%v): %v", args, err)
		}
		if got.Name != "gw" {
			t.Fatalf("Name = %q", got.Name)
		}
		if got.Cred.TenantID != tid {
			t.Fatalf("TenantID = %q, want %q (args=%v)", got.Cred.TenantID, tid, args)
		}
		if got.Cred.Auth.ClientID != "cid" {
			t.Fatalf("ClientID = %q", got.Cred.Auth.ClientID)
		}
	}
}

func TestParseCredAddArgs_RegionBuildsURL(t *testing.T) {
	tid := "e77c1408-10c8-4007-b177-abc9157fbcaa"
	got, err := parseCredAddArgs([]string{
		"--name=gw", "--region=us", "--tenant-id", tid, "--client-id=cid",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cred.Region != "us" {
		t.Fatalf("Region = %q", got.Cred.Region)
	}
	if got.Cred.Auth.Type != "oauth" {
		t.Fatalf("Type = %q (region should default oauth)", got.Cred.Auth.Type)
	}
	if got.Cred.URL != "" {
		t.Fatalf("URL should be empty when only region set, got %q", got.Cred.URL)
	}
	if got.Cred.EffectiveBaseURL() != "https://us.apigw.jamf.com" {
		t.Fatalf("EffectiveBaseURL = %q", got.Cred.EffectiveBaseURL())
	}
	if err := got.Cred.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseCredAddArgs_RejectsPositionals(t *testing.T) {
	_, err := parseCredAddArgs([]string{
		"gw", "https://us.apigw.jamf.com", "--type=oauth", "--client-id=cid",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("expected positional rejection, got %v", err)
	}
}

func TestParseCredAddArgs_UnknownFlag(t *testing.T) {
	_, err := parseCredAddArgs([]string{
		"--name=gw", "--url=https://example.jamfcloud.com", "--type=oauth", "--client-id=cid", "--tenant", "x",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestParseCredAddArgs_FlagRequiresValue(t *testing.T) {
	_, err := parseCredAddArgs([]string{
		"--name=gw", "--region=us", "--client-id=cid", "--tenant-id",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "requires a value") {
		t.Fatalf("expected requires value error, got %v", err)
	}
}

func TestParseCredAddArgs_GatewayRequiresTenantViaValidate(t *testing.T) {
	got, err := parseCredAddArgs([]string{
		"--name=gw", "--region=us", "--client-id=cid",
	}, false)
	if err != nil {
		t.Fatalf("parse ok without tenant: %v", err)
	}
	if err := got.Cred.Validate(); err == nil {
		t.Fatal("Validate should reject platform without tenantId")
	} else if !strings.Contains(err.Error(), "tenantId") {
		t.Fatalf("Validate error = %v", err)
	}
}
