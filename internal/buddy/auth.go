package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"buddy/internal/buddy/tokenauth"
)

// DirectAuthOpts holds one-off authentication settings (no config credential).
type DirectAuthOpts struct {
	BaseURL         string
	Region          string // Platform API Gateway region (builds BaseURL when set and BaseURL empty)
	TenantID        string // Platform API Gateway tenant id
	Username        string
	Password        string
	ClientID        string
	ClientSecret    string
	OAuth2CredsFile string
}

func (o DirectAuthOpts) UsesDirectAuth() bool {
	return o.BaseURL != "" ||
		o.Region != "" ||
		o.TenantID != "" ||
		o.Username != "" ||
		o.Password != "" ||
		o.ClientID != "" ||
		o.ClientSecret != "" ||
		o.OAuth2CredsFile != ""
}

// OAuth2CredsFile represents client credentials stored in a JSON file.
type OAuth2CredsFile struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// LoadOAuth2CredsFile reads client_id and client_secret from a JSON file.
func LoadOAuth2CredsFile(path string) (*OAuth2CredsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OAuth2 credentials file: %w", err)
	}
	var creds OAuth2CredsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse OAuth2 credentials file: %w", err)
	}
	if creds.ClientID == "" {
		return nil, fmt.Errorf("OAuth2 credentials file missing client_id")
	}
	if creds.ClientSecret == "" {
		return nil, fmt.Errorf("OAuth2 credentials file missing client_secret")
	}
	return &creds, nil
}

func (o *DirectAuthOpts) resolveOAuth2File() error {
	if o.OAuth2CredsFile == "" {
		return nil
	}
	creds, err := LoadOAuth2CredsFile(o.OAuth2CredsFile)
	if err != nil {
		return err
	}
	o.ClientID = creds.ClientID
	o.ClientSecret = creds.ClientSecret
	return nil
}

// ObtainRawToken returns a bearer token string using either direct auth flags or a config credential.
func ObtainRawToken(credName string, direct DirectAuthOpts, log *slog.Logger) (string, error) {
	if direct.UsesDirectAuth() {
		return obtainRawTokenDirect(direct, log)
	}
	return resolveRawToken(credName)
}

func obtainRawTokenDirect(direct DirectAuthOpts, log *slog.Logger) (string, error) {
	if err := direct.resolveOAuth2File(); err != nil {
		return "", err
	}
	if direct.BaseURL == "" && strings.TrimSpace(direct.Region) != "" {
		region, err := NormalizePlatformRegion(direct.Region)
		if err != nil {
			return "", err
		}
		direct.BaseURL = PlatformGatewayURL(region)
	}
	if direct.BaseURL == "" {
		return "", fmt.Errorf("Jamf Pro URL is required for direct auth (use --url or --region)")
	}
	platform := strings.TrimSpace(direct.TenantID) != ""
	if platform && (direct.ClientID == "" || direct.ClientSecret == "") {
		return "", fmt.Errorf("Platform API Gateway direct auth requires --client-id and --client-secret (and --tenant-id)")
	}
	cfg := tokenauth.Config{
		BaseURL:      direct.BaseURL,
		Username:     direct.Username,
		Password:     direct.Password,
		ClientId:     direct.ClientID,
		ClientSecret: direct.ClientSecret,
		Platform:     platform,
		Logger:       log,
	}
	if err := validateDirectAuth(cfg); err != nil {
		return "", err
	}
	cfg.CacheKey = directAuthCacheKey(cfg, direct.TenantID)
	a := tokenauth.NewAuthenticator(cfg)
	return a.GetToken()
}

func directAuthCacheKey(cfg tokenauth.Config, tenantID string) string {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Platform {
		return "direct-platform-" + cfg.ClientId + "@" + base + "/" + strings.TrimSpace(tenantID)
	}
	if cfg.ClientId != "" {
		return "direct-oauth-" + cfg.ClientId + "@" + base
	}
	return "direct-basic-" + cfg.Username + "@" + base
}

