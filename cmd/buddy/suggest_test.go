package main

import (
	"strings"
	"testing"
)

func TestSuggestTopLevelCommand(t *testing.T) {
	cases := map[string]string{
		"blueprints":    "blueprint",
		"licenses":      "license",
		"blueprint":     "blueprint", // exact (case fold handled separately)
		"policies":      "policy",
		"packages":      "package",
		"devices":       "device",
		"device-groups": "device-group",
		"creds":         "cred",
		"credentials":   "cred",
		"ea":            "extension-attribute",
		"eas":           "extension-attribute",
		"mobile-ea":     "mobile-extension-attribute",
		"bluepritn":     "blueprint", // transposition-ish
		"profil":        "profile",
		"xyzzy":         "",
	}
	for in, want := range cases {
		got := suggestTopLevelCommand(in)
		if in == "blueprint" {
			// suggestClosest returns exact match; alias path unused
			if got != "blueprint" {
				t.Errorf("suggestTopLevelCommand(%q)=%q want blueprint", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("suggestTopLevelCommand(%q)=%q want %q", in, got, want)
		}
	}
	if got := suggestTopLevelCommand("BLUEPRINTS"); got != "blueprint" {
		t.Errorf("case fold alias: got %q", got)
	}
}

func TestMatchSubcommandPrefix(t *testing.T) {
	got, err := matchSubcommand("policy", "li", []string{"list", "get", "logs"})
	if err != nil || got != "list" {
		t.Fatalf("li → %q %v want list", got, err)
	}
	_, err = matchSubcommand("policy", "l", []string{"list", "get", "logs"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("l want ambiguous, got %v", err)
	}
	_, err = matchSubcommand("icon", "xyzzy", []string{"list", "get"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("xyzzy want unknown, got %v", err)
	}
}

func TestSuggestClosestSubcommands(t *testing.T) {
	cands := []string{"list", "get", "create", "set", "delete", "deploy", "undeploy", "status", "components"}
	if got := suggestClosest("lists", cands); got != "list" {
		t.Errorf("lists → %q want list", got)
	}
	if got := suggestClosest("deply", cands); got != "deploy" {
		t.Errorf("deply → %q want deploy", got)
	}
	if got := suggestClosest("zzzz", cands); got != "" {
		t.Errorf("zzzz → %q want empty", got)
	}
}

func TestEditDistance(t *testing.T) {
	if d := editDistance("kitten", "sitting"); d != 3 {
		t.Fatalf("kitten/sitting = %d want 3", d)
	}
	if d := editDistance("blueprint", "blueprints"); d != 1 {
		t.Fatalf("blueprint/blueprints = %d want 1", d)
	}
}
