package buddy

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

const uriMacApps = "/JSSResource/macapplications"

// MacAppSummary is a short Classic macapplications list row.
type MacAppSummary struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	BundleID string `json:"bundle_id,omitempty"`
}

// MacAppDetail is the full Classic mac_application resource (JSON).
type MacAppDetail struct {
	General     json.RawMessage `json:"general"`
	Scope       json.RawMessage `json:"scope,omitempty"`
	SelfService json.RawMessage `json:"self_service,omitempty"`
	// Flat convenience fields populated from general when present.
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	BundleID string `json:"bundle_id,omitempty"`
	URL      string `json:"url,omitempty"`
	IsFree   bool   `json:"is_free,omitempty"`
}

// MacAppCreateSpec holds fields for creating a Mac App Store app record.
type MacAppCreateSpec struct {
	Name       string
	Version    string
	BundleID   string
	URL        string
	IsFree     bool
	Category   string // optional category name
	CategoryID int    // optional; wins over Category when > 0
}

type macAppsListXML struct {
	XMLName xml.Name `xml:"mac_applications"`
	Apps    []struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"mac_application"`
}

type macAppGeneralJSON struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	BundleID string `json:"bundle_id"`
	URL      string `json:"url"`
	IsFree   bool   `json:"is_free"`
}

type macAppJSONEnvelope struct {
	MacApplication *struct {
		General     json.RawMessage `json:"general"`
		Scope       json.RawMessage `json:"scope"`
		SelfService json.RawMessage `json:"self_service"`
	} `json:"mac_application"`
	// Some Classic JSON responses omit the wrapper.
	General     json.RawMessage `json:"general"`
	Scope       json.RawMessage `json:"scope"`
	SelfService json.RawMessage `json:"self_service"`
}

type macAppCreateXML struct {
	XMLName xml.Name `xml:"mac_application"`
	General struct {
		Name     string `xml:"name"`
		Version  string `xml:"version"`
		BundleID string `xml:"bundle_id"`
		URL      string `xml:"url"`
		IsFree   bool   `xml:"is_free"`
		Category *struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		} `xml:"category,omitempty"`
	} `xml:"general"`
}

type macAppPostResponseXML struct {
	XMLName xml.Name `xml:"mac_application"`
	General struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"general"`
	ID   int    `xml:"id"`
	Name string `xml:"name"`
}

// MacAppSearch lists Classic macapplications; filter is a client-side substring
// matched against name (and against version/bundle_id when those are loaded).
// List summaries only include id+name from Jamf; version/bundle_id are filled
// with a follow-up detail GET per match so the CLI table can show them.
func MacAppSearch(credName, filter string, logger *slog.Logger) ([]MacAppSummary, error) {
	body, err := jamfGETWithAccept(credName, uriMacApps, "application/xml")
	if err != nil {
		return nil, err
	}
	var list macAppsListXML
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse macapplications list: %w", err)
	}
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	out := make([]MacAppSummary, 0, len(list.Apps))
	for _, a := range list.Apps {
		sum := MacAppSummary{ID: a.ID, Name: a.Name}
		// Classic list is id+name only; enrich so CLI/table can show version/bundle_id.
		if detail, dErr := MacAppFetch(credName, strconv.Itoa(a.ID)); dErr == nil {
			sum.Version = detail.Version
			sum.BundleID = detail.BundleID
			if detail.Name != "" {
				sum.Name = detail.Name
			}
		}
		if !macAppSummaryMatches(sum, filterLower) {
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

// macAppSummaryMatches reports whether filterLower (already lowercased) hits
// name, version, or bundle_id. Empty filter matches everything.
func macAppSummaryMatches(s MacAppSummary, filterLower string) bool {
	if filterLower == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s.Name), filterLower) ||
		strings.Contains(strings.ToLower(s.Version), filterLower) ||
		strings.Contains(strings.ToLower(s.BundleID), filterLower)
}

// MacAppFetch loads one mac application by id (Classic JSON).
func MacAppFetch(credName, id string) (*MacAppDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("macapp id is required")
	}
	body, err := jamfGETWithAccept(credName, fmt.Sprintf("%s/id/%s", uriMacApps, url.PathEscape(id)), "application/json")
	if err != nil {
		return nil, err
	}
	return parseMacAppDetail(body)
}

// MacAppFetchByName loads one mac application by exact Classic name.
func MacAppFetchByName(credName, name string) (*MacAppDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("macapp name is required")
	}
	body, err := jamfGETWithAccept(credName, fmt.Sprintf("%s/name/%s", uriMacApps, url.PathEscape(name)), "application/json")
	if err != nil {
		return nil, err
	}
	return parseMacAppDetail(body)
}

