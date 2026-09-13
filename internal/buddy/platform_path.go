package buddy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	HeaderEnvironmentID = "X-Environment-Id"
	HeaderTenantID      = "X-Tenant-Id"
)

var (
	proVersionPathRE = regexp.MustCompile(`^(/api)/(v\d+)(/.*)$`)
	// platformProductPathRE matches Platform product APIs (logical /api/{service}/vN/…).
	// Service may include a slash (ddm/report).
	platformProductPathRE = regexp.MustCompile(`^(/api/(?:blueprints|devices|device-groups|device-actions|compliance-benchmarks|ddm/report|licensing|sso))/(v\d+)(/.*)$`)
	tenantSegmentRE       = regexp.MustCompile(`/tenant/[^/]+`)
)

// platformProductPrefixes are logical (and rewritten) path prefixes that use Platform gateway auth.
var platformProductPrefixes = []string{
	"/api/blueprints",
	"/blueprints",
	"/api/devices",
	"/devices",
	"/api/device-groups",
	"/device-groups",
	"/api/device-actions",
	"/device-actions",
	"/api/compliance-benchmarks",
	"/compliance-benchmarks",
	"/api/ddm/report",
	"/ddm/report",
	"/api/licensing",
	"/licensing",
	"/api/sso",
	"/sso",
}

