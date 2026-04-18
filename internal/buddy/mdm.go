package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

const uriMdmCommands = "/api/v2/mdm/commands"
const defaultMdmPageSize = 200

// MdmCommandClient holds the client information from an MDM command record.
type MdmCommandClient struct {
	ManagementID string `json:"managementId"`
	ClientType   string `json:"clientType"`
}

// MdmCommandRecord holds the fields returned by GET /api/v2/mdm/commands.
type MdmCommandRecord struct {
	UUID          string            `json:"uuid"`
	DateSent      string            `json:"dateSent"`
	DateCompleted string            `json:"dateCompleted"`
	Client        MdmCommandClient  `json:"client"`
	CommandState  string            `json:"commandState"`
	CommandType   string            `json:"commandType"`
}

type mdmCommandListResponse struct {
	TotalCount int                `json:"totalCount"`
	Results    []MdmCommandRecord `json:"results"`
}

// MdmCommandFind fetches MDM commands matching the given RSQL filter.
// The Jamf Pro API requires at least one filter field; an empty filter will be
// rejected by the server. sort defaults to "dateSent:asc" when empty.
// pageSize controls how many records are requested per API call; values <= 0
// default to defaultMdmPageSize. limit caps the total records returned across
// all pages; 0 means no cap (fetch everything).
func MdmCommandFind(credName, filter, sort string, pageSize, limit int, _ *slog.Logger) ([]MdmCommandRecord, error) {
	if filter == "" {
		return nil, fmt.Errorf("filter is required (API requires at least one RSQL filter field)")
	}
	if sort == "" {
		sort = "dateSent:asc"
	}
	if pageSize <= 0 {
		pageSize = defaultMdmPageSize
	}
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	var all []MdmCommandRecord
	for page := 0; ; page++ {
		// When a limit is set, reduce the page size on the final page to avoid
		// fetching more records than necessary.
		fetch := pageSize
		if limit > 0 {
			remaining := limit - len(all)
			if remaining <= 0 {
				break
			}
			if remaining < fetch {
				fetch = remaining
			}
		}
		endpoint := fmt.Sprintf("%s%s?page=%d&page-size=%d&sort=%s&filter=%s",
			baseURL, uriMdmCommands, page, fetch,
			url.QueryEscape(sort),
			url.QueryEscape(filter),
		)
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := PerformRequest(nil, req, credName)
		if err != nil {
			return nil, fmt.Errorf("list MDM commands (page %d): %w", page, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list MDM commands status %d: %s", resp.StatusCode, string(body))
		}
		var result mdmCommandListResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		all = append(all, result.Results...)
		// Stop when the page is not full — this is the last page.
		// totalCount from this API reflects the page count, not the global
		// total, so it cannot be used as a termination sentinel.
		if len(result.Results) < fetch {
			break
		}
	}
	return all, nil
}

// MdmCommandFindPrint fetches MDM commands and prints them as a table.
func MdmCommandFindPrint(credName, filter, sort string, pageSize, limit int, logger *slog.Logger) error {
	records, err := MdmCommandFind(credName, filter, sort, pageSize, limit, logger)
	if err != nil {
		return err
	}
	fmt.Printf("%-38s  %-30s  %-13s  %-12s  %-36s  %-24s  %s\n",
		"UUID", "Command", "State", "ClientType", "ManagementID", "DateSent", "DateCompleted")
	fmt.Printf("%-38s  %-30s  %-13s  %-12s  %-36s  %-24s  %s\n",
		"--------------------------------------",
		"------------------------------",
		"-------------",
		"------------",
		"------------------------------------",
		"------------------------",
		"------------------------",
	)
	for _, r := range records {
		fmt.Printf("%-38s  %-30s  %-13s  %-12s  %-36s  %-24s  %s\n",
			r.UUID,
			truncateMdm(r.CommandType, 30),
			truncateMdm(r.CommandState, 13),
			truncateMdm(r.Client.ClientType, 12),
			r.Client.ManagementID,
			truncateMdm(r.DateSent, 24),
			r.DateCompleted,
		)
	}
	fmt.Printf("\n%d command(s)\n", len(records))
	return nil
}

func truncateMdm(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
