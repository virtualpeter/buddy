package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// PageAll means fetch every page (Starlark: omit page).
const PageAll = -1

// DefaultPageSize is the default Jamf page-size for list endpoints.
const DefaultPageSize = 200

// PageOpts controls Jamf collection pagination.
// Page is 0-based. Page == PageAll fetches all pages; Page >= 0 fetches that page only.
// PageSize <= 0 defaults to DefaultPageSize.
type PageOpts struct {
	Page     int
	PageSize int
}

// Normalize returns a copy with defaults applied.
func (o PageOpts) Normalize() PageOpts {
	if o.PageSize <= 0 {
		o.PageSize = DefaultPageSize
	}
	if o.Page < PageAll {
		o.Page = PageAll
	}
	return o
}

type jamfPageEnvelope struct {
	TotalCount int             `json:"totalCount"`
	Results    json.RawMessage `json:"results"`
}

// fetchJamfPaged GETs a Jamf paginated collection (JSON with totalCount + results).
// apiPath is absolute API path without query (e.g. "/api/v1/packages").
// extra may include filter/sort; page and page-size are set by this helper.
// onPage receives each page's results JSON array and the response totalCount;
// it should return the number of items in that page.
func fetchJamfPaged(credName, apiPath string, extra url.Values, opts PageOpts, onPage func(resultsJSON []byte, totalCount int) (n int, err error)) error {
	opts = opts.Normalize()
	if extra == nil {
		extra = url.Values{}
	}

	fetchOne := func(page int) (n int, total int, err error) {
		q := cloneURLValues(extra)
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("page-size", fmt.Sprintf("%d", opts.PageSize))
		// Match JamfRequest: pick auth/surface from the path (e.g. blueprints → platform).
		op := opClassForPath(http.MethodGet, apiPath)
		endpoint, err := ResolveEndpointOp(credName, apiPath, op)
		if err != nil {
			return 0, 0, err
		}
		endpoint = endpoint + "?" + q.Encode()
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := PerformRequestOp(req, credName, op)
		if err != nil {
			return 0, 0, fmt.Errorf("list %s (page %d): %w", apiPath, page, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return 0, 0, fmt.Errorf("read %s page %d: %w", apiPath, page, readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return 0, 0, fmt.Errorf("list %s status %d: %s", apiPath, resp.StatusCode, string(body))
		}
		var env jamfPageEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return 0, 0, fmt.Errorf("parse %s: %w", apiPath, err)
		}
		results := env.Results
		if len(results) == 0 {
			results = []byte("[]")
		}
		n, err = onPage(results, env.TotalCount)
		if err != nil {
			return 0, env.TotalCount, err
		}
		return n, env.TotalCount, nil
	}

	if opts.Page >= 0 {
		_, _, err := fetchOne(opts.Page)
		return err
	}

	var got int
	for page := 0; ; page++ {
		n, total, err := fetchOne(page)
		if err != nil {
			return err
		}
		got += n
		if n == 0 || got >= total {
			return nil
		}
	}
}

func cloneURLValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}
