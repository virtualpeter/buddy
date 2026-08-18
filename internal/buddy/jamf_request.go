package buddy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// JamfRequestOpts configures a single authenticated Jamf HTTP call.
type JamfRequestOpts struct {
	Method      string // GET, POST, PUT, PATCH, DELETE
	Path        string // absolute path on the Jamf host, e.g. /api/v1/packages or /JSSResource/policies
	Body        []byte
	ContentType string // request Content-Type (POST/PUT/PATCH)
	Accept      string // Accept header; empty uses path-based default
	// Unauthenticated sends no Authorization header. Only for endpoints the Jamf
	// host serves publicly (e.g. /api/schema, /classicapi/doc/swagger.yaml); the
	// credential is then used solely to resolve the base URL.
	Unauthenticated bool
	// Op selects which linked auth to use when the process default is a tenant
	// (empty → OpAny, or OpSchema when Unauthenticated).
	Op OpClass
}

// JamfResponse is the raw HTTP outcome of JamfRequest (including non-2xx).
type JamfResponse struct {
	Status      int
	Body        []byte
	ContentType string
}

// DefaultJamfMedia returns Content-Type / Accept defaults from the path:
// Classic /JSSResource (or gateway /api/proclassic/…) → XML; everything else → JSON.
func DefaultJamfMedia(path string) (contentType, accept string) {
	if strings.HasPrefix(path, "/JSSResource") ||
		strings.HasPrefix(path, "/api/proclassic/") ||
		strings.Contains(path, "/proclassic/") {
		return "application/xml", "application/xml"
	}
	return "application/json", "application/json"
}

// NormalizeJamfPath validates a Jamf API path (must be absolute, no scheme/host, no "..").
// opClassForPath infers a default OpClass from method + path.
// Recognizes both instance-relative paths and already-rewritten gateway paths.
func opClassForPath(method, path string) OpClass {
	if strings.HasPrefix(path, "/JSSResource") || strings.Contains(path, "/proclassic/") {
		if method == http.MethodGet || method == http.MethodHead {
			return OpClassic
		}
		return OpClassicWrite
	}
	// Jamf Pro ACS UI API — instance host only (not on the Platform gateway).
	if isACSConfigProfilePath(path) {
		if method == http.MethodGet || method == http.MethodHead {
			return OpACS
		}
		return OpACSWrite
	}
	// Vendored Platform product APIs (blueprints, devices, device-groups, …).
	if isPlatformProductPath(path) {
		if method == http.MethodGet || method == http.MethodHead {
			return OpPlatform
		}
		return OpPlatformWrite
	}
	// Already rewritten Pro gateway paths — keep platform auth/surface.
	if strings.HasPrefix(path, "/api/pro/") || strings.Contains(path, "/api/pro/") {
		if method == http.MethodGet || method == http.MethodHead {
			return OpPlatform
		}
		return OpPlatformWrite
	}
	if method == http.MethodGet || method == http.MethodHead {
		return OpRead
	}
	return OpMutate
}

func NormalizeJamfPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(path, "://") || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("path must be a host-relative path starting with / (got %q)", path)
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path must start with / (got %q)", path)
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path must not contain ..")
	}
	return path, nil
}

// JamfRequest performs an authenticated request against the credential's Jamf base URL.
// Non-2xx responses return a JamfResponse and a nil error so callers can inspect status/body.
// Transport and auth failures return an error.
func JamfRequest(credName string, opts JamfRequestOpts) (*JamfResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
	default:
		return nil, fmt.Errorf("unsupported method %q", opts.Method)
	}

	path, err := NormalizeJamfPath(opts.Path)
	if err != nil {
		return nil, err
	}

	defCT, defAccept := DefaultJamfMedia(path)
	accept := strings.TrimSpace(opts.Accept)
	if accept == "" {
		accept = defAccept
	}
	contentType := strings.TrimSpace(opts.ContentType)
	if contentType == "" && len(opts.Body) > 0 && method != http.MethodGet && method != http.MethodHead && method != http.MethodDelete {
		contentType = defCT
	}

	op := opts.Op
	if op == "" {
		if opts.Unauthenticated {
			op = OpSchema
		} else {
			op = opClassForPath(method, path)
		}
	}
	endpoint, err := ResolveEndpointOp(credName, path, op)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if len(opts.Body) > 0 {
		bodyReader = bytes.NewReader(opts.Body)
	}
	req, err := http.NewRequest(method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", accept)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	var resp *http.Response
	if opts.Unauthenticated {
		resp, err = PerformRequestNoAuth(req)
	} else {
		resp, err = PerformRequestOp(req, credName, op)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return &JamfResponse{
		Status:      resp.StatusCode,
		Body:        body,
		ContentType: ct,
	}, nil
}
