package buddy

import (
	"fmt"
	"log/slog"
	"os"
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
// Run with: buddy <path> -y
// Then run "buddy client credentials <id>" to get the client secret (shown once).
func WriteSetupScript(path string) error {
	script := `# Setup script: create a Jamf API role with buddy permissions and a client scoped to it.
# Run with: buddy ` + path + ` -y
# Then: buddy client credentials <client_id>  # to get the client secret (shown once).
# Idempotent: reuses existing role "Buddy" if present.

def main():
    role_name = "Buddy"
    rl = role_list(cred)
    if not rl["ok"]:
        error("role_list failed:", rl["error"])
        exit(1)
    roles = rl["value"]
    role_id = None
    for i in range(len(roles)):
        if roles[i]["display_name"] == role_name:
            role_id = roles[i]["id"]
            break
    if role_id == None:
        pl = role_privileges_list(cred)
        if not pl["ok"]:
            error("role_privileges_list failed:", pl["error"])
            exit(1)
        r = role_create(cred, role_name, pl["value"])
        if not r["ok"]:
            error("role_create failed:", r["error"])
            exit(1)
        role_id = r["value"]
        error("Created role (ID %s)" % role_id)
    else:
        error("Using existing role (ID %s)" % role_id)

    # Create API client scoped to that role (authorizationScopes expects role display name, not ID)
    c = client_create(cred, "buddy-cli", [role_name])
    if not c["ok"]:
        error("client_create failed:", c["error"])
        exit(1)
    client_id = c["value"]["id"]
    client_oauth_id = c["value"]["client_id"]
    error("Created API client (ID %s, clientId %s)" % (client_id, client_oauth_id))
    error("")
    error("Next: run the following to get the client secret (shown once):")
    error("  buddy client credentials %s" % client_id)

main()
`
	return os.WriteFile(path, []byte(script), 0644)
}

// SetupConfigureOAuth finds the API integration by display name, generates new OAuth credentials,
// and adds (or updates) a credential in config. Returns the credential name used (may differ from
// newCredName if that name was in use for a different client). Caller must have set activeConfig
// (e.g. via SetActiveConfig) so GetBaseURL(credNameForAPI) works.
func SetupConfigureOAuth(cfg *Config, credNameForAPI, clientDisplayName, newCredName string, opts Opts, logger *slog.Logger) (finalCredName string, err error) {
	integration, err := ClientFindByDisplayName(credNameForAPI, clientDisplayName, logger)
	if err != nil {
		return "", err
	}
	clientID, clientSecret, err := ClientCredentialsWithResult(credNameForAPI, fmt.Sprintf("%d", integration.ID), opts, logger)
	if err != nil {
		return "", err
	}
	if clientID == "" && clientSecret == "" {
		return "", nil // user declined confirmation
	}
	baseURL, err := GetBaseURL(credNameForAPI)
	if err != nil {
		return "", err
	}
	finalCredName = newCredName
	if existing, ok := cfg.Credentials[newCredName]; ok {
		if existing.Auth.ClientID != clientID {
			finalCredName = uniqueCredentialName(cfg, newCredName)
		}
	}
	if cfg.Credentials == nil {
		cfg.Credentials = make(map[string]CredentialConfig)
	}
	cfg.Credentials[finalCredName] = CredentialConfig{
		URL:  baseURL,
		Auth: AuthConfig{Type: "oauth", ClientID: clientID, ClientSecret: clientSecret},
	}
	if err := SaveConfig(cfg); err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	return finalCredName, nil
}

func uniqueCredentialName(cfg *Config, base string) string {
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, exists := cfg.Credentials[candidate]; !exists {
			return candidate
		}
	}
}
