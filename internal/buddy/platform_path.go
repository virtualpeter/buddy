package buddy

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	proVersionPathRE = regexp.MustCompile(`^(/api)/(v\d+)(/.*)$`)
	blueprintsPathRE = regexp.MustCompile(`^(/api/blueprints)/(v\d+)(/.*)$`)
)

// RewriteJamfPath maps an instance-relative Jamf path onto the Platform API Gateway
// when tenantID is non-empty. With an empty tenantID the path is returned unchanged
// (aside from normalization).
//
// Rules:
//   - /api/vN/rest…           → /api/pro/vN/tenant/{id}/rest…
//   - /JSSResource/…          → /api/proclassic/tenant/{id}/JSSResource/…
//   - /api/blueprints/vN/…    → inject /tenant/{id} after the version segment
//   - already contains /tenant/ → unchanged
//   - /api/schema, /classicapi → error (not supported via gateway)
func RewriteJamfPath(tenantID, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	query := ""
	if i := strings.Index(path, "?"); i >= 0 {
		query = path[i:]
		path = path[:i]
	}
	if tenantID == "" {
		return path + query, nil
	}
	if strings.Contains(path, "/tenant/") {
		return path + query, nil
	}
	if path == "/api/schema" || strings.HasPrefix(path, "/api/schema/") ||
		path == "/classicapi" || strings.HasPrefix(path, "/classicapi/") {
		return "", fmt.Errorf("path %q is not supported via the Platform API Gateway", path)
	}

	switch {
	case strings.HasPrefix(path, "/JSSResource"):
		return "/api/proclassic/tenant/" + tenantID + path + query, nil
	case blueprintsPathRE.MatchString(path):
		m := blueprintsPathRE.FindStringSubmatch(path)
		// m[1]=/api/blueprints m[2]=vN m[3]=/rest
		rest := m[3]
		if strings.HasPrefix(rest, "/tenant/") {
			return path + query, nil
		}
		return m[1] + "/" + m[2] + "/tenant/" + tenantID + rest + query, nil
	case proVersionPathRE.MatchString(path):
		m := proVersionPathRE.FindStringSubmatch(path)
		// m[1]=/api m[2]=vN m[3]=/rest
		return m[1] + "/pro/" + m[2] + "/tenant/" + tenantID + m[3] + query, nil
	default:
		// Unknown absolute path: leave unchanged so callers can pass already-scoped
		// gateway paths or other product APIs.
		return path + query, nil
	}
}

// tenantIDForCred returns the Platform tenant ID for a credential, or "" for instance mode.
func tenantIDForCred(credName string) (string, error) {
	if activeConfig == nil {
		return "", fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	cred, err := activeConfig.ResolveCredential(credName)
	if err != nil {
		return "", err
	}
	return cred.EffectiveTenantID(), nil
}

// RewritePathForCred applies RewriteJamfPath using the credential's tenantId.
func RewritePathForCred(credName, path string) (string, error) {
	tenantID, err := tenantIDForCred(credName)
	if err != nil {
		return "", err
	}
	return RewriteJamfPath(tenantID, path)
}

// ResolveEndpoint returns the full request URL (base + rewritten path) for a credential.
func ResolveEndpoint(credName, path string) (string, error) {
	baseURL, err := resolveURL(credName)
	if err != nil {
		return "", err
	}
	rewritten, err := RewritePathForCred(credName, path)
	if err != nil {
		return "", err
	}
	return baseURL + rewritten, nil
}

// IsPlatformCred reports whether the named credential uses Platform API Gateway mode.
func IsPlatformCred(credName string) (bool, error) {
	tenantID, err := tenantIDForCred(credName)
	if err != nil {
		return false, err
	}
	return tenantID != "", nil
}