func validateDirectAuth(cfg tokenauth.Config) error {
	hasBasic := cfg.Username != "" && cfg.Password != ""
	hasOAuth := cfg.ClientId != "" && cfg.ClientSecret != ""
	methods := 0
	if hasBasic {
		methods++
	}
	if hasOAuth {
		methods++
	}
	if methods == 0 {
		return fmt.Errorf("direct auth requires username/password or client-id/client-secret (or --oauth2-creds-file)")
	}
	if methods > 1 {
		return fmt.Errorf("cannot specify multiple direct authentication methods")
	}
	return nil
}

// RefreshRawToken extends a bearer token using the keep-alive endpoint.
func RefreshRawToken(baseURL, tokenValue string, log *slog.Logger) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("Jamf Pro URL is required (use --url or --user with a configured credential)")
	}
	if tokenValue == "" {
		return "", fmt.Errorf("token is required")
	}
	a := tokenauth.NewAuthenticator(tokenauth.Config{
		BaseURL: baseURL,
		Logger:  log,
	})
	return a.RefreshTokenValue(tokenValue)
}

// AuthHeader resolves and returns the Authorization header value for the named credential.
func AuthHeader(credName string) (string, error) {
	return resolveAuthHeader(credName)
}

// GetToken resolves and prints the Authorization header value for the named credential.
// Output is the full header value, e.g. "Bearer eyJ..." or "Basic dXNl...".
func GetToken(credName string) error {
	header, err := AuthHeader(credName)
	if err != nil {
		return err
	}
	fmt.Println(header)
	return nil
}

// PrintRawToken prints the bearer token for a credential or direct auth flags.
func PrintRawToken(credName string, direct DirectAuthOpts, log *slog.Logger) error {
	token, err := ObtainRawToken(credName, direct, log)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

// AuthInfo returns compact info about the current credential's Jamf session.
// With renew=true, discards any cached token and obtains a fresh one first.
// Does not include the raw bearer token.
func AuthInfo(credName string, renew bool, log *slog.Logger) (map[string]interface{}, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return nil, err
	}
	cred := sess.Credential
	cacheName := sessionCacheName(sess)
	if renew {
		switch cred.Auth.Type {
		case "basic", "oauth":
			a, err := cachedAuthenticator(cacheName, sess.AuthName, cred)
			if err != nil {
				return nil, err
			}
			if err := a.ForceRenew(); err != nil {
				return nil, fmt.Errorf("renew token: %w", err)
			}
		case "command":
			// command auth is not disk-cached; AuthTestValue re-runs the command
		case "":
			return nil, fmt.Errorf("auth %q has no type configured", sess.AuthName)
		default:
			return nil, fmt.Errorf("auth %q has unknown type %q", sess.AuthName, cred.Auth.Type)
		}
	}

	raw, err := AuthTestValue(credName, log)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if ai, ok := raw["authInfo"].(map[string]interface{}); ok {
		for k, v := range ai {
			out[k] = v
		}
	} else {
		for k, v := range CompactAuthInfo(raw) {
			out[k] = v
		}
	}
	out["cred"] = sess.AuthName
	out["auth"] = sess.AuthName
	out["tenant"] = sess.TenantName
	out["url"] = strings.TrimRight(cred.EffectiveBaseURL(), "/")
	out["authType"] = cred.Auth.Type
	out["renewed"] = renew
	if live, ok := raw["live"]; ok {
		out["live"] = live
	}
	if tok, ok := raw["tokenAcquired"]; ok {
		out["tokenAcquired"] = tok
	}
	if cred.Auth.Type == "basic" || cred.Auth.Type == "oauth" {
		if a, err := cachedAuthenticator(cacheName, sess.AuthName, cred); err == nil {
			if exp := a.TokenExpiry(); !exp.IsZero() {
				out["expiresAt"] = exp.UTC().Format(time.RFC3339)
			}
		}
	}
	return out, nil
}

