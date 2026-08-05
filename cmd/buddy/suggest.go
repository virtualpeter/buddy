package main

import (
	"fmt"
	"sort"
	"strings"
)

// Common plural / alias typos for top-level commands (before edit-distance).
var commandAliases = map[string]string{
	"blueprints":    "blueprint",
	"policies":      "policy",
	"categories":    "category",
	"macapps":       "macapp",
	"apps":          "app",
	"profiles":      "profile",
	"packages":      "package",
	"computers":     "computer",
	"devices":       "device",
	"device-groups": "device-group",
	"groups":        "group",
	"scripts":       "script",
	"benchmarks":    "benchmark",
	"roles":         "role",
	"clients":       "client",
	"creds":         "cred",
	"credentials":   "cred",
	"auths":         "auth",
}

func knownCommandNames() []string {
	names := make([]string, 0, len(knownCommands))
	for n := range knownCommands {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// suggestClosest returns the best candidate for a mistyped name, or "".
// Prefers aliases, then small edit distance / singularization.
func suggestClosest(input string, candidates []string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || len(candidates) == 0 {
		return ""
	}
	for _, c := range candidates {
		if strings.EqualFold(c, input) {
			return c
		}
	}

	best := ""
	bestDist := 1 << 30
	for _, c := range candidates {
		cl := strings.ToLower(c)
		d := editDistance(input, cl)
		// "blueprints" → "blueprint", "lists" → "list"
		if strings.TrimSuffix(input, "s") == cl || strings.TrimSuffix(input, "es") == cl {
			if d > 1 {
				d = 1
			}
		}
		// prefix: typed command starts with candidate or vice versa with small length gap
		if strings.HasPrefix(input, cl) || strings.HasPrefix(cl, input) {
			gap := abs(len(input) - len(cl))
			if gap <= 3 && gap < d {
				d = gap
			}
		}
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	maxDist := 2
	if len(input) >= 8 {
		maxDist = 3
	}
	if bestDist <= maxDist {
		return best
	}
	return ""
}

func suggestTopLevelCommand(input string) string {
	key := strings.ToLower(strings.TrimSpace(input))
	if alias, ok := commandAliases[key]; ok {
		if isKnownCommand(alias) {
			return alias
		}
	}
	return suggestClosest(input, knownCommandNames())
}

func formatDidYouMean(suggestion string) string {
	if suggestion == "" {
		return ""
	}
	return fmt.Sprintf("\nDid you mean %q?", suggestion)
}

// errUnknownSubcommand builds a consistent unknown-subcommand error with optional suggestion.
func errUnknownSubcommand(command, got string, candidates []string) error {
	msg := fmt.Sprintf("unknown %s subcommand: %q", command, got)
	msg += formatDidYouMean(suggestClosest(got, candidates))
	msg += fmt.Sprintf(" (use: %s)", strings.Join(candidates, ", "))
	return fmt.Errorf("%s", msg)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// editDistance is classic Levenshtein distance.
func editDistance(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	// Ensure a is shorter for slightly less memory.
	if len(a) > len(b) {
		a, b = b, a
	}
	prev := make([]int, len(a)+1)
	cur := make([]int, len(a)+1)
	for i := range prev {
		prev[i] = i
	}
	for j := 1; j <= len(b); j++ {
		cur[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[i] + 1
			ins := cur[i-1] + 1
			sub := prev[i-1] + cost
			cur[i] = min3(del, ins, sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(a)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
