package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"buddy/internal/buddy/tokenauth"
)

// DirectAuthOpts holds one-off authentication settings (no config credential).
type DirectAuthOpts struct {
	BaseURL         string
	Region          string // Platform API Gateway region (builds BaseURL when set and BaseURL empty)
	EnvironmentID   string // Platform environment id (preferred GA scope)
	TenantID        string // Platform tenant id (legacy single-product scope)
	Username        string
	Password        string
	ClientID        string
	ClientSecret    string
	OAuth2CredsFile string
}

func (o DirectAuthOpts) UsesDirectAuth() bool {
	return o.BaseURL != "" ||
		o.Region != "" ||
		o.EnvironmentID != "" ||
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
	if LooksLikeRetiredPlatformGateway(direct.BaseURL) {
		return "", retiredPlatformGatewayError(direct.BaseURL)
	}
	if eid, tid := strings.TrimSpace(direct.EnvironmentID), strings.TrimSpace(direct.TenantID); eid != "" && tid != "" {
		return "", bothPlatformScopeIDsError(eid, tid)
	}
	platform := strings.TrimSpace(direct.EnvironmentID) != "" || strings.TrimSpace(direct.TenantID) != "" || LooksLikePlatformGateway(direct.BaseURL) || strings.TrimSpace(direct.Region) != ""
	if platform && (direct.ClientID == "" || direct.ClientSecret == "") {
		return "", fmt.Errorf("Platform API Gateway direct auth requires --client-id and --client-secret")
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
	scope := strings.TrimSpace(direct.EnvironmentID)
	if scope == "" {
		scope = strings.TrimSpace(direct.TenantID)
	}
	if platform && scope == "" {
		scope = "org"
	}
	cfg.CacheKey = directAuthCacheKey(cfg, scope)
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

// AuthRefreshResult is the outcome of buddy auth token refresh.
type AuthRefreshResult struct {
	Auth   string
	Tenant string
	Type   string
	Action string
	Expiry time.Time
	Token  string
}

// AuthRefresh obtains, keep-alive refreshes, or replaces the session token
// based on auth type and disk-cache state.
func AuthRefresh(credName string, log *slog.Logger) (*AuthRefreshResult, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return nil, err
	}
	cred := sess.Credential
	out := &AuthRefreshResult{
		Auth:   sess.AuthName,
		Tenant: sess.TenantName,
		Type:   sess.Auth.Type,
	}
	switch cred.Auth.Type {
	case "command":
		header, err := resolveCommandToken(sess.AuthName, cred.Auth)
		if err != nil {
			return nil, err
		}
		out.Action = tokenauth.TokenActionReplace
		out.Token = strings.TrimPrefix(header, "Bearer ")
		return out, nil
	case "basic", "oauth":
		a, err := cachedAuthenticator(sessionCacheName(sess), sess.AuthName, cred)
		if err != nil {
			return nil, err
		}
		res, err := a.RefreshSession()
		if err != nil {
			return nil, err
		}
		out.Action = res.Action
		out.Expiry = res.Expiry
		out.Token = res.Token
		return out, nil
	case "":
		return nil, fmt.Errorf("auth %q has no type configured", sess.AuthName)
	default:
		return nil, fmt.Errorf("auth %q has unknown type %q", sess.AuthName, cred.Auth.Type)
	}
}

// PrintAuthRefresh writes a short status (no bearer) for auth token refresh.
func PrintAuthRefresh(res *AuthRefreshResult) {
	if res == nil {
		return
	}
	fmt.Printf("auth:     %s\n", res.Auth)
	fmt.Printf("tenant:   %s\n", res.Tenant)
	fmt.Printf("type:     %s\n", res.Type)
	fmt.Printf("action:   %s\n", res.Action)
	if !res.Expiry.IsZero() {
		fmt.Printf("expires:  %s\n", res.Expiry.UTC().Format(time.RFC3339))
	}
}

// TokenCacheStatus is local disk-cache metadata for one session (no bearer).
type TokenCacheStatus struct {
	Auth   string
	Tenant string
	Type   string
	Cache  string // missing | present | expired | n/a
	Expiry time.Time
	Usable bool
	Key    string
	URL    string
}

func tokenCacheState(authType string, info tokenauth.DiskTokenInfo) string {
	if authType == "command" {
		return "n/a"
	}
	if !info.Present {
		return "missing"
	}
	if !info.Usable {
		return "expired"
	}
	return "present"
}

// SessionTokenStatus returns disk-cache metadata for the resolved session.
func SessionTokenStatus(credName string) (*TokenCacheStatus, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return nil, err
	}
	st := &TokenCacheStatus{
		Auth:   sess.AuthName,
		Tenant: sess.TenantName,
		Type:   sess.Auth.Type,
		Key:    sessionDiskCacheKey(sess),
	}
	if sess.Auth.Type == "command" || sess.Credential.Auth.Type == "command" {
		st.Cache = "n/a"
		return st, nil
	}
	info := tokenauth.PeekDiskToken(st.Key)
	st.Cache = tokenCacheState(sess.Auth.Type, info)
	st.Expiry = info.Expiry
	st.Usable = info.Usable
	st.URL = info.URL
	return st, nil
}