// CompactAuthInfo reduces a GET /api/v1/auth payload to a small summary dict.
func CompactAuthInfo(info map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if at, ok := info["authenticationType"]; ok {
		out["authenticationType"] = at
	}
	acct, _ := info["account"].(map[string]interface{})
	if acct != nil {
		out["username"] = acct["username"]
		out["accessLevel"] = acct["accessLevel"]
		out["privilegeSet"] = acct["privilegeSet"]
		privs := privilegeNamesFromAuthPayload(info)
		if bySite, ok := acct["privilegesBySite"].(map[string]interface{}); ok {
			n := 0
			if len(privs) > 0 {
				n = len(privs)
			} else {
				for _, v := range bySite {
					if list, ok := v.([]interface{}); ok {
						n += len(list)
					}
				}
			}
			out["privilegeCount"] = n
		}
		if len(privs) > 0 {
			out["privileges"] = privs
		}
	}
	if sites, ok := info["sites"]; ok {
		out["sites"] = sites
	}
	return out
}

func applyCompactToCaps(rec *CapsRecord, compact map[string]interface{}) {
	if rec == nil || compact == nil {
		return
	}
	if s, ok := compact["username"].(string); ok {
		rec.Username = s
	}
	if s, ok := compact["accessLevel"].(string); ok {
		rec.AccessLevel = s
	}
	if s, ok := compact["privilegeSet"].(string); ok {
		rec.PrivilegeSet = s
	}
	if n, ok := compact["privilegeCount"].(int); ok {
		rec.PrivilegeCount = n
	}
	if privs, ok := compact["privileges"].([]string); ok {
		rec.Privileges = privs
	}
}

func persistCaps(sess *Session, rec CapsRecord) {
	if sess == nil || sess.Auth.Type == "command" {
		return
	}
	rec.Auth = sess.AuthName
	rec.Tenant = sess.TenantName
	rec.Surface = sess.Surface
	if sess.Surface == SurfacePlatform {
		rec.TenantID = sess.Credential.EffectiveTenantID()
	}
	if rec.ExpiresAt.IsZero() {
		if a, err := cachedAuthenticator(sessionCacheName(sess), sess.AuthName, sess.Credential); err == nil {
			if exp := a.TokenExpiry(); !exp.IsZero() {
				rec.ExpiresAt = exp.UTC()
			}
		}
	}
	_ = SaveCaps(rec)
}

func authSecretMissing(sess *Session) bool {
	if sess == nil {
		return false
	}
	switch sess.Auth.Type {
	case "command":
		return strings.TrimSpace(sess.Auth.RunCommand) == ""
	case "basic":
		if strings.TrimSpace(sess.Auth.Password) != "" {
			return false
		}
		ok, _ := HasCredentialSecret(sess.AuthName, SecretKindPassword)
		return !ok
	case "oauth":
		if strings.TrimSpace(sess.Auth.ClientSecret) != "" {
			return false
		}
		ok, _ := HasCredentialSecret(sess.AuthName, SecretKindClientSecret)
		return !ok
	default:
		return false
	}
}

func authTestEnvelope(sess *Session, live string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"ok":      live == LiveOK || live == LiveTokenOK,
		"live":    live,
		"auth":    sess.AuthName,
		"tenant":  sess.TenantName,
		"surface": sess.Surface,
		"mode":    sess.Surface,
		"url":     strings.TrimRight(sess.Credential.EffectiveBaseURL(), "/"),
	}
	if sess.Surface == SurfacePlatform {
		out["tenantId"] = sess.Credential.EffectiveTenantID()
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// AuthTestValue tests authentication and returns a JSON-friendly object.
// Always ForceRenews basic/oauth tokens so a disk-cached bearer cannot mask a
// bad secret. Instance probes GET /api/v1/auth; platform probes buildings.
// Token success with a 401/403 probe is live=token-ok (exit 0).
func AuthTestValue(credName string, log *slog.Logger) (map[string]interface{}, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return nil, err
	}
	return authTestSession(sess, log)
}

