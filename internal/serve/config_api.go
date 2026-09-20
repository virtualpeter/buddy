package serve

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"buddy/internal/buddy"
)

func reloadActiveConfig() (*buddy.Config, error) {
	cfg, err := buddy.LoadConfig()
	if err != nil {
		return nil, err
	}
	buddy.SetConfigLoadError(nil)
	buddy.SetActiveConfig(cfg)
	return cfg, nil
}

func handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/config")
	path = strings.Trim(path, "/")
	switch {
	case path == "" && r.Method == http.MethodGet:
		handleConfigGet(w, r)
	case path == "rename" && r.Method == http.MethodPost:
		if configMutationBlocked(w) {
			return
		}
		handleConfigRename(w, r)
	case path == "restore" && r.Method == http.MethodPost:
		if configAgentLocked(w) {
			return
		}
		handleConfigRestore(w, r)
	case strings.HasPrefix(path, "tenant/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete),
		strings.HasPrefix(path, "context/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		if configMutationBlocked(w) {
			return
		}
		name := strings.TrimPrefix(path, "tenant/")
		name = strings.TrimPrefix(name, "context/")
		if r.Method == http.MethodPut {
			handleConfigPutTenant(w, r, name)
		} else {
			handleConfigDeleteTenant(w, r, name)
		}
	case strings.HasPrefix(path, "auth/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		if configMutationBlocked(w) {
			return
		}
		name := strings.TrimPrefix(path, "auth/")
		if r.Method == http.MethodPut {
			handleConfigPutAuth(w, r, name)
		} else {
			handleConfigDeleteAuth(w, r, name)
		}
	case path == "selection" && r.Method == http.MethodPut:
		if configMutationBlocked(w) {
			return
		}
		handleConfigPutSelection(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func configAPIBroken(w http.ResponseWriter) bool {
	if err := buddy.ConfigLoadError(); err == nil {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": buddy.ConfigLoadError().Error(),
	})
	return true
}

func configAgentLocked(w http.ResponseWriter) bool {
	if !buddy.ConfigEditsViaAgent(buddy.GetActiveConfig()) {
		return false
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "workplaces and auths are managed by Buddy Agent; open Buddy Agent → Settings",
	})
	return true
}

func configMutationBlocked(w http.ResponseWriter) bool {
	if configAPIBroken(w) {
		return true
	}
	return configAgentLocked(w)
}

func handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := reloadActiveConfig()
	if err != nil {
		switch {
		case buddy.IsAgentNeedsConfig(err):
			writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(&buddy.Config{}))
		case buddy.IsConfigParseError(err):
			buddy.SetConfigLoadError(err)
			fallback := buddy.GetActiveConfig()
			if fallback == nil {
				fallback = &buddy.Config{}
			}
			writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(fallback))
		default:
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

type tenantPutBody struct {
	URL           string `json:"url"`
	Region        string `json:"region"`
	Org           string `json:"org"`
	OrgID         string `json:"org_id"`
	EnvironmentID string `json:"environment_id"`
	TenantID      string `json:"tenant_id"`
}

func handleConfigPutTenant(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid context name"})
		return
	}
	var body tenantPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		cfg = &buddy.Config{}
		buddy.SetActiveConfig(cfg)
	}
	_, err := buddy.SetTenantMetadata(cfg, name, buddy.SetTenantOpts{
		URL:           body.URL,
		Region:        body.Region,
		Org:           body.Org,
		OrgID:         body.OrgID,
		SetOrg:        true,
		EnvironmentID: body.EnvironmentID,
		TenantID:      body.TenantID,
		Overwrite:     true,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err = reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

func handleConfigDeleteTenant(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no config"})
		return
	}
	if _, err := buddy.DeleteTenantMetadata(cfg, name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err := reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

type authPutBody struct {
	Type     string   `json:"type"`
	Tenants  []string `json:"tenants"`
	Contexts []string `json:"contexts"`
	Access   string   `json:"access"`
	Channels []string `json:"channels"`
	Username string   `json:"username"`
	ClientID string   `json:"client_id"`
	Scope    string   `json:"scope"`
}

func handleConfigPutAuth(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid auth name"})
		return
	}
	var body authPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no config — create a context first"})
		return
	}
	_, err := buddy.SetAuthMetadata(cfg, name, buddy.SetAuthMetadataOpts{
		Type:        body.Type,
		Tenants:     body.Tenants,
		Contexts:    body.Contexts,
		Access:      body.Access,
		Channels:    body.Channels,
		ChannelsSet: true,
		Username:    body.Username,
		ClientID:    body.ClientID,
		Scope:       body.Scope,
		Overwrite:   true,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, buddy.ErrExists) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err = reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

func handleConfigDeleteAuth(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no config"})
		return
	}
	if err := buddy.DeleteAuthMetadata(cfg, name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err := reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

type selectionPutBody struct {
	DefaultTenant string
	Channel       string
	ChannelTenant string
}

func (c *selectionPutBody) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	take := func(keys ...string) (string, error) {
		var s string
		for _, k := range keys {
			v, ok := raw[k]
			if !ok {
				continue
			}
			if err := json.Unmarshal(v, &s); err != nil {
				return "", err
			}
		}
		return s, nil
	}
	var err error
	if c.DefaultTenant, err = take("default_tenant", "default_context"); err != nil {
		return err
	}
	if c.Channel, err = take("channel"); err != nil {
		return err
	}
	if c.ChannelTenant, err = take("channel_tenant", "channel_context"); err != nil {
		return err
	}
	return nil
}

func handleConfigPutSelection(w http.ResponseWriter, r *http.Request) {
	var body selectionPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no config"})
		return
	}
	if err := buddy.SetSelection(cfg, buddy.SetSelectionOpts{
		DefaultTenant: body.DefaultTenant,
		Channel:       body.Channel,
		ChannelTenant: body.ChannelTenant,
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err := reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

type renameBody struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

func handleConfigRename(w http.ResponseWriter, r *http.Request) {
	var body renameBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	cfg := buddy.GetActiveConfig()
	if cfg == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no config"})
		return
	}
	from := strings.TrimSpace(body.From)
	to := strings.TrimSpace(body.To)
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	var err error
	switch kind {
	case "auth":
		err = buddy.RenameAuth(cfg, from, to)
	case "context", "tenant", "workplace", "":
		err = buddy.RenameWorkplace(cfg, from, to)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be context or auth"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg, err = reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

func handleConfigRestore(w http.ResponseWriter, r *http.Request) {
	if err := buddy.RestoreLastGoodConfig(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cfg, err := reloadActiveConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}
