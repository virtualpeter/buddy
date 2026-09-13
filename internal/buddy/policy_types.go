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

// Local policy types (replacing jamfpro SDK types for Classic API XML).

const uriPolicies = "/JSSResource/policies"

// responsePoliciesList is the XML response from GET /JSSResource/policies.
type responsePoliciesList struct {
	XMLName xml.Name `xml:"policies"`
	Size    int      `xml:"size"`
	Policy  []struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"policy"`
}

// resourcePolicy is the full policy XML from GET /JSSResource/policies/id/{id}.
type resourcePolicy struct {
	XMLName              xml.Name                         `xml:"policy"`
	General              policySubsetGeneral              `xml:"general"`
	Scope                policySubsetScope                `xml:"scope,omitempty"`
	SelfService          policySubsetSelfService          `xml:"self_service,omitempty"`
	PackageConfiguration policySubsetPackageConfiguration `xml:"package_configuration"`
	Scripts              []policySubsetScript             `xml:"scripts>script"`
	Maintenance          policySubsetMaintenance          `xml:"maintenance"`
	FilesProcesses       policySubsetFilesProcesses       `xml:"files_processes,omitempty"`
}

type policySubsetGeneral struct {
	ID                         int    `xml:"id"`
	Name                       string `xml:"name"`
	Enabled                    bool   `xml:"enabled"`
	Trigger                    string `xml:"trigger,omitempty"`
	TriggerCheckin             bool   `xml:"trigger_checkin"`
	TriggerEnrollmentComplete  bool   `xml:"trigger_enrollment_complete"`
	TriggerLogin               bool   `xml:"trigger_login"`
	TriggerLogout              bool   `xml:"trigger_logout"`
	TriggerNetworkStateChanged bool   `xml:"trigger_network_state_changed"`
	TriggerStartup             bool   `xml:"trigger_startup"`
	TriggerOther               string `xml:"trigger_other"`
	Frequency                  string `xml:"frequency,omitempty"`
	Category                   *struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"category,omitempty"`
	Site *struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"site,omitempty"`
}

type policySubsetScope struct {
	AllComputers   bool                          `xml:"all_computers"`
	AllJSSUsers    bool                          `xml:"all_jss_users"`
	Computers      *[]policySubsetComputer       `xml:"computers>computer"`
	ComputerGroups *[]policySubsetComputerGroup  `xml:"computer_groups>computer_group"`
	JSSUsers       *[]policySubsetJSSUser        `xml:"jss_users>user"`
	JSSUserGroups  *[]policySubsetJSSUserGroup   `xml:"jss_user_groups>user_group"`
	Buildings      *[]policySubsetBuilding       `xml:"buildings>building"`
	Departments    *[]policySubsetDepartment     `xml:"departments>department"`
	Limitations    *policySubsetScopeLimitations `xml:"limitations"`
	Exclusions     *policySubsetScopeExclusions  `xml:"exclusions"`
}

type policySubsetComputer struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
	UDID string `xml:"udid,omitempty"`
}

type policySubsetComputerGroup struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetJSSUser struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetJSSUserGroup struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetBuilding struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetDepartment struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetScopeLimitations struct {
	Users           *[]policySubsetUser           `xml:"users>user"`
	UserGroups      *[]policySubsetUserGroup      `xml:"user_groups>user_group"`
	NetworkSegments *[]policySubsetNetworkSegment `xml:"network_segments>network_segment"`
	IBeacons        *[]policySubsetIBeacon        `xml:"ibeacons>ibeacon"`
}

type policySubsetScopeExclusions struct {
	Computers       *[]policySubsetComputer       `xml:"computers>computer"`
	ComputerGroups  *[]policySubsetComputerGroup  `xml:"computer_groups>computer_group"`
	Users           *[]policySubsetUser           `xml:"users>user"`
	UserGroups      *[]policySubsetUserGroup      `xml:"user_groups>user_group"`
	Buildings       *[]policySubsetBuilding       `xml:"buildings>building"`
	Departments     *[]policySubsetDepartment     `xml:"departments>department"`
	NetworkSegments *[]policySubsetNetworkSegment `xml:"network_segments>network_segment"`
	JSSUsers        *[]policySubsetJSSUser        `xml:"jss_users>user"`
	JSSUserGroups   *[]policySubsetJSSUserGroup   `xml:"jss_user_groups>user_group"`
	IBeacons        *[]policySubsetIBeacon        `xml:"ibeacons>ibeacon"`
}

type policySubsetUser struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetUserGroup struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetNetworkSegment struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetIBeacon struct {
	ID   int    `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
}