func parseMacAppDetail(body []byte) (*MacAppDetail, error) {
	var env macAppJSONEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse macapplication JSON: %w", err)
	}
	d := &MacAppDetail{}
	if env.MacApplication != nil {
		d.General = env.MacApplication.General
		d.Scope = env.MacApplication.Scope
		d.SelfService = env.MacApplication.SelfService
	} else {
		d.General = env.General
		d.Scope = env.Scope
		d.SelfService = env.SelfService
	}
	if len(d.General) > 0 {
		var g macAppGeneralJSON
		if err := json.Unmarshal(d.General, &g); err == nil {
			d.ID = g.ID
			d.Name = g.Name
			d.Version = g.Version
			d.BundleID = g.BundleID
			d.URL = g.URL
			d.IsFree = g.IsFree
		}
	}
	return d, nil
}

// MacAppCreate POSTs a new Classic mac application.
func MacAppCreate(credName string, opts Opts, spec MacAppCreateSpec, logger *slog.Logger) (*MacAppSummary, error) {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Version = strings.TrimSpace(spec.Version)
	spec.BundleID = strings.TrimSpace(spec.BundleID)
	spec.URL = strings.TrimSpace(spec.URL)
	if spec.Name == "" || spec.Version == "" || spec.BundleID == "" || spec.URL == "" {
		return nil, fmt.Errorf("name, version, bundle_id, and url are required")
	}
	payload := macAppCreateXML{}
	payload.General.Name = spec.Name
	payload.General.Version = spec.Version
	payload.General.BundleID = spec.BundleID
	payload.General.URL = spec.URL
	payload.General.IsFree = spec.IsFree

	catID := spec.CategoryID
	catName := strings.TrimSpace(spec.Category)
	if catID <= 0 && catName != "" {
		idStr, err := CategoryFindIDByExactName(credName, catName)
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("category id %q: %w", idStr, err)
		}
		catID = n
	}
	if catID > 0 {
		name := catName
		if name == "" {
			if c, err := CategoryFetch(credName, strconv.Itoa(catID)); err == nil {
				name = c.Name
			}
		}
		payload.General.Category = &struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		}{ID: catID, Name: name}
	}

	body, err := xml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal macapplication: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate Mac App Store app %q (%s)?", spec.Name, spec.BundleID), logger) {
			return nil, nil
		}
	}
	resp, err := jamfPOST(credName, uriMacApps+"/id/0", body, "application/xml")
	if err != nil {
		return nil, fmt.Errorf("create macapplication: %w", err)
	}
	sum := &MacAppSummary{Name: spec.Name, Version: spec.Version, BundleID: spec.BundleID}
	var created macAppPostResponseXML
	if err := xml.Unmarshal(resp, &created); err == nil {
		if created.General.ID > 0 {
			sum.ID = created.General.ID
			if created.General.Name != "" {
				sum.Name = created.General.Name
			}
		} else if created.ID > 0 {
			sum.ID = created.ID
			if created.Name != "" {
				sum.Name = created.Name
			}
		}
	}
	if sum.ID <= 0 {
		// Resolve by name if POST body did not include an id.
		if d, err := MacAppFetchByName(credName, spec.Name); err == nil && d.ID > 0 {
			sum.ID = d.ID
			sum.Name = d.Name
			sum.Version = d.Version
			sum.BundleID = d.BundleID
		}
	}
	if sum.ID > 0 {
		fmt.Printf("Created Mac App Store app %q (ID %d)\n", sum.Name, sum.ID)
	} else {
		fmt.Printf("Created Mac App Store app %q\n", sum.Name)
	}
	return sum, nil
}

// MacAppDelete deletes one or more Classic mac applications by id.
func MacAppDelete(credName string, opts Opts, ids []int, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one macapp-id is required")
	}
	type info struct {
		id   int
		name string
	}
	infos := make([]info, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid macapp-id %d", id)
		}
		name := strconv.Itoa(id)
		if d, err := MacAppFetch(credName, strconv.Itoa(id)); err == nil && d.Name != "" {
			name = d.Name
		}
		infos = append(infos, info{id: id, name: name})
	}
	if !opts.SkipConfirm {
		msg := "\nDelete Mac App Store apps:"
		for _, p := range infos {
			msg += fmt.Sprintf("\n  %q (ID %d)", p.name, p.id)
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	for _, p := range infos {
		if err := jamfDELETE(credName, fmt.Sprintf("%s/id/%d", uriMacApps, p.id)); err != nil {
			return fmt.Errorf("delete macapplication %d: %w", p.id, err)
		}
		fmt.Printf("Deleted Mac App Store app %q (ID %d)\n", p.name, p.id)
	}
	return nil
}
