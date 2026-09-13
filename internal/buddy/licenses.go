package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// uriLicenses is the logical Account Licensing list path (rewritten to
// /licensing/v1/licenses on the organization gateway).
const uriLicenses = "/api/licensing/v1/licenses"

// LicenseListValues lists Jamf Account licenses for the organization on the
// access token. Not paginated; empty org is 200 with an empty results list.
func LicenseListValues(credName string, logger *slog.Logger) ([]map[string]interface{}, error) {
	if err := requireOrgPlatformCred(credName); err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, uriLicenses)
	if err != nil {
		return nil, err
	}
	var env struct {
		TotalCount int                      `json:"totalCount"`
		Results    []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse licenses: %w", err)
	}
	if env.Results == nil {
		return []map[string]interface{}{}, nil
	}
	return env.Results, nil
}

// LicenseList prints a table of Account licenses.
func LicenseList(credName string, logger *slog.Logger) error {
	items, err := LicenseListValues(credName, logger)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No licenses found.")
		return nil
	}
	fmt.Printf("%-28s  %-18s  %-12s  %6s  %-12s  %s\n", "Product", "SKU", "Type", "Seats", "Start", "End")
	fmt.Printf("%-28s  %-18s  %-12s  %6s  %-12s  %s\n", strings.Repeat("-", 28), strings.Repeat("-", 18), strings.Repeat("-", 12), strings.Repeat("-", 6), strings.Repeat("-", 12), strings.Repeat("-", 12))
	for _, row := range items {
		product := licenseField(row, "productName", "title")
		if len(product) > 28 {
			product = product[:25] + "..."
		}
		fmt.Printf("%-28s  %-18s  %-12s  %6s  %-12s  %s\n",
			product,
			licenseField(row, "sku"),
			licenseField(row, "licenseType"),
			licenseField(row, "purchasedSeats"),
			licenseField(row, "startDate"),
			licenseField(row, "endDate"),
		)
	}
	return nil
}

func licenseField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			continue
		}
		return s
	}
	return ""
}
