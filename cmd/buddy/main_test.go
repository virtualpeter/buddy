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
