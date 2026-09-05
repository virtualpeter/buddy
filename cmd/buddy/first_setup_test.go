package main

import (
	"testing"

	"buddy/internal/clispec"
)

func TestShouldOfferFirstTimeSetup(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{clispec.CmdHelp}, false},
		{[]string{clispec.CmdVersion}, false},
		{[]string{clispec.CmdSetup, "basic-setup"}, false},
		{[]string{clispec.CmdCred, "list"}, false},
		{[]string{clispec.CmdAuth, "test"}, false},
		{[]string{clispec.CmdToken, "direct"}, false},
		{[]string{clispec.CmdServe}, false},
		{[]string{clispec.CmdLSP}, false},
		{[]string{clispec.CmdMCP}, false},
		{[]string{clispec.CmdMCP, "setup"}, false},
		{[]string{clispec.CmdPolicy, "list"}, true},
		{[]string{clispec.CmdGroup, "list"}, true},
	}
	for _, tc := range cases {
		got := shouldOfferFirstTimeSetup(tc.args)
		if got != tc.want {
			t.Errorf("shouldOfferFirstTimeSetup(%v) = %v want %v", tc.args, got, tc.want)
		}
	}
}
