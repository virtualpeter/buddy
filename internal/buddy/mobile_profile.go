package buddy

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Mobile device configuration profiles are Classic-API only
// (/JSSResource/mobiledeviceconfigurationprofiles). The XML envelope mirrors the
// macOS profile shape (root <configuration_profile>, entity-encoded <payloads>),
// so the offline payloads_* / profile_xml_* plist helpers work unchanged.
const uriClassicMobileProfiles = "/JSSResource/mobiledeviceconfigurationprofiles/id"

// mobileProfileListXML parses GET /JSSResource/mobiledeviceconfigurationprofiles.
type mobileProfileListXML struct {
	XMLName  xml.Name `xml:"configuration_profiles"`
	Profiles []struct {
		ID      int    `xml:"id"`
		Name    string `xml:"name"`
		General *struct {
			Name string `xml:"name"`
		} `xml:"general"`
	} `xml:"configuration_profile"`
}

// MobileProfileSearch lists mobile device configuration profiles, with an optional
// case-insensitive name substring filter (client-side; Classic has no filter).
func MobileProfileSearch(credName string, nameFilter string, _ *slog.Logger) ([]ProfileRecord, error) {
	body, err := jamfGETWithAccept(credName, "/JSSResource/mobiledeviceconfigurationprofiles", "application/xml")
	if err != nil {
		return nil, fmt.Errorf("list mobile profiles: %w", err)
	}
	var list mobileProfileListXML
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse mobile profile list: %w", err)
	}
	var out []ProfileRecord
	filterLower := strings.ToLower(strings.TrimSpace(nameFilter))
	for _, p := range list.Profiles {
		name := p.Name
		if name == "" && p.General != nil {
			name = p.General.Name
		}
		if filterLower != "" && !strings.Contains(strings.ToLower(name), filterLower) {
			continue
		}
		out = append(out, ProfileRecord{ID: p.ID, Name: name})
	}
	return out, nil
}

// MobileProfileList fetches and prints mobile device configuration profiles.
func MobileProfileList(credName string, nameFilter string, logger *slog.Logger) error {
	records, err := MobileProfileSearch(credName, nameFilter, logger)
	if err != nil {
		return err
	}
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, r := range records {
		fmt.Printf("%-8d  %s\n", r.ID, truncate(r.Name, 60))
	}
	fmt.Printf("\n%d profile(s)\n", len(records))
	return nil
}

// MobileProfileFetchXML fetches the full mobile profile XML.
func MobileProfileFetchXML(credName string, profileID int) ([]byte, error) {
	body, err := jamfGETWithAccept(credName, fmt.Sprintf("%s/%d", uriClassicMobileProfiles, profileID), "application/xml")
	if err != nil {
		return nil, fmt.Errorf("fetch mobile profile %d: %w", profileID, err)
	}
	return body, nil
}

// MobileProfileGetXML fetches a mobile profile and writes its XML to w.
func MobileProfileGetXML(credName string, profileID int, w io.Writer, _ *slog.Logger) error {
	body, err := MobileProfileFetchXML(credName, profileID)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// MobileProfilePutXML PUTs full mobile profile XML back to Jamf. If redeploy is true,
// <redeploy_on_update> is set to "All" first.
func MobileProfilePutXML(credName string, opts Opts, profileID int, xmlBytes []byte, redeploy bool, logger *slog.Logger) error {
	if !bytes.Contains(xmlBytes, []byte("<configuration_profile")) {
		return fmt.Errorf("input does not appear to be mobile device configuration profile XML (missing <configuration_profile> root)")
	}
	if redeploy {
		xmlBytes = setRedeployAll(xmlBytes)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nPUT mobile device configuration profile (ID %d) with XML", profileID)
		if redeploy {
			msg += " [redeploy_on_update=All]"
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%d", uriClassicMobileProfiles, profileID), xmlBytes, "application/xml"); err != nil {
		return fmt.Errorf("update mobile profile %d: %w", profileID, err)
	}
	return nil
}

// MobileProfileCreateXML POSTs mobile profile XML to create a new profile and
// returns the new profile ID from the Classic response.
func MobileProfileCreateXML(credName string, opts Opts, xmlBytes []byte, logger *slog.Logger) (int, error) {
	if !bytes.Contains(xmlBytes, []byte("<configuration_profile")) {
		return 0, fmt.Errorf("input does not appear to be mobile device configuration profile XML (missing <configuration_profile> root)")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("\nPOST new mobile device configuration profile", logger) {
			return 0, nil
		}
	}
	out, err := jamfPOST(credName, uriClassicMobileProfiles+"/0", xmlBytes, "application/xml")
	if err != nil {
		return 0, fmt.Errorf("create mobile profile: %w", err)
	}
	var created struct {
		XMLName xml.Name `xml:"mobile_device_configuration_profile"`
		ID      int      `xml:"id"`
	}
	if err := xml.Unmarshal(out, &created); err != nil || created.ID == 0 {
		// Some Jamf versions return <configuration_profile><id>…
		var alt struct {
			ID int `xml:"id"`
		}
		if err2 := xml.Unmarshal(out, &alt); err2 == nil && alt.ID != 0 {
			return alt.ID, nil
		}
		return 0, fmt.Errorf("parse create response: %s", string(out))
	}
	return created.ID, nil
}

// MobileProfileDelete deletes a mobile device configuration profile by ID.
func MobileProfileDelete(credName string, opts Opts, profileID int, logger *slog.Logger) error {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nDelete mobile device configuration profile (ID %d)", profileID), logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, fmt.Sprintf("%s/%d", uriClassicMobileProfiles, profileID)); err != nil {
		return fmt.Errorf("delete mobile profile %d: %w", profileID, err)
	}
	return nil
}
