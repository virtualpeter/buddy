package main

import "testing"

func TestSuggestTopLevelCommand(t *testing.T) {
	cases := map[string]string{
		"blueprints":    "blueprint",
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