type policySubsetSelfService struct {
	UseForSelfService           bool   `xml:"use_for_self_service"`
	SelfServiceDisplayName      string `xml:"self_service_display_name"`
	InstallButtonText           string `xml:"install_button_text"`
	ReinstallButtonText         string `xml:"reinstall_button_text"`
	SelfServiceDescription      string `xml:"self_service_description"`
	ForceUsersToViewDescription bool   `xml:"force_users_to_view_description"`
	FeatureOnMainPage           bool   `xml:"feature_on_main_page"`
	SelfServiceIcon             *struct {
		ID int `xml:"id"`
	} `xml:"self_service_icon,omitempty"`
}

type policySubsetPackageConfiguration struct {
	Packages          []policySubsetPackageConfigurationPackage `xml:"packages>package"`
	DistributionPoint string                                    `xml:"distribution_point"`
}

type policySubsetPackageConfigurationPackage struct {
	ID     int    `xml:"id"`
	Name   string `xml:"name,omitempty"`
	Action string `xml:"action"`
}

type policySubsetScript struct {
	ID          string `xml:"id"`
	Name        string `xml:"name,omitempty"`
	Priority    string `xml:"priority"`
	Parameter4  string `xml:"parameter4,omitempty"`
	Parameter5  string `xml:"parameter5,omitempty"`
	Parameter6  string `xml:"parameter6,omitempty"`
	Parameter7  string `xml:"parameter7,omitempty"`
	Parameter8  string `xml:"parameter8,omitempty"`
	Parameter9  string `xml:"parameter9,omitempty"`
	Parameter10 string `xml:"parameter10,omitempty"`
	Parameter11 string `xml:"parameter11,omitempty"`
}

type policySubsetMaintenance struct {
	Recon bool `xml:"recon"`
}

type policySubsetFilesProcesses struct {
	SearchByPath         string `xml:"search_by_path"`
	DeleteFile           bool   `xml:"delete_file"`
	LocateFile           string `xml:"locate_file"`
	UpdateLocateDatabase bool   `xml:"update_locate_database"`
	SpotlightSearch      string `xml:"spotlight_search"`
}

// policyGetByID fetches a single policy by ID and parses the XML.
func policyGetByID(credName string, policyID int) (*resourcePolicy, error) {
	path := fmt.Sprintf("%s/id/%d", uriPolicies, policyID)
	body, err := jamfGETWithAccept(credName, path, "application/xml")
	if err != nil {
		return nil, err
	}
	var p resourcePolicy
	if err := xml.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parse policy XML: %w", err)
	}
	return &p, nil
}

const uriPoliciesV1 = "/api/v1/policies"

// policyFetchList fetches the policy id/name list. Prefers undocumented Pro
// GET /api/v1/policies when an instance session can be resolved (paginated;
// instance-only — not on the Platform gateway and not gated by
// features.classic_instance_only). Environment-only workplaces skip that
// probe. Otherwise falls back to Classic GET /JSSResource/policies (which
// may still succeed on the gateway as /proclassic/policies).
func policyFetchList(credName string) ([]policyListItem, error) {
	return policyFetchListHybrid(credName, nil)
}