func authTestSession(sess *Session, log *slog.Logger) (map[string]interface{}, error) {
	if sess == nil {
		return nil, fmt.Errorf("internal error: session is nil")
	}
	if authSecretMissing(sess) {
		rec := CapsRecord{Live: LiveSecretMissing, TokenOk: false}
		persistCaps(sess, rec)
		kind := "secret"
		switch sess.Auth.Type {
		case "basic":
			kind = "password"
		case "oauth":
			kind = "client secret"
		}
		return nil, fmt.Errorf("auth test: %s missing for auth %q — run 'buddy cred set secret %s' (live=%s)", kind, sess.AuthName, sess.AuthName, LiveSecretMissing)
	}

	platform := sess.Surface == SurfacePlatform
	if sess.Auth.Type == "basic" || sess.Auth.Type == "oauth" {
		a, err := cachedAuthenticator(sessionCacheName(sess), sess.AuthName, sess.Credential)
		if err != nil {
			persistCaps(sess, CapsRecord{Live: LiveTokenFail, TokenOk: false, Error: err.Error()})
			return nil, fmt.Errorf("auth test: %w", err)
		}
		if err := a.ForceRenew(); err != nil {
			persistCaps(sess, CapsRecord{Live: LiveTokenFail, TokenOk: false, Error: err.Error()})
			return nil, fmt.Errorf("auth test: token failed: %w", err)
		}
	}

	probePath := "/api/v1/auth"
	probeName := "auth"
	op := OpRead
	if platform {
		probePath = "/api/v1/buildings"
		probeName = "buildings"
		op = OpPlatform
	}
	endpoint, err := ResolveEndpointOp(sess.AuthName, probePath, op)
	if err != nil {
		return nil, err
	}
	if platform {
		endpoint += "?page=0&page-size=1"
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequestOp(req, sess.AuthName, op)
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		extra := map[string]interface{}{
			"tokenAcquired": true,
			"probe":         probeName,
			"probeStatus":   resp.StatusCode,
			"status":        resp.StatusCode,
		}
		rec := CapsRecord{Live: LiveOK, TokenOk: true, ProbeOk: true, Probe: probeName, ProbeStatus: resp.StatusCode}
		if !platform {
			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err == nil {
				compact := CompactAuthInfo(result)
				extra["authInfo"] = compact
				applyCompactToCaps(&rec, compact)
			} else {
				extra["raw"] = string(body)
			}
		}
		persistCaps(sess, rec)
		return authTestEnvelope(sess, LiveOK, extra), nil
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		if log != nil {
			log.Debug("auth probe not permitted; token acquired", "status", resp.StatusCode, "probe", probeName)
		}
		note := "Bearer token acquired; probe not permitted (privilege, not a bad secret)"
		if platform {
			note = "Bearer token acquired; buildings probe not permitted (need read:pro:buildings for a full probe)"
		}
		persistCaps(sess, CapsRecord{
			Live: LiveTokenOK, TokenOk: true, ProbeOk: false,
			Probe: probeName, ProbeStatus: resp.StatusCode,
		})
		return authTestEnvelope(sess, LiveTokenOK, map[string]interface{}{
			"tokenAcquired": true,
			"probe":         probeName,
			"probeStatus":   resp.StatusCode,
			"note":          note,
		}), nil
	}

	if platform {
		tenantID := sess.Credential.EffectiveTenantID()
		looksUUID := len(tenantID) == 36 && strings.Count(tenantID, "-") == 4
		if strings.Contains(string(body), "REQUEST_CONTEXT_NOT_PROVIDED") && !looksUUID {
			return nil, fmt.Errorf("platform auth test returned %d: %s\nHint: tenantId %q does not look like a Jamf Account tenant UUID. In Jamf Account → Integrations, open the integration and click the tenant pill to copy the tenant id (not the jamfcloud.com subdomain)", resp.StatusCode, string(body), tenantID)
		}
	}
	return nil, fmt.Errorf("auth test returned %d: %s", resp.StatusCode, string(body))
}

