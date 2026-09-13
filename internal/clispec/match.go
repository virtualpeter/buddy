package clispec

import (
	"sort"
	"strings"
)

// MatchToken resolves got against candidates (case-insensitive).
// Exact match wins. Otherwise a unique prefix of a candidate wins.
// Duplicate candidate spellings that equal-fold are one hit.
// Returns ("", matches) when two or more candidates prefix-match (ambiguous).
// Returns ("", nil) when nothing matches.
func MatchToken(got string, candidates []string) (match string, ambiguous []string) {
	got = strings.ToLower(strings.TrimSpace(got))
	if got == "" || len(candidates) == 0 {
		return "", nil
	}
	for _, c := range candidates {
		if strings.EqualFold(c, got) {
			return c, nil
		}
	}
	var hits []string
	seen := map[string]bool{}
	for _, c := range candidates {
		cl := strings.ToLower(c)
		if !strings.HasPrefix(cl, got) || seen[cl] {
			continue
		}
		seen[cl] = true
		hits = append(hits, c)
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) > 1 {
		return "", hits
	}
	return "", nil
}

// ResolveCommand maps a typed token to a dispatch ID (exact, alias, or unique prefix).
// ok is true when exactly one command matches. ambiguous is sorted dispatch IDs when
// two or more commands share the prefix. Unknown tokens return ok=false and ambiguous=nil;
// canonical is the original name (possibly trimmed) so callers can try a script path.
func ResolveCommand(name string) (canonical string, ambiguous []string, ok bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return name, nil, false
	}
	for _, n := range DispatchNames() {
		if n == key {
			return n, nil, true
		}
	}
	if id, found := CommandAliases()[key]; found {
		return id, nil, true
	}
	ids := uniquePrefixCommandIDs(key)
	if len(ids) == 1 {
		return ids[0], nil, true
	}
	if len(ids) > 1 {
		sort.Strings(ids)
		return "", ids, false
	}
	return name, nil, false
}

func uniquePrefixCommandIDs(key string) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, n := range DispatchNames() {
		if strings.HasPrefix(n, key) {
			add(n)
		}
	}
	for alias, id := range CommandAliases() {
		if strings.HasPrefix(alias, key) {
			add(id)
		}
	}
	return ids
}
