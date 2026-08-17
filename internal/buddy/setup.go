package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"buddy/internal/buddy/secrets"
)

// DefaultBuddyPrivileges is a curated list of Jamf Pro API role privilege strings
// that cover the endpoints buddy uses (policies, packages, scripts, profiles,
// groups, computers, MDM, categories, API roles, API integrations). Privilege
// names can vary by Jamf Pro version; run "buddy role privileges" to see your
// instance's list and adjust the generated script if needed.
var DefaultBuddyPrivileges = []string{
	"Create and edit policies in Jamf Pro API",
	"Read policies in Jamf Pro API",
	"Delete policies in Jamf Pro API",
	"Create and edit packages in Jamf Pro API",
	"Read packages in Jamf Pro API",
	"Delete packages in Jamf Pro API",
	"Create and edit scripts in Jamf Pro API",
	"Read scripts in Jamf Pro API",
	"Delete scripts in Jamf Pro API",
	"Create and edit configuration profiles in Jamf Pro API",
	"Read configuration profiles in Jamf Pro API",
	"Delete configuration profiles in Jamf Pro API",
	"Create and edit computer groups in Jamf Pro API",
	"Read computer groups in Jamf Pro API",
	"Delete computer groups in Jamf Pro API",
	"Read computer inventory in Jamf Pro API",
	"Send MDM commands in Jamf Pro API",
	"View MDM command information in Jamf Pro API",
	"Read categories in Jamf Pro API",
	"Create and edit API roles in Jamf Pro API",
	"Read API roles in Jamf Pro API",
	"Create and edit API integrations in Jamf Pro API",
	"Read API integrations in Jamf Pro API",
}

// WriteSetupScript writes a Starlark script to path that creates an API role
// with privileges needed for buddy and an API client (integration) scoped to
// that role. The script fetches the privilege list from your Jamf instance at
// runtime so it always uses the correct privilege names for your version.
//
// Run with an admin-capable auth already selected (tenant + auth in config):
//
//	buddy <path> -y
//	buddy setup store-oauth
func WriteSetupScript(path string) error {
	base := path
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		base = path[i+1:]
	}
	script := `#!/usr/bin/env buddy
# Setup script: create a Jamf Pro *instance* API role and client for buddy.
# (Platform gateway oauth cannot manage API roles/integrations.)
#
# Prerequisites: tenant with an instance URL + an instance-capable auth
# (typically basic admin). Select with --user / --tenant.
#
# Run with:
#   ./` + base + ` -y
#   buddy ` + base + ` -y
#
# Then rotate the instance client secret and store as buddy oauth (surface=instance):
#   buddy setup store-oauth
#   buddy --user <auth-name> auth test
#
# Idempotent: reuses existing role "Buddy" and client "buddy-cli" when present.

def main():
    role_name = "Buddy"
    client_name = "buddy-cli"

    rl = roles()
    role_id = None
    for row in rl:
        if row["display_name"] == role_name:
            role_id = row["id"]
            break
    if role_id == None:
        pl = role_privileges()
        role_id = role_create(role_name, pl)
        error("Created role %s (ID %s)" % (role_name, role_id))
    else:
        error("Using existing role %s (ID %s)" % (role_name, role_id))

    # Create API client scoped to that role (authorizationScopes expects role display name)
    cl = clients()
    client_id = None
    client_oauth_id = None
    for row in cl:
        name = row.get("displayName") or row.get("display_name") or ""
        if name == client_name:
            client_id = str(row["id"])
            client_oauth_id = row.get("clientId") or row.get("client_id") or ""
            break
    if client_id == None:
        c = client_create(client_name, [role_name])
        client_id = c["id"]
        client_oauth_id = c["client_id"]
        error("Created API client %s (ID %s, clientId %s)" % (client_name, client_id, client_oauth_id))
    else:
        error("Using existing API client %s (ID %s, clientId %s)" % (client_name, client_id, client_oauth_id))

    error("")
    error("Next: rotate the instance client secret and store it as buddy oauth (surface=instance):")
    error("  buddy setup store-oauth")
    error("  # matches an existing auth for this tenant+clientId, or prompts for a new name")
    error("  buddy --user <auth-name> auth test")
    error("")
    error("Or fetch the client secret once (shown only once) with:")
    error("  buddy client credentials %s" % client_id)
`
	return os.WriteFile(path, []byte(script), 0755)
}

