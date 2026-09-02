package buddy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	bareVersionRe     = regexp.MustCompile(`^\d+\.\d+(?:\.\d+)*$`)
	pkgVersionRe      = regexp.MustCompile(`-(\d+\.\d+(?:\.\d+)*)(?:-arm64|-amd64)?\.pkg$`)
	pkgVersionSpaceRe = regexp.MustCompile(`[\s_](\d+\.\d+(?:\.\d+)*)(?:-arm64|-amd64)?\.pkg$`)
	dmgVersionRe      = regexp.MustCompile(`[-_\s]?(\d+\.\d+(?:\.\d+)*)\.dmg$`)
	versionTokenRe    = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)
)

// ExtractVersion returns a dotted-numeric version from s.
// Bare versions (e.g. "1.2.3") are returned as-is. Otherwise the best match
// is taken from package/policy-style freeform text (e.g. "GoogleChrome-130.0.6723.59.pkg").
// Returns "" when no version is found.
func ExtractVersion(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if bareVersionRe.MatchString(s) {
		return s
	}
	name := strings.ToLower(s)
	if m := pkgVersionRe.FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	if m := pkgVersionSpaceRe.FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	if m := dmgVersionRe.FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	matches := versionTokenRe.FindAllString(name, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return ""
}

// CompareVersions compares two version strings (or freeform strings containing versions).
// Missing/empty versions sort lowest. Returns -1, 0, or 1.
func CompareVersions(a, b string) int {
	va := ExtractVersion(a)
	vb := ExtractVersion(b)
	if va == "" && vb == "" {
		return 0
	}
	if va == "" {
		return -1
	}
	if vb == "" {
		return 1
	}
	return compareVersionParts(va, vb)
}

func compareVersionParts(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var na, nb int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &na)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &nb)
		}
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
	}
	return 0
}

// SortByVersion returns a stably sorted copy of strs by extracted version.
// order is "asc" (default) or "desc". Unversioned strings sort after versioned
// items for asc and before them for desc.
func SortByVersion(strs []string, order string) ([]string, error) {
	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return nil, fmt.Errorf("order must be \"asc\" or \"desc\", got %q", order)
	}
	desc := order == "desc"
	out := make([]string, len(strs))
	copy(out, strs)
	sort.SliceStable(out, func(i, j int) bool {
		return VersionSortLess(out[i], out[j], desc)
	})
	return out, nil
}

// VersionSortLess reports whether a should come before b when sorting by version.
// Unversioned items sort after versioned (asc) / before (desc).
func VersionSortLess(a, b string, desc bool) bool {
	va := ExtractVersion(a)
	vb := ExtractVersion(b)
	if va == "" && vb == "" {
		return false
	}
	if va == "" {
		return desc
	}
	if vb == "" {
		return !desc
	}
	cmp := compareVersionParts(va, vb)
	if desc {
		return cmp > 0
	}
	return cmp < 0
}

// SortPackageRecordsByVersion stably sorts packages by version extracted from PackageName.
// order is "asc" or "desc" (default "desc" when empty — latest first for CLI list).
func SortPackageRecordsByVersion(pkgs []PackageRecord, order string) error {
	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return fmt.Errorf("order must be \"asc\" or \"desc\", got %q", order)
	}
	desc := order == "desc"
	sort.SliceStable(pkgs, func(i, j int) bool {
		return VersionSortLess(pkgs[i].PackageName, pkgs[j].PackageName, desc)
	})
	return nil
}
