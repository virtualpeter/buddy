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
	buddy.SetActiveConfig(cfg)
	return cfg, nil
}

func handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/config")
	path = strings.Trim(path, "/")
	switch {
	case path == "" && r.Method == http.MethodGet:
		handleConfigGet(w, r)
	case strings.HasPrefix(path, "tenant/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		name := strings.TrimPrefix(path, "tenant/")
		if r.Method == http.MethodPut {
			handleConfigPutTenant(w, r, name)
		} else {
			handleConfigDeleteTenant(w, r, name)
		}
	case strings.HasPrefix(path, "auth/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		name := strings.TrimPrefix(path, "auth/")
		if r.Method == http.MethodPut {
			handleConfigPutAuth(w, r, name)
		} else {
			handleConfigDeleteAuth(w, r, name)
		}
	case path == "selection" && r.Method == http.MethodPut:
		handleConfigPutSelection(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg := buddy.GetActiveConfig()
	writeJSON(w, http.StatusOK, buddy.BuildConfigSnapshot(cfg))
}

type tenantPutBody struct {
	URL           string `json:"url"`
	Region        string `json:"region"`
	EnvironmentID string `json:"environment_id"`
	TenantID      string `json:"tenant_id"`
}

func handleConfigPutTenant(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tenant name"})
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
	Access   string   `json:"access"`
	Channels []string `json:"channels"`
	Username string   `json:"username"`
	ClientID string   `json:"client_id"`
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no config — create a tenant first"})
		return
	}
	_, err := buddy.SetAuthMetadata(cfg, name, buddy.SetAuthMetadataOpts{
		Type:        body.Type,
		Tenants:     body.Tenants,
		Access:      body.Access,
		Channels:    body.Channels,
		ChannelsSet: true,
		Username:    body.Username,
		ClientID:    body.ClientID,
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
	DefaultTenant string `json:"default_tenant"`
	Channel       string `json:"channel"`
	ChannelTenant string `json:"channel_tenant"`
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