// StoreOAuthResult is returned by SetupStoreOAuth.
type StoreOAuthResult struct {
	AuthName   string
	TenantName string
	ClientID   string
	Updated    bool // true if an existing auth was updated
}

// StoreOAuthOpts controls SetupStoreOAuth.
type StoreOAuthOpts struct {
	// APIAuth is the auth used to call Jamf (must be able to rotate client credentials).
	APIAuth string
	// ClientDisplayName is the Jamf API integration display name (default buddy-cli).
	ClientDisplayName string
	// AuthName is the buddy auth to create/update. Empty → auto: match existing, else prompt/default.
	AuthName string
	// TenantName links the auth to this tenant. Empty → tenant from the API session.
	TenantName string
	Opts       Opts
}

// SetupStoreOAuth finds a Jamf Pro (instance) API integration, rotates its OAuth
// client credentials (invalidating the previous secret), and stores clientId +
// secret as a buddy oauth auth with surface=instance linked to a tenant.
// Platform gateway oauth cannot create/manage API roles or integrations; this
// flow is instance-only. If an oauth auth already exists for that tenant with
// the same clientId, it is updated in place.
func SetupStoreOAuth(cfg *Config, so StoreOAuthOpts, logger *slog.Logger) (*StoreOAuthResult, error) {
	clientDisplayName := strings.TrimSpace(so.ClientDisplayName)
	if clientDisplayName == "" {
		clientDisplayName = "buddy-cli"
	}
	integration, err := ClientFindByDisplayName(so.APIAuth, clientDisplayName, logger)
	if err != nil {
		return nil, err
	}
	sess, err := ResolveSessionForCred(so.APIAuth, OpAny)
	if err != nil {
		return nil, err
	}
	tenantName := strings.TrimSpace(so.TenantName)
	if tenantName == "" {
		tenantName = sess.TenantName
	}
	if _, ok := cfg.Tenants[tenantName]; !ok {
		return nil, fmt.Errorf("tenant %q not found in config (use 'buddy cred set tenant' or --tenant=)", tenantName)
	}

	existingName, foundExisting := findOAuthAuthForClient(cfg, tenantName, integration.ClientID)
	authName := strings.TrimSpace(so.AuthName)
	updating := false
	switch {
	case authName != "":
		if existing, ok := cfg.Auths[authName]; ok {
			if existing.Type != "" && existing.Type != "oauth" {
				return nil, fmt.Errorf("auth %q exists with type %q (expected oauth)", authName, existing.Type)
			}
			if existing.ClientID != "" && existing.ClientID != integration.ClientID {
				return nil, fmt.Errorf("auth %q already has clientId %s (Jamf client %q is %s); pick another --name or omit --name to update the matching auth",
					authName, existing.ClientID, clientDisplayName, integration.ClientID)
			}
			updating = true
		} else if foundExisting && existingName != authName {
			// Creating a new auth name while another already points at this client — allowed but warn.
			fmt.Fprintf(os.Stderr, "note: auth %q already references this clientId on tenant %q; creating/updating %q as well\n",
				existingName, tenantName, authName)
		}
	case foundExisting:
		authName = existingName
		updating = true
		fmt.Printf("Matched existing auth %q for tenant %q clientId %s\n", authName, tenantName, integration.ClientID)
	default:
		defaultName := defaultOAuthAuthName(tenantName)
		if StdinIsTerminal() {
			line, err := PromptSecretLine(fmt.Sprintf("Auth name [%s]", defaultName))
			if err != nil {
				return nil, fmt.Errorf("read auth name: %w", err)
			}
			if line != "" {
				authName = line
			} else {
				authName = defaultName
			}
		} else {
			authName = defaultName
		}
		if existing, ok := cfg.Auths[authName]; ok {
			if existing.ClientID != "" && existing.ClientID != integration.ClientID {
				return nil, fmt.Errorf("auth %q already has a different clientId; choose another name", authName)
			}
			updating = true
		}
	}
	if authName == "" {
		return nil, fmt.Errorf("auth name is required (use --name=<name> or enter a name at the prompt)")
	}

	action := "create"
	if updating {
		action = "update"
	}
	fmt.Printf("Jamf client %q (id %d, clientId %s) on tenant %q → buddy auth %q (%s)\n",
		clientDisplayName, integration.ID, integration.ClientID, tenantName, authName, action)

	clientID, clientSecret, err := ClientCredentialsWithResult(so.APIAuth, fmt.Sprintf("%d", integration.ID), so.Opts, logger)
	if err != nil {
		return nil, err
	}
	if clientID == "" && clientSecret == "" {
		return nil, nil // user declined confirmation
	}

	if cfg.Auths == nil {
		cfg.Auths = make(map[string]AuthEntry)
	}
	prev := cfg.Auths[authName]
	// Jamf Pro API integrations are instance oauth (not Platform gateway).
	surface := SurfaceInstance
	access := AccessFull
	if prev.Access != "" {
		access = prev.Access
	}
	tenants := mergeTenantList(prev.Tenants, tenantName)
	cfg.Auths[authName] = AuthEntry{
		Type:     "oauth",
		ClientID: clientID,
		Tenants:  tenants,
		Access:   access,
		Surface:  surface,
		Channels: prev.Channels,
	}
	if err := SetCredentialSecret(authName, secrets.KindClientSecret, clientSecret); err != nil {
		return nil, fmt.Errorf("store client secret: %w", err)
	}
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return &StoreOAuthResult{
		AuthName:   authName,
		TenantName: tenantName,
		ClientID:   clientID,
		Updated:    updating,
	}, nil
}

