package buddy

import (
	"sort"
	"strings"
)

// RelatedPolicyRole classifies a candidate during import interview.
type RelatedPolicyRole string

const (
	RelatedRoleSS        RelatedPolicyRole = "ss"
	RelatedRoleInstall   RelatedPolicyRole = "install"
	RelatedRoleCache     RelatedPolicyRole = "cache"
	RelatedRoleUninstall RelatedPolicyRole = "uninstall"
	RelatedRoleOther     RelatedPolicyRole = "other"
)

// RelatedPolicyHit is one ranked candidate for an app slug.
type RelatedPolicyHit struct {
	Summary PolicyCacheSummary
	Role    RelatedPolicyRole
	Score   int
}

// RelatedPolicyGroups partitions ranked hits by role for interview display.
type RelatedPolicyGroups struct {
	SS        []RelatedPolicyHit
	Install   []RelatedPolicyHit
	Cache     []RelatedPolicyHit
	Uninstall []RelatedPolicyHit
	Other     []RelatedPolicyHit
}

// RankRelatedPolicies scores cache summaries against an app slug.
// Loose token / trigger / package matching — suggestions only, not auto-claims.
func RankRelatedPolicies(slug string, summaries []PolicyCacheSummary) RelatedPolicyGroups {
	slug = normalizeAppSlugKey(slug)
	var hits []RelatedPolicyHit
	for _, s := range summaries {
		score := relatedPolicyScore(slug, s)
		if score <= 0 {
			continue
		}
		hits = append(hits, RelatedPolicyHit{
			Summary: s,
			Role:    classifyRelatedRole(s),
			Score:   score,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Summary.ID < hits[j].Summary.ID
	})
	var g RelatedPolicyGroups
	for _, h := range hits {
		switch h.Role {
		case RelatedRoleSS:
			g.SS = append(g.SS, h)
		case RelatedRoleInstall:
			g.Install = append(g.Install, h)
		case RelatedRoleCache:
			g.Cache = append(g.Cache, h)
		case RelatedRoleUninstall:
			g.Uninstall = append(g.Uninstall, h)
		default:
			g.Other = append(g.Other, h)
		}
	}
	return g
}

func relatedPolicyScore(slug string, s PolicyCacheSummary) int {
	if slug == "" {
		return 0
	}
	score := 0
	name := strings.ToLower(s.Name)
	trig := normalizeTriggerForMatch(s.Trigger)
	if policyNameContainsAppSlug(name, slug) || appSlugsCompatible(policyAppNameFromListName(s.Name), slug) {
		score += 10
	} else if policyListNameHasAppHint(s.Name, slug) {
		score += 6
	}
	if trig != "" {
		trigName, _, _ := parseTrigger(trig)
		if appSlugsCompatible(trigName, slug) {
			score += 8
		} else if policyNameContainsAppSlug(trig, slug) {
			score += 5
		}
		if looksLikeUninstallTrigger(trig) && (policyNameContainsAppSlug(trig, slug) || appSlugsCompatible(trigName, slug)) {
			score += 4
		}
	}
	for _, pkg := range s.PackageNames {
		if policyNameContainsAppSlug(pkg, slug) {
			score += 4
			break
		}
	}
	for _, param := range s.ScriptParams {
		p := normalizeTriggerForMatch(param)
		if p == "" {
			continue
		}
		if looksLikeInstallTrigger(p) || looksLikeCacheTrigger(p) || looksLikeUninstallTrigger(p) {
			pn, _, _ := parseTrigger(p)
			if appSlugsCompatible(pn, slug) || policyNameContainsAppSlug(p, slug) {
				score += 7
				break
			}
		}
	}
	return score
}

func classifyRelatedRole(s PolicyCacheSummary) RelatedPolicyRole {
	trig := normalizeTriggerForMatch(s.Trigger)
	nameLower := strings.ToLower(s.Name)
	if looksLikeUninstallTrigger(trig) || strings.Contains(nameLower, "uninstall") {
		return RelatedRoleUninstall
	}
	if s.UseForSelfService {
		return RelatedRoleSS
	}
	if looksLikeCacheTrigger(trig) || strings.Contains(nameLower, "cache ") || strings.HasPrefix(nameLower, "cache ") {
		return RelatedRoleCache
	}
	if looksLikeInstallTrigger(trig) || strings.Contains(nameLower, "install ") {
		return RelatedRoleInstall
	}
	for _, param := range s.ScriptParams {
		p := normalizeTriggerForMatch(param)
		if looksLikeInstallTrigger(p) {
			return RelatedRoleSS // script-driven SS stub often has no SS flag? still prefer SS if use_for_self_service
		}
	}
	if ssNameMarkerRe.MatchString(s.Name) {
		return RelatedRoleSS
	}
	return RelatedRoleOther
}

func looksLikeUninstallTrigger(s string) bool {
	t := normalizeTriggerForMatch(s)
	if t == "" {
		return false
	}
	return strings.HasPrefix(t, "uninstall-") || strings.HasSuffix(t, "-uninstall") || strings.Contains(t, "uninstall")
}