// PrintTokenStatus prints cache metadata for the session, or every tokens/ file when all.
func PrintTokenStatus(credName string, all bool) error {
	if all {
		return printAllTokenStatus()
	}
	st, err := SessionTokenStatus(credName)
	if err != nil {
		return err
	}
	fmt.Printf("auth:     %s\n", st.Auth)
	fmt.Printf("tenant:   %s\n", st.Tenant)
	fmt.Printf("type:     %s\n", st.Type)
	fmt.Printf("cache:    %s\n", st.Cache)
	fmt.Printf("key:      %s\n", st.Key)
	if !st.Expiry.IsZero() {
		fmt.Printf("expires:  %s (%s)\n", st.Expiry.UTC().Format(time.RFC3339), formatTokenRemaining(st.Expiry))
	}
	if st.Cache != "n/a" && st.Cache != "missing" {
		fmt.Printf("usable:   %s\n", boolYesNo(st.Usable))
	}
	if st.URL != "" {
		fmt.Printf("url:      %s\n", st.URL)
	}
	return nil
}

func printAllTokenStatus() error {
	list, err := tokenauth.ListDiskTokens()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no token cache files")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tCACHE\tEXPIRES\tREMAINING\tUSABLE\tURL")
	for _, info := range list {
		cache := tokenCacheState("", info)
		exp := ""
		remain := ""
		if !info.Expiry.IsZero() {
			exp = info.Expiry.UTC().Format(time.RFC3339)
			remain = formatTokenRemaining(info.Expiry)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Key, cache, exp, remain, boolYesNo(info.Usable), info.URL)
	}
	return tw.Flush()
}

func formatTokenRemaining(exp time.Time) string {
	d := time.Until(exp)
	if d <= 0 {
		return "expired"
	}
	return d.Truncate(time.Second).String()
}

func boolYesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// DropTokenCache invalidates the session disk+process token cache, or every file when all.
func DropTokenCache(credName string, all bool) error {
	if all {
		evictAllAuthenticators()
		res, err := tokenauth.DropAllDiskTokens()
		if err != nil {
			return err
		}
		if res.Dir != "" {
			fmt.Printf("Token cache: %s\n", res.Dir)
		}
		fmt.Printf("dropped %d token cache file(s)\n", res.Dropped)
		return nil
	}
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err != nil {
		return err
	}
	if sess.Auth.Type == "command" || sess.Credential.Auth.Type == "command" {
		fmt.Printf("no disk cache for command auth %q\n", sess.AuthName)
		return nil
	}
	key := sessionDiskCacheKey(sess)
	evictAuthenticator(sessionCacheName(sess))
	if tokenauth.DropDiskToken(key) {
		fmt.Printf("dropped token cache for %s\n", key)
		return nil
	}
	fmt.Printf("no token cache for %s\n", key)
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
		rec.EnvironmentID = sess.Credential.EffectiveEnvironmentID()
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
	// The Buddy Agent holds this secret, so there is nothing local to check:
	// let the mint run and report the Agent's own answer (locked, or unknown
	// auth) instead of pointing at 'cred set secret'.
	if GetActiveConfig().AuthFromAgent(sess.AuthName) {
		return false
	}
	switch sess.Auth.Type {
	case AuthTypeCommand:
		return strings.TrimSpace(sess.Auth.RunCommand) == ""
	case AuthTypeBasic:
		if strings.TrimSpace(sess.Auth.Password) != "" {
			return false
		}
		ok, _ := HasCredentialSecret(sess.AuthName, SecretKindPassword)
		return !ok
	case AuthTypeOAuth, AuthTypePlatform:
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
		if eid := sess.Credential.EffectiveEnvironmentID(); eid != "" {
			out["environmentId"] = eid
		}
		if tid := sess.Credential.EffectiveTenantID(); tid != "" {
			out["tenantId"] = tid
		}
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// AuthTestValue tests authentication and returns a JSON-friendly object.
// Always ForceRenews basic/oauth tokens so a disk-cached bearer cannot mask a
// bad secret. Instance probes GET /api/v1/auth; platform probes buildings when
// the workplace has an environment or product-tenant id. Organization workplaces
// probe GET /licensing/v1/licenses (no X-Environment-Id / X-Tenant-Id). Token
// success with a 401/403 probe is live=token-ok (exit 0); licenses 401 after
// ForceRenew is a token failure.
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
		case AuthTypeBasic:
			kind = "password"
		case AuthTypeOAuth, AuthTypePlatform:
			kind = "client secret"
		}
		return nil, fmt.Errorf("auth test: %s missing for auth %q — run 'buddy cred set secret %s' (live=%s)", kind, sess.AuthName, sess.AuthName, LiveSecretMissing)
	}

	platform := sess.Surface == SurfacePlatform
	if sess.Auth.Type == AuthTypeBasic || sess.Auth.UsesClientCredentials() {
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
		if platformAuthTestHasProductScope(sess) {
			probePath = "/api/v1/buildings"
			probeName = "buildings"
			op = OpPlatform
		} else {
			probePath = uriLicenses
			probeName = "licenses"
			op = OpPlatform
		}
	}
	endpoint, err := ResolveEndpointOp(sess.AuthName, probePath, op)
	if err != nil {
		return nil, err
	}
	if platform && probeName == "buildings" {
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

	if resp.StatusCode == http.StatusForbidden || (resp.StatusCode == http.StatusUnauthorized && probeName != "licenses") {
		if log != nil {
			log.Debug("auth probe not permitted; token acquired", "status", resp.StatusCode, "probe", probeName)
		}
		note := "Bearer token acquired; probe not permitted (privilege, not a bad secret)"
		if platform && probeName == "licenses" {
			note = "Bearer token acquired; licenses probe not permitted (need licensing privilege on the org integration for a full probe)"
		} else if platform {
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

	if platform && strings.Contains(string(body), "REQUEST_CONTEXT_NOT_PROVIDED") && !platformAuthTestHasProductScope(sess) {
		note := "Bearer token acquired; probe needs X-Environment-Id or X-Tenant-Id (organization workplace sends none). Account licensing does not need a scope header; use --context on an environment or product tenant for a Pro probe."
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
		if hint := platformAuthTestHint(string(body), sess.Credential); hint != "" {
			return nil, fmt.Errorf("platform auth test returned %d: %s\nHint: %s", resp.StatusCode, string(body), hint)
		}
	}
	return nil, fmt.Errorf("auth test returned %d: %s", resp.StatusCode, string(body))
}

// platformAuthTestHasProductScope is true when a buildings (Pro) probe can send
// X-Environment-Id or X-Tenant-Id. Organization workplaces send neither.
func platformAuthTestHasProductScope(sess *Session) bool {
	if sess == nil {
		return false
	}
	return sess.Credential.EffectiveEnvironmentID() != "" || sess.Credential.EffectiveTenantID() != ""
}

func platformAuthTestHint(body string, cred CredentialConfig) string {
	eid := cred.EffectiveEnvironmentID()
	tid := cred.EffectiveTenantID()
	switch {
	case strings.Contains(body, "OWNERSHIP_FORBIDDEN"):
		if eid != "" {
			return fmt.Sprintf("X-Environment-Id %q is not accepted for these credentials (wrong organization, or a tenant id in the environment field)", eid)
		}
		if tid != "" {
			return fmt.Sprintf("X-Tenant-Id %q is not accepted for these credentials (wrong organization, or an environment id in the tenant field)", tid)
		}
		return "the gateway refused this scope (OWNERSHIP_FORBIDDEN); check environmentId / tenantId against the integration"
	case strings.Contains(body, "ENVIRONMENT_NOT_FOUND"):
		if eid != "" {
			return fmt.Sprintf("the gateway does not know environmentId %q (a tenant id in the environment field reads as this)", eid)
		}
		return "the gateway does not know this environment-id (a tenant id in the environment field reads as ENVIRONMENT_NOT_FOUND)"
	case strings.Contains(body, "REQUEST_CONTEXT_NOT_PROVIDED"):
		if eid == "" && tid == "" {
			return "no scope header was sent; organization workplaces probe licensing without X-Environment-Id (use --context on an environment or product tenant to probe Pro)"
		}
		id := eid
		if id == "" {
			id = tid
		}
		if id != "" && !LooksLikeTenantUUID(id) {
			return fmt.Sprintf("scope id %q does not look like a Jamf Account UUID. In Jamf Account → Integrations, open the integration and click the environment or tenant pill to copy the id", id)
		}
		return "the credential expects a scope header that was not sent; set environmentId or tenantId to match how the integration was registered"
	case strings.Contains(body, "INVALID_REQUEST_CONTEXT_TYPE"):
		if tid != "" && eid == "" {
			return "this endpoint does not accept tenant scope; set environmentId instead of tenantId"
		}
		if eid != "" {
			return "this endpoint does not accept environment scope with these credentials; try tenantId, or register an integration at the level the API declares"
		}
		return "the scope level sent is not the level that endpoint accepts"
	}
	return ""
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

// PrintAuthCaps dumps cached Jamf authorization details (GET /api/v1/auth).
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
			tid = sess.Credential.PlatformScopeKey()
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