func defaultOAuthAuthName(tenantName string) string {
	t := strings.TrimSpace(tenantName)
	if t != "" && t != DefaultTenant {
		return t + "-oauth"
	}
	return "buddy-oauth"
}

// findOAuthAuthForClient returns the auth name linked to tenantName whose clientId matches.
// If several match, the lexicographically first name wins (stable).
func findOAuthAuthForClient(cfg *Config, tenantName, clientID string) (string, bool) {
	if cfg == nil || clientID == "" {
		return "", false
	}
	var matches []string
	for name, a := range cfg.Auths {
		if a.Type != "oauth" || a.ClientID != clientID {
			continue
		}
		if authLinkedToTenant(a, tenantName) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}

func authLinkedToTenant(a AuthEntry, tenantName string) bool {
	for _, t := range a.Tenants {
		if t == tenantName {
			return true
		}
	}
	return false
}

func mergeTenantList(existing []string, tenantName string) []string {
	for _, t := range existing {
		if t == tenantName {
			out := make([]string, len(existing))
			copy(out, existing)
			return out
		}
	}
	out := make([]string, 0, len(existing)+1)
	out = append(out, existing...)
	out = append(out, tenantName)
	return out
}

// BasicSetupResult is returned by SetupBasic.
type BasicSetupResult struct {
	TenantName string
	AuthName   string
	URL        string
	Username   string
}

// BasicSetupOpts controls SetupBasic. Empty fields are prompted when stdin is a TTY.
type BasicSetupOpts struct {
	TenantName  string
	URL         string
	AuthName    string
	Username    string
	Password    string // optional; prompted when empty and TTY
	SkipConfirm bool
}

// SetupBasic creates the first instance tenant + basic auth (and stores the password).
// Intended for empty or incomplete config. Does not create Jamf roles/clients or oauth.
func SetupBasic(cfg *Config, opts BasicSetupOpts) (*BasicSetupResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	interactive := StdinIsTerminal()

	tenantName := strings.TrimSpace(opts.TenantName)
	if tenantName == "" {
		if !interactive {
			return nil, fmt.Errorf("--tenant is required when stdin is not a terminal")
		}
		def := DefaultTenant
		line, err := PromptSecretLine(fmt.Sprintf("Tenant name [%s]", def))
		if err != nil {
			return nil, err
		}
		if line != "" {
			tenantName = line
		} else {
			tenantName = def
		}
	}

	url := strings.TrimSpace(opts.URL)
	if url == "" {
		if !interactive {
			return nil, fmt.Errorf("--url is required when stdin is not a terminal")
		}
		line, err := PromptSecretLine("Jamf Pro instance URL (https://…)")
		if err != nil {
			return nil, err
		}
		url = strings.TrimSpace(line)
	}
	if url == "" {
		return nil, fmt.Errorf("instance URL is required")
	}

	authName := strings.TrimSpace(opts.AuthName)
	if authName == "" {
		def := tenantName + "-admin"
		if !interactive {
			authName = def
		} else {
			line, err := PromptSecretLine(fmt.Sprintf("Basic auth name [%s]", def))
			if err != nil {
				return nil, err
			}
			if line != "" {
				authName = line
			} else {
				authName = def
			}
		}
	}

	username := strings.TrimSpace(opts.Username)
	if username == "" {
		if !interactive {
			return nil, fmt.Errorf("--username is required when stdin is not a terminal")
		}
		line, err := PromptSecretLine("Jamf username")
		if err != nil {
			return nil, err
		}
		username = strings.TrimSpace(line)
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	password := opts.Password
	if password == "" {
		if !interactive {
			return nil, fmt.Errorf("--password is required when stdin is not a terminal (or use interactive mode)")
		}
		var err error
		password, err = PromptSecret(fmt.Sprintf("Password for %s", username))
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("password is required")
	}

	tenant := TenantConfig{URL: url}
	if err := tenant.Validate(); err != nil {
		return nil, fmt.Errorf("tenant: %w", err)
	}
	auth := AuthEntry{
		Type:     "basic",
		Username: username,
		Tenants:  []string{tenantName},
		Access:   AccessFull,
		Surface:  SurfaceInstance,
	}
	if err := auth.Validate(); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	_, tenantExists := cfg.Tenants[tenantName]
	_, authExists := cfg.Auths[authName]
	if (tenantExists || authExists) && !opts.SkipConfirm {
		if !interactive {
			return nil, fmt.Errorf("tenant/auth already exists; re-run with -y to overwrite")
		}
		msg := "Overwrite existing "
		parts := []string{}
		if tenantExists {
			parts = append(parts, fmt.Sprintf("tenant %q", tenantName))
		}
		if authExists {
			parts = append(parts, fmt.Sprintf("auth %q", authName))
		}
		msg += strings.Join(parts, " and ") + "?"
		if !ConfirmPrompt(msg, slog.Default()) {
			return nil, nil
		}
	}

	if cfg.Tenants == nil {
		cfg.Tenants = map[string]TenantConfig{}
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	}
	cfg.Tenants[tenantName] = tenant
	cfg.Auths[authName] = auth
	// Prefer this tenant as default when it is the only one, or when no default was set.
	if strings.TrimSpace(cfg.Tenant.Default) == "" || len(cfg.Tenants) == 1 {
		cfg.Tenant.Default = tenantName
	}

	if err := SetCredentialSecret(authName, secrets.KindPassword, password); err != nil {
		return nil, fmt.Errorf("store password: %w", err)
	}
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return &BasicSetupResult{
		TenantName: tenantName,
		AuthName:   authName,
		URL:        url,
		Username:   username,
	}, nil
}

// SetupSecretsResult is returned by SetupSecrets.
type SetupSecretsResult struct {
	CreatedAuths  []string
	StoredSecrets []string
	UpdatedAuths  []string // clientId / username filled in
	Missing       []string
}

// SetupSecretsOpts controls SetupSecrets. Empty secret/identity fields are
// prompted when stdin is a TTY; non-empty values are used as-is (tests).
type SetupSecretsOpts struct {
	SkipConfirm  bool
	Type         string // oauth (default) or basic, when creating an auth
	AuthName     string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// SetupSecrets creates missing auths for tenants that have none, then stores
// missing passwords / client secrets in the platform secret store.
func SetupSecrets(cfg *Config, opts SetupSecretsOpts) (*SetupSecretsResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	res := &SetupSecretsResult{}

	tenantNames := make([]string, 0, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tenantNames = append(tenantNames, n)
	}
	sort.Strings(tenantNames)

	changed := false
	for _, tn := range tenantNames {
		if len(cfg.AuthsForTenant(tn)) > 0 {
			continue
		}
		created, err := setupSecretsCreateAuth(cfg, tn, opts, interactive)
		if err != nil {
			return res, err
		}
		if created == "" {
			res.Missing = append(res.Missing, fmt.Sprintf("tenant %q has no auth — run 'buddy setup secrets' in a terminal or 'buddy cred set auth'", tn))
			continue
		}
		res.CreatedAuths = append(res.CreatedAuths, created)
		changed = true
	}

	authNames := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		authNames = append(authNames, n)
	}
	sort.Strings(authNames)

	for _, name := range authNames {
		updated, stored, miss, err := setupSecretsFillAuth(cfg, name, opts, interactive)
		if err != nil {
			return res, err
		}
		if updated {
			res.UpdatedAuths = append(res.UpdatedAuths, name)
			changed = true
		}
		if stored != "" {
			res.StoredSecrets = append(res.StoredSecrets, stored)
		}
		if miss != "" {
			res.Missing = append(res.Missing, miss)
		}
	}

	if changed {
		if err := SaveConfig(cfg); err != nil {
			return res, fmt.Errorf("save config: %w", err)
		}
	}
	if len(res.Missing) > 0 {
		return res, fmt.Errorf("%s", strings.Join(res.Missing, "\n"))
	}
	return res, nil
}

func setupSecretsCreateAuth(cfg *Config, tenantName string, opts SetupSecretsOpts, interactive bool) (string, error) {
	typ := strings.ToLower(strings.TrimSpace(opts.Type))
	if typ == "" {
		typ = "oauth"
	}
	if interactive && opts.Type == "" {
		line, err := PromptSecretLine(fmt.Sprintf("Tenant %q has no auth. Type [oauth]", tenantName))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) != "" {
			typ = strings.ToLower(strings.TrimSpace(line))
		}
	}
	switch typ {
	case "oauth", "basic":
	default:
		return "", fmt.Errorf("auth type must be oauth or basic (got %q)", typ)
	}

	defName := tenantName + "-admin"
	if typ == "oauth" {
		defName = defaultOAuthAuthName(tenantName)
	}
	authName := strings.TrimSpace(opts.AuthName)
	if authName == "" {
		if !interactive {
			if opts.ClientID == "" && opts.Username == "" {
				return "", nil
			}
			authName = defName
		} else {
			line, err := PromptSecretLine(fmt.Sprintf("Auth name [%s]", defName))
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(line) != "" {
				authName = strings.TrimSpace(line)
			} else {
				authName = defName
			}
		}
	}
	if authName == "" {
		return "", nil
	}

	auth := AuthEntry{
		Type:    typ,
		Tenants: []string{tenantName},
		Access:  AccessFull,
		Surface: SurfaceInstance,
	}
	switch typ {
	case "oauth":
		cid := strings.TrimSpace(opts.ClientID)
		if cid == "" {
			if !interactive {
				return "", nil
			}
			line, err := PromptSecretLine("OAuth client ID")
			if err != nil {
				return "", err
			}
			cid = strings.TrimSpace(line)
		}
		if cid == "" {
			return "", fmt.Errorf("client ID is required for oauth auth %q", authName)
		}
		auth.ClientID = cid
		secret := opts.ClientSecret
		if secret == "" {
			if !interactive {
				// create stub; secret pass will report missing
			} else {
				var err error
				secret, err = PromptSecret(fmt.Sprintf("Client secret for %s", authName))
				if err != nil {
					return "", err
				}
			}
		}
		if strings.TrimSpace(secret) != "" {
			if err := SetCredentialSecret(authName, secrets.KindClientSecret, secret); err != nil {
				return "", fmt.Errorf("store client secret: %w", err)
			}
		}
	case "basic":
		user := strings.TrimSpace(opts.Username)
		if user == "" {
			if !interactive {
				return "", nil
			}
			line, err := PromptSecretLine("Jamf username")
			if err != nil {
				return "", err
			}
			user = strings.TrimSpace(line)
		}
		if user == "" {
			return "", fmt.Errorf("username is required for basic auth %q", authName)
		}
		auth.Username = user
		password := opts.Password
		if password == "" && interactive {
			var err error
			password, err = PromptSecret(fmt.Sprintf("Password for %s", user))
			if err != nil {
				return "", err
			}
		}
		if strings.TrimSpace(password) != "" {
			if err := SetCredentialSecret(authName, secrets.KindPassword, password); err != nil {
				return "", fmt.Errorf("store password: %w", err)
			}
		}
	}
	if err := auth.Validate(); err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	}
	cfg.Auths[authName] = auth
	return authName, nil
}