// AuthTest tests authentication and pretty-prints the response.
func AuthTest(credName string, log *slog.Logger) error {
	result, err := AuthTestValue(credName, log)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}

// AuthTestAllResult is one cell from buddy auth test --all.
type AuthTestAllResult struct {
	Tenant  string
	Surface string
	Auth    string
	Live    string
	OK      bool
	Err     string
	Result  map[string]interface{}
}

// AuthTestAll ForceRenews and probes every covering auth × tenant × surface.
// Returns a non-nil error if any cell is secret-missing or token-fail.
func AuthTestAll(cfg *Config, log *slog.Logger) ([]AuthTestAllResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	cells := CoverageTestCells(cfg)
	if len(cells) == 0 {
		return nil, nil
	}
	var out []AuthTestAllResult
	var failN int
	for _, cell := range cells {
		row := AuthTestAllResult{Tenant: cell.Tenant, Surface: cell.Surface, Auth: cell.Auth}
		err := WithSessionOverrides(SessionOverrides{Tenant: cell.Tenant, Auth: cell.Auth, Surface: cell.Surface}, func() error {
			res, err := AuthTestValue(cell.Auth, log)
			if err != nil {
				row.Err = err.Error()
				row.OK = false
				switch {
				case strings.Contains(err.Error(), "live="+LiveSecretMissing), strings.Contains(err.Error(), "missing for auth"):
					row.Live = LiveSecretMissing
				default:
					row.Live = LiveTokenFail
				}
				return nil
			}
			row.Result = res
			if s, ok := res["live"].(string); ok {
				row.Live = s
			}
			if ok, _ := res["ok"].(bool); ok {
				row.OK = ok
			}
			return nil
		})
		if err != nil {
			row.Err = err.Error()
			row.Live = LiveTokenFail
		}
		if row.Live == LiveTokenFail || row.Live == LiveSecretMissing {
			failN++
		}
		out = append(out, row)
	}
	if failN > 0 {
		return out, fmt.Errorf("auth test --all: %d of %d cells failed (secret missing or token failed)", failN, len(out))
	}
	return out, nil
}

// PrintAuthTestAll writes one JSON object per cell plus a one-line summary.
func PrintAuthTestAll(results []AuthTestAllResult) {
	var okN, tokenOkN, failN, missN int
	for _, r := range results {
		fmt.Printf("# %s / %s / %s\n", r.Tenant, r.Surface, r.Auth)
		if r.Result != nil {
			out, _ := json.MarshalIndent(r.Result, "", "  ")
			fmt.Println(string(out))
		} else if r.Err != "" {
			fmt.Printf("{\n  \"ok\": false,\n  \"live\": %q,\n  \"auth\": %q,\n  \"tenant\": %q,\n  \"surface\": %q,\n  \"error\": %q\n}\n",
				r.Live, r.Auth, r.Tenant, r.Surface, r.Err)
		}
		switch r.Live {
		case LiveOK:
			okN++
		case LiveTokenOK:
			tokenOkN++
		case LiveSecretMissing:
			missN++
		default:
			failN++
		}
	}
	fmt.Printf("%d ok, %d token-ok, %d token-fail, %d secret-missing (%d cells)\n",
		okN, tokenOkN, failN, missN, len(results))
}

// PrintAuthCaps dumps cached last-check records as JSON.
func PrintAuthCaps(cfg *Config, credName string, all bool) error {
	var recs []CapsRecord
	if all {
		recs = ListCapsRecords()
	} else {
		sess, err := ResolveSessionForCred(credName, OpAny)
		if err != nil {
			return err
		}
		tid := ""
		if sess.Surface == SurfacePlatform {
			tid = sess.Credential.EffectiveTenantID()
		}
		if rec := LoadCaps(sess.AuthName, sess.TenantName, sess.Surface, tid); rec != nil {
			recs = []CapsRecord{*rec}
		}
	}
	if len(recs) == 0 {
		fmt.Println("[]")
		return nil
	}
	out, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