// isPlatformProductPath reports whether path targets a vendored Platform product API
// (instance-relative /api/{service} or GA rewritten /{service}).
func isPlatformProductPath(path string) bool {
	for _, p := range platformProductPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// isACSConfigProfilePath reports Jamf Pro Application & Custom Settings UI API
// paths (/api/config-profiles/…). Those exist only on the instance host.
func isACSConfigProfilePath(path string) bool {
	return path == "/api/config-profiles" || strings.HasPrefix(path, "/api/config-profiles/")
}

// isUndocumentedProPoliciesListPath reports the unpublished Pro policy summary
// list (GET /api/v1/policies). Not in OpenAPI; the Platform gateway returns 403.
// Always instance — not gated by features.classic_instance_only.
func isUndocumentedProPoliciesListPath(path string) bool {
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	return path == "/api/v1/policies"
}

func stripTenantSegment(path string) string {
	return tenantSegmentRE.ReplaceAllString(path, "")
}

// RewriteJamfPath maps an instance-relative Jamf path onto the Platform API Gateway
// when platform is true. With platform false the path is returned unchanged
// (aside from normalization). GA scope travels in headers, not /tenant/{id}.
//
// Rules:
//   - /api/vN/rest…           → /pro/vN/rest…
//   - /JSSResource/…          → /proclassic/…  (JSSResource prefix stripped)
//   - /api/{product}/vN/…     → /{service}/vN/…
//     (blueprints, devices, device-groups, device-actions, compliance-benchmarks, ddm/report, licensing, sso)
//   - already rewritten /pro/… or /proclassic/… → strip leftover /tenant/{id} and /api
//   - /api/schema, /classicapi, /api/config-profiles, GET /api/v1/policies → error (not supported via gateway)
func RewriteJamfPath(platform bool, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	query := ""
	if i := strings.Index(path, "?"); i >= 0 {
		query = path[i:]
		path = path[:i]
	}
	if !platform {
		return path + query, nil
	}
	if path == "/api/schema" || strings.HasPrefix(path, "/api/schema/") ||
		path == "/classicapi" || strings.HasPrefix(path, "/classicapi/") {
		return "", fmt.Errorf("path %q is not supported via the Platform API Gateway", path)
	}
	if isACSConfigProfilePath(path) {
		return "", fmt.Errorf("path %q is not supported via the Platform API Gateway (ACS is instance-only)", path)
	}
	if isUndocumentedProPoliciesListPath(path) {
		return "", fmt.Errorf("path %q is not supported via the Platform API Gateway (undocumented Pro policy list is instance-only)", path)
	}

	path = stripTenantSegment(path)

	switch {
	case strings.HasPrefix(path, "/JSSResource"):
		rest := strings.TrimPrefix(path, "/JSSResource")
		if rest == "" {
			rest = "/"
		}
		return "/proclassic" + rest + query, nil
	case path == "/api/proclassic" || strings.HasPrefix(path, "/api/proclassic/"):
		return strings.TrimPrefix(path, "/api") + query, nil
	case strings.HasPrefix(path, "/api/pro/"):
		return strings.TrimPrefix(path, "/api") + query, nil
	case platformProductPathRE.MatchString(path):
		m := platformProductPathRE.FindStringSubmatch(path)
		// m[1]=/api/{service} m[2]=vN m[3]=/rest
		return strings.TrimPrefix(m[1], "/api") + "/" + m[2] + m[3] + query, nil
	case proVersionPathRE.MatchString(path):
		m := proVersionPathRE.FindStringSubmatch(path)
		// m[1]=/api m[2]=vN m[3]=/rest
		return "/pro/" + m[2] + m[3] + query, nil
	default:
		// Unknown absolute path: leave unchanged so callers can pass already-scoped
		// gateway paths or other product APIs.
		return path + query, nil
	}
}

func applyPlatformScopeHeaders(req *http.Request, cred CredentialConfig) {
	if req == nil {
		return
	}
	if eid := cred.EffectiveEnvironmentID(); eid != "" {
		req.Header.Set(HeaderEnvironmentID, eid)
		req.Header.Del(HeaderTenantID)
		return
	}
	if tid := cred.EffectiveTenantID(); tid != "" {
		req.Header.Set(HeaderTenantID, tid)
		req.Header.Del(HeaderEnvironmentID)
		return
	}
	req.Header.Del(HeaderEnvironmentID)
	req.Header.Del(HeaderTenantID)
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

func platformSessionForCredOp(credName string, op OpClass) (*Session, error) {
	sess, err := ResolveSessionForCred(credName, op)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// RewritePathForCred applies RewriteJamfPath using the session's platform surface.
func RewritePathForCred(credName, path string) (string, error) {
	return RewritePathForCredOp(credName, path, OpAny)
}

// RewritePathForCredOp is like RewritePathForCred with an operation class.
func RewritePathForCredOp(credName, path string, op OpClass) (string, error) {
	sess, err := platformSessionForCredOp(credName, op)
	if err != nil {
		return "", err
	}
	return RewriteJamfPath(sess.Surface == SurfacePlatform, path)
}

// ResolveEndpoint returns the full request URL (base + rewritten path) for a credential/session.
// Auth/surface are chosen from the path (same rules as JamfRequest).
func ResolveEndpoint(credName, path string) (string, error) {
	return ResolveEndpointOp(credName, path, opClassForPath("GET", path))
}

// ResolveEndpointOp resolves the endpoint using an operation class for auth picking.
func ResolveEndpointOp(credName, path string, op OpClass) (string, error) {
	// Apply the logical (unrewritten) path's resource class before picking
	// tenant/auth. fetchJamfPaged / JamfRequest resolve the URL first, then
	// PerformRequestOp sets the class from the possibly rewritten URL. Without
	// this, OpRead on a hybrid tenant can pick a platform URL (alphabetical /
	// caps tie) and later re-pick an instance auth that has the Pro privilege
	// — instance token against the gateway → 403.
	restore := setResourceClass(resourceClassForPath(path))
	defer restore()
	baseURL, err := resolveURLOp(credName, op)
	if err != nil {
		return "", err
	}
	if LooksLikeRetiredPlatformGateway(baseURL) {
		return "", retiredPlatformGatewayError(baseURL)
	}
	rewritten, err := RewritePathForCredOp(credName, path, op)
	if err != nil {
		return "", err
	}
	return baseURL + rewritten, nil
}

// IsPlatformCred reports whether the resolved session uses Platform API Gateway mode.
func IsPlatformCred(credName string) (bool, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return false, err
	}
	return sess.Surface == SurfacePlatform, nil
}
