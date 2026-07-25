package buddy

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	proVersionPathRE = regexp.MustCompile(`^(/api)/(v\d+)(/.*)$`)
	// platformProductPathRE matches Platform product APIs that inject /tenant/{id}
	// after the version segment (blueprints, devices, device-groups, …).
	// Service may include a slash (ddm/report).
	platformProductPathRE = regexp.MustCompile(`^(/api/(?:blueprints|devices|device-groups|device-actions|compliance-benchmarks|ddm/report))/(v\d+)(/.*)$`)
)

// platformProductPrefixes are logical path prefixes that use Platform gateway auth.
var platformProductPrefixes = []string{
	"/api/blueprints",
	"/api/devices",
	"/api/device-groups",
	"/api/device-actions",
	"/api/compliance-benchmarks",
	"/api/ddm/report",
}

// isPlatformProductPath reports whether path targets a vendored Platform product API
// (instance-relative or already gateway-rewritten; both keep the /api/{service} prefix).
func isPlatformProductPath(path string) bool {
	for _, p := range platformProductPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// RewriteJamfPath maps an instance-relative Jamf path onto the Platform API Gateway
// when tenantID is non-empty. With an empty tenantID the path is returned unchanged
// (aside from normalization).
//
// Rules:
//   - /api/vN/rest…           → /api/pro/vN/tenant/{id}/rest…
//   - /JSSResource/…          → /api/proclassic/tenant/{id}/JSSResource/…
//   - /api/{product}/vN/…     → inject /tenant/{id} after the version segment
//     (blueprints, devices, device-groups, device-actions, compliance-benchmarks, ddm/report)
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
	case platformProductPathRE.MatchString(path):
		m := platformProductPathRE.FindStringSubmatch(path)
		// m[1]=/api/{service} m[2]=vN m[3]=/rest
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

// tenantIDForCred returns the Platform tenant ID for the resolved session, or "" for instance mode.
func tenantIDForCred(credName string) (string, error) {
	return tenantIDForCredOp(credName, OpAny)
}

func tenantIDForCredOp(credName string, op OpClass) (string, error) {
	sess, err := ResolveSessionForCred(credName, op)
	if err != nil {
		return "", err
	}
	if sess.Surface == SurfaceInstance {
		return "", nil
	}
	return sess.Credential.EffectiveTenantID(), nil
}

// RewritePathForCred applies RewriteJamfPath using the session's tenantId.
func RewritePathForCred(credName, path string) (string, error) {
	return RewritePathForCredOp(credName, path, OpAny)
}

// RewritePathForCredOp is like RewritePathForCred with an operation class.
func RewritePathForCredOp(credName, path string, op OpClass) (string, error) {
	tenantID, err := tenantIDForCredOp(credName, op)
	if err != nil {
		return "", err
	}
	return RewriteJamfPath(tenantID, path)
}

// ResolveEndpoint returns the full request URL (base + rewritten path) for a credential/session.
// Auth/surface are chosen from the path (same rules as JamfRequest).
func ResolveEndpoint(credName, path string) (string, error) {
	return ResolveEndpointOp(credName, path, opClassForPath("GET", path))
}

// ResolveEndpointOp resolves the endpoint using an operation class for auth picking.
func ResolveEndpointOp(credName, path string, op OpClass) (string, error) {
	baseURL, err := resolveURLOp(credName, op)
	if err != nil {
		return "", err
	}
	rewritten, err := RewritePathForCredOp(credName, path, op)
	if err != nil {
		return "", err
	}
	return baseURL + rewritten, nil
}

// IsPlatformCred reports whether the resolved session uses Platform API Gateway mode.
func IsPlatformCred(credName string) (bool, error) {
	tenantID, err := tenantIDForCred(credName)
	if err != nil {
		return false, err
	}
	return tenantID != "", nil
}
