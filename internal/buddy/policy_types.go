package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
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
	XMLName xml.Name `xml:"policy"`
	General              policySubsetGeneral              `xml:"general"`
	Scope                policySubsetScope                `xml:"scope,omitempty"`
	SelfService          policySubsetSelfService          `xml:"self_service,omitempty"`
	PackageConfiguration policySubsetPackageConfiguration `xml:"package_configuration"`
	Scripts              []policySubsetScript `xml:"scripts>script"`
	Maintenance    policySubsetMaintenance    `xml:"maintenance"`
	FilesProcesses policySubsetFilesProcesses `xml:"files_processes,omitempty"`
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
	AllComputers   bool                      `xml:"all_computers"`
	AllJSSUsers    bool                      `xml:"all_jss_users"`
	Computers      *[]policySubsetComputer  `xml:"computers>computer"`
	ComputerGroups *[]policySubsetComputerGroup `xml:"computer_groups>computer_group"`
	JSSUsers       *[]policySubsetJSSUser   `xml:"jss_users>user"`
	JSSUserGroups  *[]policySubsetJSSUserGroup `xml:"jss_user_groups>user_group"`
	Buildings      *[]policySubsetBuilding  `xml:"buildings>building"`
	Departments    *[]policySubsetDepartment `xml:"departments>department"`
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
	Computers      *[]policySubsetComputer      `xml:"computers>computer"`
	ComputerGroups *[]policySubsetComputerGroup `xml:"computer_groups>computer_group"`
	Users          *[]policySubsetUser          `xml:"users>user"`
	UserGroups     *[]policySubsetUserGroup     `xml:"user_groups>user_group"`
	Buildings      *[]policySubsetBuilding      `xml:"buildings>building"`
	Departments    *[]policySubsetDepartment    `xml:"departments>department"`
	NetworkSegments *[]policySubsetNetworkSegment `xml:"network_segments>network_segment"`
	JSSUsers       *[]policySubsetJSSUser       `xml:"jss_users>user"`
	JSSUserGroups  *[]policySubsetJSSUserGroup  `xml:"jss_user_groups>user_group"`
	IBeacons       *[]policySubsetIBeacon       `xml:"ibeacons>ibeacon"`
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

// policyFetchList fetches the policy list and returns id/name pairs.
func policyFetchList(credName string) ([]policyListItem, error) {
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

// categoryGetByName fetches categories and returns the ID for the given name.
func categoryGetByName(credName, name string) (string, error) {
	return CategoryFindIDByExactName(credName, name)
}

// iconUpload uploads an icon file and returns the new icon ID.
func iconUpload(credName, filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open icon file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", path.Base(filePath))
	if err != nil {
		return 0, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return 0, fmt.Errorf("write file to form: %w", err)
	}
	if err := w.Close(); err != nil {
		return 0, fmt.Errorf("close multipart: %w", err)
	}

	url, err := ResolveEndpoint(credName, "/api/v1/icon")
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("icon upload status %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse icon response: %w", err)
	}
	return result.ID, nil
}