// policyListV1Session resolves an instance session for undocumented
// GET /api/v1/policies. Environment-only workplaces (no Jamf Pro URL) and
// platform-only auths fail here; Classic-on-gateway still works.
func policyListV1Session(credName string) error {
	_, err := ResolveSessionForCred(credName, OpProUI)
	return err
}

// policyFetchListHybrid tries Pro v1 then Classic. logger may be nil.
func policyFetchListHybrid(credName string, logger *slog.Logger) ([]policyListItem, error) {
	var v1Err error
	if err := policyListV1Session(credName); err != nil {
		if logger != nil {
			logger.Debug("policy list skipping v1; using classic", "error", err.Error())
		}
	} else {
		v1, err := policyFetchListV1(credName)
		if err == nil && len(v1) > 0 {
			if logger != nil {
				logger.Debug("policy list source", "source", "v1", "count", len(v1))
			}
			return v1, nil
		}
		v1Err = err
		if err != nil && logger != nil {
			logger.Debug("policy list v1 failed; falling back to classic", "error", err.Error())
		}
	}
	classic, cerr := policyFetchListClassic(credName)
	if cerr != nil {
		if v1Err != nil {
			return nil, fmt.Errorf("policy list v1: %w; classic: %v", v1Err, cerr)
		}
		return nil, cerr
	}
	if logger != nil {
		logger.Debug("policy list source", "source", "classic", "count", len(classic))
	}
	return classic, nil
}

func policyFetchListClassic(credName string) ([]policyListItem, error) {
	body, err := jamfGETWithAccept(credName, uriPolicies, "application/xml")
	if err != nil {
		return nil, err
	}
	var list responsePoliciesList
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse policy list XML: %w", err)
	}
	result := make([]policyListItem, 0, len(list.Policy))
	for _, p := range list.Policy {
		result = append(result, policyListItem{ID: p.ID, Name: p.Name})
	}
	return result, nil
}

func policyFetchListV1(credName string) ([]policyListItem, error) {
	var all []policyListItem
	err := fetchJamfPaged(credName, uriPoliciesV1, url.Values{}, PageOpts{Page: PageAll}, func(resultsJSON []byte, _ int) (int, error) {
		page, err := parseV1PolicyListResults(resultsJSON)
		if err != nil {
			return 0, err
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// parseV1PolicyListResults parses a JSON array of Pro v1 policy list rows.
// IDs may be JSON numbers or strings.
func parseV1PolicyListResults(resultsJSON []byte) ([]policyListItem, error) {
	var raw []map[string]interface{}
	if err := json.Unmarshal(resultsJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse v1 policy list: %w", err)
	}
	out := make([]policyListItem, 0, len(raw))
	for _, m := range raw {
		id, ok := policyListIDFromAny(m["id"])
		if !ok || id <= 0 {
			continue
		}
		name, _ := m["name"].(string)
		item := policyListItem{ID: id, Name: name}
		if cat, ok := m["categoryId"].(string); ok {
			item.CategoryID = cat
		} else if catNum, ok := m["categoryId"].(float64); ok {
			item.CategoryID = strconv.FormatInt(int64(catNum), 10)
		}
		if ef, ok := m["executionFrequency"].(string); ok {
			item.ExecutionFrequency = ef
		}
		if tm, ok := m["triggerManual"].(string); ok {
			s := tm
			item.TriggerManual = &s
		}
		if b, ok := m["enabled"].(bool); ok {
			item.Enabled = &b
		}
		if b, ok := m["triggerCheckin"].(bool); ok {
			item.TriggerCheckin = &b
		}
		if b, ok := m["updateInventory"].(bool); ok {
			item.UpdateInventory = &b
		}
		out = append(out, item)
	}
	return out, nil
}

func policyListIDFromAny(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case json.Number:
		n, err := t.Int64()
		return int(n), err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		n, err := strconv.Atoi(s)
		return n, err == nil
	default:
		return 0, false
	}
}

// categoryGetByName fetches categories and returns the ID for the given name.
func categoryGetByName(credName, name string) (string, error) {
	return CategoryFindIDByExactName(credName, name)
}
