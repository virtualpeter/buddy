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

// MdmCommandClient holds the client information from an MDM command record.
type MdmCommandClient struct {
	ManagementID string `json:"managementId"`
	ClientType   string `json:"clientType"`
}

// MdmCommandRecord holds the fields returned by GET /api/v2/mdm/commands.
type MdmCommandRecord struct {
	UUID          string           `json:"uuid"`
	DateSent      string           `json:"dateSent"`
	DateCompleted string           `json:"dateCompleted"`
	Client        MdmCommandClient `json:"client"`
	CommandState  string           `json:"commandState"`
	CommandType   string           `json:"commandType"`
}

type mdmCommandListResponse struct {
	TotalCount int                `json:"totalCount"`
	Results    []MdmCommandRecord `json:"results"`
}

// MdmCommandFind fetches MDM commands matching the given RSQL filter.
// The Jamf Pro API requires at least one filter field; an empty filter will be
// rejected by the server. sort defaults to "dateSent:asc" when empty.
//
// When opts.Page is PageAll, pages are walked until exhausted. When opts.Page >= 0,
// only that page is returned. limit caps the total records when walking all pages
// (0 = no cap); ignored for a single-page fetch.
//
// Note: this endpoint's totalCount is unreliable as a global total, so all-page
// termination uses a short final page rather than fetchJamfPaged.
func MdmCommandFind(credName, filter, sort string, opts PageOpts, limit int, _ *slog.Logger) ([]MdmCommandRecord, error) {
	if filter == "" {
		return nil, fmt.Errorf("filter is required (API requires at least one RSQL filter field)")
	}
	if sort == "" {
		sort = "dateSent:asc"
	}
	opts = opts.Normalize()
	baseEndpoint, err := ResolveEndpoint(credName, uriMdmCommands)
	if err != nil {
		return nil, err
	}

	fetchPage := func(page, pageSize int) ([]MdmCommandRecord, error) {
		endpoint := fmt.Sprintf("%s?page=%d&page-size=%d&sort=%s&filter=%s",
			baseEndpoint, page, pageSize,
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
		return result.Results, nil
	}

	if opts.Page >= 0 {
		return fetchPage(opts.Page, opts.PageSize)
	}

	var all []MdmCommandRecord
	for page := 0; ; page++ {
		fetch := opts.PageSize
		if limit > 0 {
			remaining := limit - len(all)
			if remaining <= 0 {
				break
			}
			if remaining < fetch {
				fetch = remaining
			}
		}
		results, err := fetchPage(page, fetch)
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
		// Stop when the page is not full — this is the last page.
		if len(results) < fetch {
			break
		}
	}
	return all, nil
}

// MdmCommandFindPrint fetches MDM commands and prints them as a table.
func MdmCommandFindPrint(credName, filter, sort string, opts PageOpts, limit int, logger *slog.Logger) error {
	records, err := MdmCommandFind(credName, filter, sort, opts, limit, logger)
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
