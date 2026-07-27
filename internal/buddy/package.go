package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

// PackageRecord represents a Jamf Pro package record from the v1 API.
type PackageRecord struct {
	ID            string `json:"id"`
	PackageName   string `json:"packageName"`
	FileName      string `json:"fileName"`
	CategoryID    string `json:"categoryId"`
	Info          string `json:"info,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Priority      int    `json:"priority"`
	LastChanged   string `json:"lastChanged,omitempty"`
	LastChangedBy string `json:"lastChangedBy,omitempty"`
}

// PackageHistoryEntry is one record from GET /api/v1/packages/{id}/history.
type PackageHistoryEntry struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Date     string `json:"date"` // ISO8601 e.g. "2025-11-12T14:23:07.000Z"
	Note     string `json:"note,omitempty"`
	Details  string `json:"details,omitempty"`
}

type packageHistoryResponse struct {
	TotalCount int                   `json:"totalCount"`
	Results    []PackageHistoryEntry `json:"results"`
}

// PackagesSearchResults is the paginated response from GET /api/v1/packages.
type PackagesSearchResults struct {
	TotalCount int             `json:"totalCount"`
	Results    []PackageRecord `json:"results"`
}

// packageDeleteRequest is the body for POST /api/v1/packages/delete-multiple.
type packageDeleteRequest struct {
	IDs []string `json:"ids"`
}

// PackageList fetches and prints packages, with an optional RSQL filter.
// sortOrder is "" (no sort), "asc", or "desc" for client-side version sort of PackageName.
// Example filter: packageName=="*Chrome*"
func PackageList(credName, filter, sortOrder string, logger *slog.Logger) error {
	results, err := PackageSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if sortOrder != "" {
		if err := SortPackageRecordsByVersion(results, sortOrder); err != nil {
			return err
		}
	}
	fmt.Printf("%-8s  %-40s  %s\n", "ID", "Package Name", "File Name")
	fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "--------------------")
	for _, p := range results {
		fmt.Printf("%-8s  %-40s  %s\n", p.ID, truncatePkg(p.PackageName, 40), p.FileName)
	}
	fmt.Printf("\n%d package(s)\n", len(results))
	return nil
}

// PackageSearch returns package records matching the optional RSQL filter without printing.
// Used by DSL builtins that need structured data rather than human-readable output.
// When opts.Page is PageAll, all pages are fetched.
func PackageSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]PackageRecord, error) {
	extra := url.Values{}
	if filter != "" {
		extra.Set("filter", filter)
	}
	var all []PackageRecord
	err := fetchJamfPaged(credName, "/api/v1/packages", extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []PackageRecord
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// PackageGet fetches and pretty-prints a single package record by ID.
// It also fetches the most recent history entry and injects lastChanged and
// lastChangedBy into the printed record.
func PackageGet(credName, id string, logger *slog.Logger) error {
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("get package %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get package %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var pkg PackageRecord
	if err := json.Unmarshal(body, &pkg); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if latest, err := packageGetLatestHistory(credName, id); err == nil && latest != nil {
		pkg.LastChanged = latest.Date
		pkg.LastChangedBy = latest.Username
	}
	out, _ := json.MarshalIndent(pkg, "", "  ")
	fmt.Println(string(out))
	return nil
}

// PackageDelete deletes one or more packages by ID.
// A single ID uses DELETE /api/v1/packages/{id}.
// Multiple IDs use POST /api/v1/packages/delete-multiple.
func PackageDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one package ID required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete %d package record(s): %s", len(ids), strings.Join(ids, ", ")), logger) {
			return nil
		}
	}
	if len(ids) == 1 {
		return packageDeleteOne(credName, ids[0])
	}
	return packageDeleteMultiple(credName, ids)
}

func packageDeleteOne(credName, id string) error {
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("delete package %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete package %s status %d: %s", id, resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted package (ID %s)\n", id)
	return nil
}

func packageDeleteMultiple(credName string, ids []string) error {
	payload := packageDeleteRequest{IDs: ids}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/delete-multiple")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("delete packages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete packages status %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted %d package(s): %s\n", len(ids), strings.Join(ids, ", "))
	return nil
}

// PackageFindID finds a package by app name and version using the v1 API.
// It filters server-side by app name and then picks the best version match in Go,
// using the same heuristic as the legacy Classic API path in policy.go.
// Returns the string ID and package name, or empty strings if not found.
//
// When patternOverride is non-empty, it overrides the default app+version search:
//   - Exact: "FalconSensorMacOS-7.32.20408.pkg" → RSQL exact match
//   - Template: "FalconSensorMacOS-{version}.pkg" → substitute {version}, then exact RSQL
//   - Regex: "re:FalconSensor.*\.pkg" → broad RSQL + client-side regex filter
func PackageFindID(credName, appName, version, patternOverride string, logger *slog.Logger) (id, name string, err error) {
	if patternOverride != "" {
		return packageFindByPattern(credName, appName, version, patternOverride, logger)
	}
	return packageFindByAppVersion(credName, appName, version, logger)
}

func packageFindByAppVersion(credName, appName, version string, logger *slog.Logger) (id, name string, err error) {
	// Build RSQL: packageName=="*appName*"
	rsql := `packageName=="*` + appName + `*"`
	results, err := PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", "", err
	}

	appLower := strings.ToLower(appName)
	versionLower := strings.ToLower(version)
	var candidates []PackageRecord
	for _, p := range results {
		n := strings.ToLower(p.PackageName)
		if strings.Contains(n, appLower) && strings.Contains(n, versionLower) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return "", "", nil
	}
	// Prefer exact version match extracted from the package name.
	for _, c := range candidates {
		if strings.EqualFold(ExtractVersion(c.PackageName), version) {
			return c.ID, c.PackageName, nil
		}
	}
	return candidates[0].ID, candidates[0].PackageName, nil
}

func packageFindByPattern(credName, appName, version, patternOverride string, logger *slog.Logger) (id, name string, err error) {
	const rePrefix = "re:"
	if strings.HasPrefix(patternOverride, rePrefix) {
		return packageFindByRegex(credName, version, strings.TrimPrefix(patternOverride, rePrefix), logger)
	}
	// Exact or template: substitute {version}
	target := strings.ReplaceAll(patternOverride, "{version}", version)
	escaped := EscapeRSQLString(target)
	rsql := `packageName=="` + escaped + `"`
	results, err := PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", "", err
	}
	if len(results) == 0 {
		return "", "", nil
	}
	return results[0].ID, results[0].PackageName, nil
}

// packageFindByRegex fetches packages with a broad RSQL filter, then filters by regex.
func packageFindByRegex(credName, version, rePattern string, logger *slog.Logger) (id, name string, err error) {
	re, err := regexp.Compile(rePattern)
	if err != nil {
		return "", "", fmt.Errorf("invalid regex pattern %q: %w", rePattern, err)
	}
	// Extract a safe substring for RSQL: longest alphanumeric prefix before first regex metachar.
	rsqlHint := extractRSQLHintFromRegex(rePattern)
	var results []PackageRecord
	if rsqlHint != "" {
		rsql := `packageName=="*` + rsqlHint + `*"`
		results, err = PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	} else {
		results, err = PackageSearch(credName, "", PageOpts{Page: PageAll}, logger)
	}
	if err != nil {
		return "", "", err
	}
	for _, p := range results {
		if re.MatchString(p.PackageName) {
			return p.ID, p.PackageName, nil
		}
	}
	return "", "", nil
}

// extractRSQLHintFromRegex returns the longest alphanumeric prefix before the first regex metachar.
// Used to narrow RSQL results before client-side regex filtering.
func extractRSQLHintFromRegex(rePattern string) string {
	var b strings.Builder
	for _, r := range rePattern {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			break
		}
	}
	return b.String()
}

// PackageFull is the complete Package object required for PUT /api/v1/packages/{id}.
// All boolean required fields are always serialised (no omitempty) so the API
// receives a valid body even when their value is false.
type PackageFull struct {
	ID                   string `json:"id,omitempty"`
	PackageName          string `json:"packageName"`
	FileName             string `json:"fileName"`
	CategoryID           string `json:"categoryId"`
	Info                 string `json:"info,omitempty"`
	Notes                string `json:"notes,omitempty"`
	Priority             int    `json:"priority"`
	OsRequirements       string `json:"osRequirements,omitempty"`
	FillUserTemplate     bool   `json:"fillUserTemplate"`
	FillExistingUsers    bool   `json:"fillExistingUsers,omitempty"`
	Swu                  bool   `json:"swu,omitempty"`
	RebootRequired       bool   `json:"rebootRequired"`
	SelfHealNotify       bool   `json:"selfHealNotify,omitempty"`
	SelfHealingAction    string `json:"selfHealingAction,omitempty"`
	OsInstall            bool   `json:"osInstall"`
	SerialNumber         string `json:"serialNumber,omitempty"`
	ParentPackageID      string `json:"parentPackageId,omitempty"`
	BasePath             string `json:"basePath,omitempty"`
	SuppressUpdates      bool   `json:"suppressUpdates"`
	IgnoreConflicts      bool   `json:"ignoreConflicts,omitempty"`
	SuppressFromDock     bool   `json:"suppressFromDock"`
	SuppressEula         bool   `json:"suppressEula"`
	SuppressRegistration bool   `json:"suppressRegistration"`
	InstallLanguage      string `json:"installLanguage,omitempty"`
	Md5                  string `json:"md5,omitempty"`
	Sha256               string `json:"sha256,omitempty"`
	HashType             string `json:"hashType,omitempty"`
	HashValue            string `json:"hashValue,omitempty"`
	OsInstallerVersion   string `json:"osInstallerVersion,omitempty"`
	Manifest             string `json:"manifest,omitempty"`
	ManifestFileName     string `json:"manifestFileName,omitempty"`
}

// packageGetFull fetches the full Package object for a given ID without printing.
func packageGetFull(credName, id string) (*PackageFull, error) {
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("get package %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get package %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var pkg PackageFull
	if err := json.Unmarshal(body, &pkg); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &pkg, nil
}

// PackageFetchFull fetches the full Package object for a given ID (for script/component use).
func PackageFetchFull(credName, id string) (*PackageFull, error) {
	return packageGetFull(credName, id)
}

// packagePut sends a full Package object to PUT /api/v1/packages/{id}.
func packagePut(credName, id string, pkg *PackageFull) (*PackageFull, error) {
	data, err := json.Marshal(pkg)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("update package %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update package %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var updated PackageFull
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &updated, nil
}

// PackagePutFields fetches the existing package, applies updates from fields, then PUTs the full payload.
// Intended for scripts/component use where callers want to update a subset of fields.
//
// Supported fields (keys are the API JSON field names):
// - packageName (string)
// - notes (string)
// - info (string)
// - priority (int)
// - categoryId (string)
// - rebootRequired (bool)
// - suppressUpdates (bool)
// - suppressFromDock (bool)
// - suppressEula (bool)
// - suppressRegistration (bool)
// - fillUserTemplate (bool)
// - osInstall (bool)
func PackagePutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*PackageFull, error) {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return nil, err
	}

	// Apply updates.
	if v, ok := fields["packageName"].(string); ok {
		pkg.PackageName = v
	}
	if v, ok := fields["notes"].(string); ok {
		pkg.Notes = v
	}
	if v, ok := fields["info"].(string); ok {
		pkg.Info = v
	}
	if v, ok := fields["categoryId"].(string); ok {
		pkg.CategoryID = v
	}
	if v, ok := fields["priority"]; ok {
		switch n := v.(type) {
		case int:
			pkg.Priority = n
		case int64:
			pkg.Priority = int(n)
		case float64:
			pkg.Priority = int(n)
		}
	}
	if v, ok := fields["rebootRequired"].(bool); ok {
		pkg.RebootRequired = v
	}
	if v, ok := fields["suppressUpdates"].(bool); ok {
		pkg.SuppressUpdates = v
	}
	if v, ok := fields["suppressFromDock"].(bool); ok {
		pkg.SuppressFromDock = v
	}
	if v, ok := fields["suppressEula"].(bool); ok {
		pkg.SuppressEula = v
	}
	if v, ok := fields["suppressRegistration"].(bool); ok {
		pkg.SuppressRegistration = v
	}
	if v, ok := fields["fillUserTemplate"].(bool); ok {
		pkg.FillUserTemplate = v
	}
	if v, ok := fields["osInstall"].(bool); ok {
		pkg.OsInstall = v
	}

	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT package (ID %s) with updated fields", id), logger) {
			return nil, nil
		}
	}
	return packagePut(credName, id, pkg)
}

// packageCreatePayload is the minimal payload for POST /api/v1/packages (create).
type packageCreatePayload struct {
	PackageName          string `json:"packageName"`
	FileName             string `json:"fileName"`
	CategoryID           string `json:"categoryId"`
	Priority             int    `json:"priority"`
	FillUserTemplate     bool   `json:"fillUserTemplate"`
	RebootRequired       bool   `json:"rebootRequired"`
	OsInstall            bool   `json:"osInstall"`
	SuppressUpdates      bool   `json:"suppressUpdates"`
	SuppressFromDock     bool   `json:"suppressFromDock"`
	SuppressEula         bool   `json:"suppressEula"`
	SuppressRegistration bool   `json:"suppressRegistration"`
}

// packageCreate POSTs a new package record and returns the created package ID.
func packageCreate(credName string, payload *packageCreatePayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal create payload: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return "", fmt.Errorf("create package: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create package status %d: %s", resp.StatusCode, string(body))
	}
	var created PackageRecord
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("create package response missing id")
	}
	return created.ID, nil
}

// packageFindByFileName searches for a package with the given fileName (exact match).
// Returns the first match's ID if found, otherwise "" and nil.
// Used to allow retrying uploads when the create step succeeded but upload failed.
func packageFindByFileName(credName, fileName string, logger *slog.Logger) (id string, err error) {
	escaped := EscapeRSQLString(fileName)
	rsql := `fileName=="` + escaped + `"`
	results, err := PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	return results[0].ID, nil
}

// progressReader wraps io.Reader and invokes onProgress as bytes are read.
type progressReader struct {
	r         io.Reader
	total     int64
	read      int64
	onProgress func(read, total int64)
}

func (p *progressReader) Read(b []byte) (n int, err error) {
	n, err = p.r.Read(b)
	p.read += int64(n)
	if p.onProgress != nil {
		p.onProgress(p.read, p.total)
	}
	return n, err
}

// formatBytes returns human-readable size (B, KB, MB, GB).
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMG"[exp])
}

// progressBarWidth is the number of characters for the bar.
const progressBarWidth = 40

// formatProgressBar returns a line like "[########################--------] 75% (1.1 GB / 1.5 GB)".
func formatProgressBar(read, total int64) string {
	if total <= 0 {
		return ""
	}
	pct := float64(read) / float64(total)
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * progressBarWidth)
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", progressBarWidth-filled)
	return fmt.Sprintf("[%s] %.0f%% (%s / %s)", bar, pct*100, formatBytes(read), formatBytes(total))
}

// minProgressSize is the minimum file size (bytes) to show progress bar.
const minProgressSize = 100 * 1024 // 100 KB

// packageUploadFile POSTs the file as multipart/form-data to /api/v1/packages/{id}/upload.
func packageUploadFile(credName, id string, file *os.File, opts Opts) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	total := info.Size()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", path.Base(file.Name()))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	var src io.Reader = file
	if total >= minProgressSize && total > 0 {
		var lastPct int
		src = &progressReader{
			r:     file,
			total: total,
			onProgress: func(read, tot int64) {
				pct := int(100 * float64(read) / float64(tot))
				if pct > 100 {
					pct = 100
				}
				// Throttle: update every 1% or at completion
				if pct != lastPct && (pct-lastPct >= 1 || pct == 100 || read == tot) {
					lastPct = pct
					fmt.Fprintf(os.Stderr, "\r%s", formatProgressBar(read, tot))
				}
			},
		}
	}
	if _, err := io.Copy(part, src); err != nil {
		return fmt.Errorf("write file to form: %w", err)
	}
	if total >= minProgressSize && total > 0 {
		fmt.Fprintf(os.Stderr, "\r%s\n", formatProgressBar(total, total))
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id + "/upload")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("upload package file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload package file status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// PackageUpload creates a new package record and uploads the file to Jamf Pro.
func PackageUpload(credName string, opts Opts, filePath string, nameOverride, categoryID string, priority int, logger *slog.Logger) error {
	if priority < 1 || priority > 20 {
		return fmt.Errorf("priority must be between 1 and 20, got %d", priority)
	}
	fileName := path.Base(filePath)
	ext := strings.ToLower(path.Ext(fileName))
	if ext != ".pkg" && ext != ".dmg" {
		return fmt.Errorf("unsupported file type %q: only .pkg and .dmg are supported (bundle packages must be zipped first)", ext)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()
	packageName := nameOverride
	if packageName == "" {
		packageName = fileName
	}
	if categoryID == "" {
		categoryID = "-1"
	}
	payload := &packageCreatePayload{
		PackageName:          packageName,
		FileName:             fileName,
		CategoryID:           categoryID,
		Priority:             priority,
		FillUserTemplate:     false,
		RebootRequired:       false,
		OsInstall:            false,
		SuppressUpdates:      false,
		SuppressFromDock:     false,
		SuppressEula:         false,
		SuppressRegistration: false,
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpload package %q as %q (priority %d)?", filePath, packageName, priority), logger) {
			return nil
		}
	}
	var id string
	existingID, err := packageFindByFileName(credName, fileName, logger)
	if err != nil {
		return err
	}
	if existingID != "" {
		id = existingID
	} else {
		id, err = packageCreate(credName, payload)
		if err != nil {
			return err
		}
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek file for upload: %w", err)
	}
	if err := packageUploadFile(credName, id, file, opts); err != nil {
		return err
	}
	fmt.Printf("Uploaded package %q (ID %s)\n", packageName, id)
	return nil
}

// PackageSetName updates the display name of a package record.
func PackageSetName(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s name: %q → %q", id, pkg.PackageName, value), logger) {
			return nil
		}
	}
	pkg.PackageName = value
	updated, err := packagePut(credName, id, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("Updated package %s: name = %q\n", id, updated.PackageName)
	return nil
}

// PackageSetNotes updates the notes field of a package record.
func PackageSetNotes(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s (%s) notes", id, pkg.PackageName), logger) {
			return nil
		}
	}
	pkg.Notes = value
	if _, err := packagePut(credName, id, pkg); err != nil {
		return err
	}
	fmt.Printf("Updated package %s: notes set\n", id)
	return nil
}

// PackageSetInfo updates the info field of a package record.
func PackageSetInfo(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s (%s) info", id, pkg.PackageName), logger) {
			return nil
		}
	}
	pkg.Info = value
	if _, err := packagePut(credName, id, pkg); err != nil {
		return err
	}
	fmt.Printf("Updated package %s: info set\n", id)
	return nil
}

// PackageSetPriority updates the install priority of a package record (valid range 1–10).
func PackageSetPriority(credName, id string, opts Opts, priority int, logger *slog.Logger) error {
	if priority < 1 || priority > 10 {
		return fmt.Errorf("priority must be between 1 and 10, got %d", priority)
	}
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s (%s) priority: %d → %d", id, pkg.PackageName, pkg.Priority, priority), logger) {
			return nil
		}
	}
	pkg.Priority = priority
	updated, err := packagePut(credName, id, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("Updated package %s: priority = %d\n", id, updated.Priority)
	return nil
}

// PackageSetCategory updates the category of a package record.
// Use categoryId "-1" to clear the category.
func PackageSetCategory(credName, id string, opts Opts, categoryID string, logger *slog.Logger) error {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s (%s) category: %q → %q", id, pkg.PackageName, pkg.CategoryID, categoryID), logger) {
			return nil
		}
	}
	pkg.CategoryID = categoryID
	updated, err := packagePut(credName, id, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("Updated package %s: categoryId = %s\n", id, updated.CategoryID)
	return nil
}

// PackageSetReboot updates the rebootRequired flag of a package record.
func PackageSetReboot(credName, id string, opts Opts, value bool, logger *slog.Logger) error {
	pkg, err := packageGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set package %s (%s) rebootRequired: %v → %v", id, pkg.PackageName, pkg.RebootRequired, value), logger) {
			return nil
		}
	}
	pkg.RebootRequired = value
	updated, err := packagePut(credName, id, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("Updated package %s: rebootRequired = %v\n", id, updated.RebootRequired)
	return nil
}

// packageGetLatestHistory fetches the single most-recent history entry for a package.
// Returns nil (no error) when no history entries exist.
func packageGetLatestHistory(credName, id string) (*PackageHistoryEntry, error) {
	endpoint, err := ResolveEndpoint(credName, "/api/v1/packages/" + id + "/history?sort=date:desc&page-size=1")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("get package history %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get package history %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var result packageHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse history response: %w", err)
	}
	if len(result.Results) == 0 {
		return nil, nil
	}
	return &result.Results[0], nil
}

// PackageHistory fetches and prints the recent history for a package record.
// limit controls the maximum number of entries shown (default 20).
func PackageHistory(credName, id string, limit int, logger *slog.Logger) error {
	if limit <= 0 {
		limit = 20
	}
	endpoint, err := ResolveEndpoint(credName, fmt.Sprintf("/api/v1/packages/%s/history?sort=date:desc&page-size=%d", id, limit))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("get package history %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get package history %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var result packageHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse history response: %w", err)
	}
	if len(result.Results) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}
	fmt.Printf("%-30s  %-20s  %s\n", "Date", "User", "Note")
	fmt.Printf("%-30s  %-20s  %s\n", "------------------------------", "--------------------", "--------------------")
	for _, e := range result.Results {
		fmt.Printf("%-30s  %-20s  %s\n", e.Date, truncatePkg(e.Username, 20), truncatePkg(e.Note, 60))
	}
	fmt.Printf("\n%d of %d history entry/entries\n", len(result.Results), result.TotalCount)
	return nil
}

// truncatePkg shortens s to at most n runes, adding "…" if truncated.
func truncatePkg(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