func setupSecretsFillAuth(cfg *Config, authName string, opts SetupSecretsOpts, interactive bool) (updated bool, stored string, missing string, err error) {
	auth, ok := cfg.Auths[authName]
	if !ok {
		return false, "", "", nil
	}
	switch auth.Type {
	case "command":
		return false, "", "", nil
	case "oauth":
		if strings.TrimSpace(auth.ClientID) == "" {
			cid := strings.TrimSpace(opts.ClientID)
			if cid == "" && interactive {
				line, err := PromptSecretLine(fmt.Sprintf("OAuth client ID for auth %q", authName))
				if err != nil {
					return false, "", "", err
				}
				cid = strings.TrimSpace(line)
			}
			if cid == "" {
				return false, "", fmt.Sprintf("auth %q is oauth with no clientId — run 'buddy cred set auth %s --client-id=…'", authName, authName), nil
			}
			auth.ClientID = cid
			cfg.Auths[authName] = auth
			updated = true
		}
		st, err := CredentialSecretStatusFor(cfg, authName)
		if err != nil {
			return updated, "", "", err
		}
		if st.ClientSecretInConfig || st.ClientSecretInStore {
			return updated, "", "", nil
		}
		secret := opts.ClientSecret
		if secret == "" && interactive {
			secret, err = PromptSecret(fmt.Sprintf("Client secret for %s", authName))
			if err != nil {
				return updated, "", "", err
			}
		}
		if strings.TrimSpace(secret) == "" {
			return updated, "", fmt.Sprintf("auth %q is missing a client secret — run 'buddy cred set secret %s --client-secret'", authName, authName), nil
		}
		if err := SetCredentialSecret(authName, secrets.KindClientSecret, secret); err != nil {
			return updated, "", "", fmt.Errorf("store client secret: %w", err)
		}
		return updated, authName + " client-secret", "", nil
	case "basic":
		if strings.TrimSpace(auth.Username) == "" {
			user := strings.TrimSpace(opts.Username)
			if user == "" && interactive {
				line, err := PromptSecretLine(fmt.Sprintf("Jamf username for auth %q", authName))
				if err != nil {
					return false, "", "", err
				}
				user = strings.TrimSpace(line)
			}
			if user == "" {
				return false, "", fmt.Sprintf("auth %q is basic with no username — run 'buddy cred set auth %s --username=…'", authName, authName), nil
			}
			auth.Username = user
			cfg.Auths[authName] = auth
			updated = true
		}
		st, err := CredentialSecretStatusFor(cfg, authName)
		if err != nil {
			return updated, "", "", err
		}
		if st.PasswordInConfig || st.PasswordInStore {
			return updated, "", "", nil
		}
		password := opts.Password
		if password == "" && interactive {
			password, err = PromptSecret(fmt.Sprintf("Password for %s", auth.Username))
			if err != nil {
				return updated, "", "", err
			}
		}
		if strings.TrimSpace(password) == "" {
			return updated, "", fmt.Sprintf("auth %q is missing a password — run 'buddy cred set secret %s'", authName, authName), nil
		}
		if err := SetCredentialSecret(authName, secrets.KindPassword, password); err != nil {
			return updated, "", "", fmt.Errorf("store password: %w", err)
		}
		return updated, authName + " password", "", nil
	default:
		return false, "", "", nil
	}
}
