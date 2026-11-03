package buddy

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const uriPackages = "/api/v1/packages"

// PackageRecord is a Jamf Pro package from the v1 API (list or get).
// List endpoints may leave optional fields empty; GET /api/v1/packages/{id}
// returns the full object including checksums. LastChanged/LastChangedBy are
// client-enriched from history when printing package get (not Jamf package fields).
type PackageRecord struct {
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
	Sha3512              string `json:"sha3512,omitempty"`
	HashType             string `json:"hashType,omitempty"`
	HashValue            string `json:"hashValue,omitempty"`
	Size                 string `json:"size,omitempty"`                // server-managed, read-only
	CloudTransferStatus  string `json:"cloudTransferStatus,omitempty"` // server-managed; READY when JCDS available
	OsInstallerVersion   string `json:"osInstallerVersion,omitempty"`
	Manifest             string `json:"manifest,omitempty"`
	ManifestFileName     string `json:"manifestFileName,omitempty"`
	Format               string `json:"format,omitempty"`
	LastChanged          string `json:"lastChanged,omitempty"`
	LastChangedBy        string `json:"lastChangedBy,omitempty"`
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
func PackageList(credName, filter, sortOrder string, asJSON bool, logger *slog.Logger) error {
	results, err := PackageSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if sortOrder != "" {
		if err := SortPackageRecordsByVersion(results, sortOrder); err != nil {
			return err
		}
	}
	if asJSON {
		return printJSONValue(results)
	}
	fmt.Printf("%-8s  %-40s  %s\n", "ID", "Package Name", "File Name")
	fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "--------------------")
	for _, p := range results {
		fmt.Printf("%-8s  %-40s  %s\n", p.ID, truncatePkg(p.PackageName, 40), p.FileName)
	}
	fmt.Printf("\n%d package(s)\n", len(results))
	return nil
}

// PackageSearch returns package records matching the optional filter without printing.
// Bare term → packageName=="*term*"; RSQL expression → pass-through.
// When opts.Page is PageAll, all pages are fetched.
func PackageSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]PackageRecord, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecPackages)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []PackageRecord
	err = fetchJamfPaged(credName, uriPackages, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
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
	pkg, err := packageGet(credName, id)
	if err != nil {
		return err
	}
	if latest, err := packageGetLatestHistory(credName, id); err == nil && latest != nil {
		pkg.LastChanged = latest.Date
		pkg.LastChangedBy = latest.Username
	}
	return printJSON(pkg)
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
	if err := jamfDELETE(credName, uriPackages+"/"+id); err != nil {
		return fmt.Errorf("delete package %s: %w", id, err)
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
	if _, err := jamfPOST(credName, uriPackages+"/delete-multiple", data, "application/json"); err != nil {
		return fmt.Errorf("delete packages: %w", err)
	}
	fmt.Printf("Deleted %d package(s): %s\n", len(ids), strings.Join(ids, ", "))
	return nil
}

// PackageFindID finds a package by name keyword and version using the v1 API.
// It filters server-side by the keyword and then picks the best version match in Go,
// using the same heuristic as the legacy Classic API path in policy.go.
// Returns the string ID and package name, or empty strings if not found.
//
// When patternOverride is non-empty, it overrides the default name+version search:
//   - Exact: "FalconSensorMacOS-7.32.20408.pkg" → RSQL exact match
//   - Template: "FalconSensorMacOS-{version}.pkg" → substitute {version}, then exact RSQL
//   - Regex: "re:FalconSensor.*\.pkg" → broad RSQL + client-side regex filter
func PackageFindID(credName, name, version, patternOverride string, logger *slog.Logger) (id, pkgName string, err error) {
	if patternOverride != "" {
		return packageFindByPattern(credName, name, version, patternOverride, logger)
	}
	return packageFindByNameVersion(credName, name, version, logger)
}

// PackageFindIDByExactName returns the package id whose packageName equals name (case-insensitive).
func PackageFindIDByExactName(credName, name string, logger *slog.Logger) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("package name is required")
	}
	escaped := EscapeRSQLString(name)
	rsql := `packageName=="` + escaped + `"`
	results, err := PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", err
	}
	nameLower := strings.ToLower(name)
	for _, p := range results {
		if strings.ToLower(p.PackageName) == nameLower {
			return p.ID, nil
		}
	}
	// Client-side fallback if RSQL exact match is picky about quoting.
	all, err := PackageSearch(credName, "", PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", err
	}
	for _, p := range all {
		if strings.ToLower(p.PackageName) == nameLower {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no package found with name %q", name)
}

func packageFindByNameVersion(credName, name, version string, logger *slog.Logger) (id, pkgName string, err error) {
	// Build RSQL: packageName=="*name*"
	rsql := `packageName=="*` + name + `*"`
	results, err := PackageSearch(credName, rsql, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", "", err
	}

	nameLower := strings.ToLower(name)
	versionLower := strings.ToLower(version)
	var candidates []PackageRecord
	for _, p := range results {
		n := strings.ToLower(p.PackageName)
		if strings.Contains(n, nameLower) && strings.Contains(n, versionLower) {
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

func packageFindByPattern(credName, _, version, patternOverride string, logger *slog.Logger) (id, pkgName string, err error) {
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

// packageGet fetches a package by ID (full v1 object).
func packageGet(credName, id string) (*PackageRecord, error) {
	pkg, status, err := packageGetStatus(credName, id)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get package %s status %d", id, status)
	}
	return pkg, nil
}

// packageGetStatus GETs /api/v1/packages/{id} and returns the HTTP status.
// On 404, pkg is nil and err is nil.
func packageGetStatus(credName, id string) (pkg *PackageRecord, status int, err error) {
	endpoint, err := ResolveEndpoint(credName, uriPackages+"/"+id)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, 0, fmt.Errorf("get package %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, err := readHTTPBody(resp)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("get package %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var out PackageRecord
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse response: %w", err)
	}
	return &out, http.StatusOK, nil
}

// PackageFetchFull fetches the package object for a given ID (for script/component use).
func PackageFetchFull(credName, id string) (*PackageRecord, error) {
	return packageGet(credName, id)
}

// packageCloudTransferReady is Jamf’s cloudTransferStatus when the package is
// available from JCDS (UI “Availability Pending” clears).
const packageCloudTransferReady = "READY"

const packageWaitReadyDefaultTimeout = 10 * time.Minute

func packageIsCloudReady(pkg *PackageRecord) bool {
	return pkg != nil && pkg.CloudTransferStatus == packageCloudTransferReady
}

// packageHasComparableChecksum reports whether the record has a checksum we can
// compare to a local file (sha256, md5, or MD5/SHA-256 hashValue).
func packageHasComparableChecksum(pkg *PackageRecord) bool {
	if pkg == nil {
		return false
	}
	if strings.TrimSpace(pkg.Sha256) != "" || strings.TrimSpace(pkg.Md5) != "" {
		return true
	}
	if strings.TrimSpace(pkg.HashValue) == "" {
		return false
	}
	switch strings.ToUpper(strings.ReplaceAll(pkg.HashType, "-", "_")) {
	case "SHA256", "SHA_256", "MD5":
		return true
	default:
		return false
	}
}

// packageHasPendingTransferEvidence reports whether Jamf has started cloud
// transfer for this package. Mid-transfer records typically populate hashValue
// with a non-comparable type (usually SHA3_512 / hashType SHA3_512) while sha256
// is still empty. A failed upload leaves no hashValue, or hashValue set to
// SHA3-512 of an empty file, and no comparable checksums.
func packageHasPendingTransferEvidence(pkg *PackageRecord) bool {
	if pkg == nil || strings.TrimSpace(pkg.HashValue) == "" || packageHashIsEmptyFile(pkg.HashValue) {
		return false
	}
	// MD5/SHA-256 hashValue is a comparable checksum (see packageHasComparableChecksum).
	switch strings.ToUpper(strings.ReplaceAll(pkg.HashType, "-", "_")) {
	case "SHA256", "SHA_256", "MD5":
		return false
	default:
		// Empty hashType with a hashValue still counts — Jamf often uses SHA3 here.
		return true
	}
}

// packageUploadShouldWaitNotReady is true when the record looks mid-transfer
// (not READY, no comparable checksums yet, but a real hashValue present). Callers
// should wait with package_wait_ready instead of re-uploading. Empty checksums
// with no hashValue, or hashValue equal to SHA3-512 of an empty file, mean the
// upload stored no bytes — re-upload is appropriate.
func packageUploadShouldWaitNotReady(pkg *PackageRecord) bool {
	return pkg != nil &&
		!packageIsCloudReady(pkg) &&
		!packageHasComparableChecksum(pkg) &&
		packageHasPendingTransferEvidence(pkg)
}

// packageUploadWaitOnly is true when upload should skip the file transfer and
// only poll until READY: --wait-ready on a mid-transfer record (hashValue set,
// sha256 empty). Without WaitReady that same record is an error (do not re-upload).
func packageUploadWaitOnly(upOpts PackageUploadOptions, pkg *PackageRecord) bool {
	return upOpts.WaitReady && !upOpts.Force && packageUploadShouldWaitNotReady(pkg)
}

// Package issue classes returned by PackageScan / packages_scan.
const (
	PackageIssueFailedUpload  = "failed-upload"  // no file stored: no hashValue, or SHA3-512 of empty bytes, and no md5/sha256
	PackageIssuePending       = "pending"        // mid-transfer: real hashValue set, sha256 empty, not READY
	PackageIssueUnusableReady = "unusable-ready" // READY but sha256 still empty
)

// PackageScanHit is one package that matches an issue class.
type PackageScanHit struct {
	ID                  string `json:"id"`
	PackageName         string `json:"packageName"`
	FileName            string `json:"fileName"`
	CloudTransferStatus string `json:"cloudTransferStatus,omitempty"`
	Class               string `json:"class"`
	HashType            string `json:"hashType,omitempty"`
	HashValue           string `json:"hashValue,omitempty"`
	Sha256              string `json:"sha256,omitempty"`
	Md5                 string `json:"md5,omitempty"`
}

// packageIssueClass returns a scan class for pkg, or "" if it looks healthy.
func packageIssueClass(pkg *PackageRecord) string {
	if pkg == nil {
		return ""
	}
	// READY without sha256 is its own class (even when hashValue is also missing).
	if packageIsCloudReady(pkg) && strings.TrimSpace(pkg.Sha256) == "" {
		return PackageIssueUnusableReady
	}
	if packageUploadShouldWaitNotReady(pkg) {
		return PackageIssuePending
	}
	if !packageHasComparableChecksum(pkg) && !packageHasPendingTransferEvidence(pkg) {
		return PackageIssueFailedUpload
	}
	return ""
}

// ParsePackageScanClasses parses a class filter. Empty or "all" → all issue classes.
// Accepts a single class or comma-separated list.
func ParsePackageScanClasses(spec string) ([]string, error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" || spec == "all" {
		return []string{PackageIssueFailedUpload, PackageIssuePending, PackageIssueUnusableReady}, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case PackageIssueFailedUpload, PackageIssuePending, PackageIssueUnusableReady:
			if !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		default:
			return nil, fmt.Errorf("unknown package scan class %q (use failed-upload, pending, unusable-ready, or all)", part)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no package scan classes specified")
	}
	return out, nil
}

// PackageScan lists packages (optional RSQL/bare filter) and returns those whose
// issue class is in classes. classes must be non-empty (see ParsePackageScanClasses).
// Uses the collection list response (checksums/status are included there).
func PackageScan(credName, filter string, classes []string, logger *slog.Logger) ([]PackageScanHit, error) {
	want := map[string]bool{}
	for _, c := range classes {
		want[c] = true
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("package scan: no classes selected")
	}
	pkgs, err := PackageSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return nil, err
	}
	var hits []PackageScanHit
	for i := range pkgs {
		pkg := &pkgs[i]
		class := packageIssueClass(pkg)
		if class == "" || !want[class] {
			continue
		}
		hits = append(hits, PackageScanHit{
			ID:                  pkg.ID,
			PackageName:         pkg.PackageName,
			FileName:            pkg.FileName,
			CloudTransferStatus: pkg.CloudTransferStatus,
			Class:               class,
			HashType:            pkg.HashType,
			HashValue:           pkg.HashValue,
			Sha256:              pkg.Sha256,
			Md5:                 pkg.Md5,
		})
	}
	return hits, nil
}

// PackageScanPrint runs PackageScan and prints a table.
func PackageScanPrint(credName, filter, classSpec string, logger *slog.Logger) error {
	classes, err := ParsePackageScanClasses(classSpec)
	if err != nil {
		return err
	}
	hits, err := PackageScan(credName, filter, classes, logger)
	if err != nil {
		return err
	}
	fmt.Printf("%-8s  %-16s  %-8s  %-36s  %s\n", "ID", "CLASS", "STATUS", "Package Name", "File Name")
	fmt.Printf("%-8s  %-16s  %-8s  %-36s  %s\n", "--------", "----------------", "--------", "------------------------------------", "--------------------")
	for _, h := range hits {
		status := h.CloudTransferStatus
		if status == "" {
			status = "(none)"
		}
		fmt.Printf("%-8s  %-16s  %-8s  %-36s  %s\n", h.ID, h.Class, truncatePkg(status, 8), truncatePkg(h.PackageName, 36), h.FileName)
	}
	fmt.Printf("\n%d issue(s)\n", len(hits))
	return nil
}

// packageIsUsable is READY with a non-empty sha256 (JCDS inventory settled).
func packageIsUsable(pkg *PackageRecord) bool {
	return packageIsCloudReady(pkg) && strings.TrimSpace(pkg.Sha256) != ""
}

// PackageReady reports whether the package exists, cloudTransferStatus is READY,
// and sha256 is populated. A missing package (HTTP 404) returns false, nil — not an error.
func PackageReady(credName, id string) (bool, error) {
	pkg, status, err := packageGetStatus(credName, id)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound || pkg == nil {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("get package %s status %d", id, status)
	}
	return packageIsUsable(pkg), nil
}

// PackageWaitReady polls until the package exists, cloudTransferStatus is READY,
// and sha256 is non-empty. timeout ≤ 0 uses the default (10 minutes). Optionally
// nudges JCDS inventory using the package fileName between polls.
func PackageWaitReady(credName, id string, timeout time.Duration, logger *slog.Logger) error {
	_, err := packageWaitUntil(credName, id, timeout, true, logger, nil)
	return err
}

// errPackageWaitInterrupted is returned when the post-upload READY poll is stopped
// with Ctrl-C. maybeWaitPackageReady turns it into success; the upload already finished.
// PackageWaitReady (buddy package wait-ready) does not convert this sentinel.
var errPackageWaitInterrupted = errors.New("package wait interrupted")

func packageWaitInterruptOK(err error) bool {
	return errors.Is(err, errPackageWaitInterrupted)
}

// packageWaitUntil polls GET /api/v1/packages/{id} until cloudTransferStatus is
// READY (and optionally sha256 is non-empty). Returns the last successful record.
// A closed or received interrupt stops the poll with errPackageWaitInterrupted.
// interrupt may be nil (buddy package wait-ready).
func packageWaitUntil(credName, id string, timeout time.Duration, requireSHA256 bool, logger *slog.Logger, interrupt <-chan struct{}) (*PackageRecord, error) {
	if timeout <= 0 {
		timeout = packageWaitReadyDefaultTimeout
	}
	start := time.Now()
	deadline := start.Add(timeout)
	backoff := packageWaitInitialBackoff
	var lastAnnounce time.Time
	var last *PackageRecord
	gets := 0
	for {
		gets++
		pkg, status, err := packageGetStatus(credName, id)
		if err != nil {
			return nil, err
		}
		if status != http.StatusNotFound && status != http.StatusOK {
			return nil, fmt.Errorf("get package %s status %d", id, status)
		}
		if status == http.StatusOK && pkg != nil {
			last = pkg
			ready := packageIsCloudReady(pkg)
			if ready && requireSHA256 && strings.TrimSpace(pkg.Sha256) == "" {
				ready = false
			}
			if ready {
				_ = logger
				return pkg, nil
			}
			if pkg.FileName != "" {
				_ = jcdsRefreshInventory(credName, pkg.FileName)
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			want := "cloudTransferStatus READY"
			if requireSHA256 {
				want += " and sha256"
			}
			statusNote := "missing"
			if last != nil {
				statusNote = fmt.Sprintf("cloudTransferStatus=%q", last.CloudTransferStatus)
				if requireSHA256 {
					sha := strings.TrimSpace(last.Sha256)
					if sha == "" {
						statusNote += " sha256=(none)"
					} else {
						statusNote += fmt.Sprintf(" sha256=%q", sha)
					}
				}
			}
			return nil, fmt.Errorf("package %s not %s within %s (%d polls, %s; last: %s)",
				id, want, timeout, gets, formatUploadDuration(time.Since(start)), statusNote)
		}
		now := time.Now()
		if lastAnnounce.IsZero() || now.Sub(lastAnnounce) >= packageWaitAnnounceEvery {
			msg := fmt.Sprintf("Waiting for package %s cloudTransferStatus=READY", id)
			if requireSHA256 {
				msg += " and sha256"
			}
			if last != nil {
				parts := []string{}
				if last.CloudTransferStatus != "" {
					parts = append(parts, fmt.Sprintf("status=%q", last.CloudTransferStatus))
				} else {
					parts = append(parts, `status=""`)
				}
				if requireSHA256 {
					if strings.TrimSpace(last.Sha256) == "" {
						parts = append(parts, "sha256=(none)")
					} else {
						parts = append(parts, "sha256=set")
					}
				}
				msg += " (" + strings.Join(parts, " ") + ")"
			}
			fmt.Fprintf(os.Stderr, "%s…\n", msg)
			lastAnnounce = now
		}
		d := backoff
		if d > remaining {
			d = remaining
		}
		if err := sleepOrInterrupt(d, interrupt); err != nil {
			return last, err
		}
		backoff *= 2
		if backoff > packageWaitMaxBackoff {
			backoff = packageWaitMaxBackoff
		}
	}
}

// sleepOrInterrupt waits for d, or returns errPackageWaitInterrupted when interrupt fires.
// A nil interrupt sleeps without watching for a signal.
func sleepOrInterrupt(d time.Duration, interrupt <-chan struct{}) error {
	if interrupt == nil {
		time.Sleep(d)
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-interrupt:
		return errPackageWaitInterrupted
	}
}

// jcdsRefreshInventory POSTs /api/v1/jcds/refresh-inventory?file-name=… to nudge
// JCDS availability checks. Failures are ignored by the caller (best-effort).
func jcdsRefreshInventory(credName, fileName string) error {
	q := url.Values{}
	q.Set("file-name", fileName)
	endpoint, err := ResolveEndpoint(credName, "/api/v1/jcds/refresh-inventory?"+q.Encode())
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jcds refresh-inventory status %d", resp.StatusCode)
	}
	return nil
}

type jcdsDownloadURI struct {
	URI string `json:"uri"`
}

type jcdsHTTPStatusError struct {
	StatusCode int
}

func (e *jcdsHTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// jcdsFetchPresignedURI GETs /api/v1/jcds/files/{fileName} and returns the uri.
func jcdsFetchPresignedURI(credName, fileName string) (string, error) {
	endpoint, err := ResolveEndpoint(credName, "/api/v1/jcds/files/"+url.PathEscape(fileName))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return "", fmt.Errorf("get jcds file %q: %w", fileName, err)
	}
	defer resp.Body.Close()
	body, err := readHTTPBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get jcds file %q status %d: %s", fileName, resp.StatusCode, string(body))
	}
	var result jcdsDownloadURI
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse jcds download URI: %w", err)
	}
	if result.URI == "" {
		return "", fmt.Errorf("jcds returned empty download URI for %q", fileName)
	}
	return result.URI, nil
}

// PackageDownload waits until the package is READY with a sha256, fetches a JCDS
// presigned URI, streams to a temp file while hashing SHA-256, verifies against
// the package record, and renames into place only on match. Retries once on URI
// HTTP 403 (presigned TTL). JCDS only — not for on-prem non-JCDS distribution points.
// outfile empty defaults to the package fileName in the current directory.
func PackageDownload(credName, id, outfile string, logger *slog.Logger) error {
	pkg, err := packageWaitUntil(credName, id, 0, true, logger, nil)
	if err != nil {
		return err
	}
	if pkg.FileName == "" {
		return fmt.Errorf("package %s has empty fileName", id)
	}
	wantSHA := strings.TrimSpace(pkg.Sha256)
	if wantSHA == "" {
		return fmt.Errorf("package %s has empty sha256 after wait", id)
	}
	if outfile == "" {
		outfile = filepath.Base(pkg.FileName)
	}
	outfile, err = filepath.Abs(outfile)
	if err != nil {
		return fmt.Errorf("resolve outfile: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outfile), 0o755); err != nil {
		return fmt.Errorf("create outfile directory: %w", err)
	}

	uri, err := jcdsFetchPresignedURI(credName, pkg.FileName)
	if err != nil {
		return err
	}
	n, gotSHA, err := jcdsStreamVerifyToFile(uri, outfile, wantSHA, logger)
	var httpErr *jcdsHTTPStatusError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusForbidden {
		fmt.Fprintf(os.Stderr, "warning: pre-signed URL expired (403), retrying with fresh URL\n")
		uri, err = jcdsFetchPresignedURI(credName, pkg.FileName)
		if err != nil {
			return err
		}
		n, gotSHA, err = jcdsStreamVerifyToFile(uri, outfile, wantSHA, logger)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded package %s (%s) → %s (%s, sha256 ok)\n", id, pkg.FileName, outfile, formatBytes(n))
	_ = gotSHA
	_ = logger
	return nil
}

// jcdsStreamVerifyToFile downloads uri to a temp file beside outfile while hashing
// SHA-256. Renames to outfile only when the hex digest matches wantSHA (case-insensitive).
// Deletes the temp file on any failure. Non-2xx responses return *jcdsHTTPStatusError.
func jcdsStreamVerifyToFile(uri, outfile, wantSHA string, logger *slog.Logger) (n int64, gotSHA string, err error) {
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return 0, "", fmt.Errorf("create download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return 0, "", &jcdsHTTPStatusError{StatusCode: resp.StatusCode}
	}

	dir := filepath.Dir(outfile)
	tmp, err := os.CreateTemp(dir, ".buddy-pkg-*.tmp")
	if err != nil {
		return 0, "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	var body io.Reader = io.TeeReader(resp.Body, h)
	total := resp.ContentLength
	if total >= minProgressSize {
		var lastWrite time.Time
		pr := &progressReader{
			r:     body,
			total: total,
			onProgress: func(read, tot int64) {
				now := time.Now()
				if lastWrite.IsZero() || now.Sub(lastWrite) >= 200*time.Millisecond || read == tot {
					fmt.Fprintf(os.Stderr, "\r%s", formatProgressBar("Downloading", read, tot))
					lastWrite = now
				}
			},
		}
		body = pr
	}

	buf := make([]byte, 4<<20)
	n, err = io.CopyBuffer(tmp, body, buf)
	if total >= minProgressSize {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return 0, "", fmt.Errorf("write download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, "", fmt.Errorf("close temp file: %w", err)
	}
	gotSHA = hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(gotSHA, wantSHA) {
		return n, gotSHA, fmt.Errorf("sha256 mismatch: got %s, want %s", gotSHA, wantSHA)
	}
	if err := os.Rename(tmpName, outfile); err != nil {
		return n, gotSHA, fmt.Errorf("rename into place: %w", err)
	}
	cleanup = false
	_ = logger
	return n, gotSHA, nil
}

// packagePut sends a Package object to PUT /api/v1/packages/{id}.
func packagePut(credName, id string, pkg *PackageRecord) (*PackageRecord, error) {
	// Strip fields Jamf manages or that we only inject for display. Sending
	// size on PUT can blank the server-managed value.
	toSend := *pkg
	toSend.Size = ""
	toSend.CloudTransferStatus = ""
	toSend.LastChanged = ""
	toSend.LastChangedBy = ""

	data, err := json.Marshal(&toSend)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	body, err := jamfPUT(credName, uriPackages+"/"+id, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("update package %s: %w", id, err)
	}
	var updated PackageRecord
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
func PackagePutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*PackageRecord, error) {
	pkg, err := packageGet(credName, id)
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

// packageCreatePayload is the payload for POST /api/v1/packages (create).
type packageCreatePayload struct {
	PackageName          string `json:"packageName"`
	FileName             string `json:"fileName"`
	CategoryID           string `json:"categoryId"`
	Priority             int    `json:"priority"`
	Notes                string `json:"notes,omitempty"`
	Info                 string `json:"info,omitempty"`
	FillUserTemplate     bool   `json:"fillUserTemplate"`
	RebootRequired       bool   `json:"rebootRequired"`
	OsInstall            bool   `json:"osInstall"`
	SuppressUpdates      bool   `json:"suppressUpdates"`
	SuppressFromDock     bool   `json:"suppressFromDock"`
	SuppressEula         bool   `json:"suppressEula"`
	SuppressRegistration bool   `json:"suppressRegistration"`
}

// PackageAttrs are optional package metadata fields shared by upload and set.
// A nil pointer means "leave unchanged" (set) or "use default" (upload create).
type PackageAttrs struct {
	Name       *string
	CategoryID *string
	Priority   *int
	Notes      *string
	Info       *string
	Reboot     *bool
}

// AnySet reports whether at least one attribute was explicitly provided.
func (a PackageAttrs) AnySet() bool {
	return a.Name != nil || a.CategoryID != nil || a.Priority != nil ||
		a.Notes != nil || a.Info != nil || a.Reboot != nil
}

// Validate checks priority range when Priority is set.
func (a PackageAttrs) Validate() error {
	if a.Priority != nil && (*a.Priority < 1 || *a.Priority > 20) {
		return fmt.Errorf("priority must be between 1 and 20, got %d", *a.Priority)
	}
	return nil
}

// Apply writes set attributes onto pkg.
func (a PackageAttrs) Apply(pkg *PackageRecord) {
	if pkg == nil {
		return
	}
	if a.Name != nil {
		pkg.PackageName = *a.Name
	}
	if a.CategoryID != nil {
		pkg.CategoryID = *a.CategoryID
	}
	if a.Priority != nil {
		pkg.Priority = *a.Priority
	}
	if a.Notes != nil {
		pkg.Notes = *a.Notes
	}
	if a.Info != nil {
		pkg.Info = *a.Info
	}
	if a.Reboot != nil {
		pkg.RebootRequired = *a.Reboot
	}
}

// DiffersFrom reports whether applying attrs would change pkg.
func (a PackageAttrs) DiffersFrom(pkg *PackageRecord) bool {
	if pkg == nil {
		return a.AnySet()
	}
	if a.Name != nil && *a.Name != pkg.PackageName {
		return true
	}
	if a.CategoryID != nil && *a.CategoryID != pkg.CategoryID {
		return true
	}
	if a.Priority != nil && *a.Priority != pkg.Priority {
		return true
	}
	if a.Notes != nil && *a.Notes != pkg.Notes {
		return true
	}
	if a.Info != nil && *a.Info != pkg.Info {
		return true
	}
	if a.Reboot != nil && *a.Reboot != pkg.RebootRequired {
		return true
	}
	return false
}

func (a PackageAttrs) summaryLines(pkg *PackageRecord) []string {
	var lines []string
	if a.Name != nil {
		lines = append(lines, fmt.Sprintf("name=%q", *a.Name))
	}
	if a.CategoryID != nil {
		lines = append(lines, fmt.Sprintf("categoryId=%s", *a.CategoryID))
	}
	if a.Priority != nil {
		lines = append(lines, fmt.Sprintf("priority=%d", *a.Priority))
	}
	if a.Notes != nil {
		lines = append(lines, "notes=(set)")
	}
	if a.Info != nil {
		lines = append(lines, "info=(set)")
	}
	if a.Reboot != nil {
		lines = append(lines, fmt.Sprintf("rebootRequired=%v", *a.Reboot))
	}
	_ = pkg
	return lines
}

// packageCreate POSTs a new package record and returns the created package ID.
func packageCreate(credName string, payload *packageCreatePayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal create payload: %w", err)
	}
	body, err := jamfPOST(credName, uriPackages, data, "application/json")
	if err != nil {
		return "", fmt.Errorf("create package: %w", err)
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

// packageProbeReadable GETs /api/v1/packages/{id}.
// Returns (true, nil) when the record is readable (HTTP 200),
// (false, nil) when Jamf has not propagated it yet (HTTP 404),
// or an error for other failures.
func packageProbeReadable(credName, id string) (bool, error) {
	endpoint, err := ResolveEndpoint(credName, uriPackages+"/"+id)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return false, fmt.Errorf("get package %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, err := readHTTPBody(resp)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("get package %s status %d: %s", id, resp.StatusCode, string(body))
	}
}

const (
	packageWaitTimeout        = 2 * time.Minute
	packageWaitInitialBackoff = 500 * time.Millisecond
	packageWaitMaxBackoff     = 3 * time.Second
	packageWaitAnnounceEvery  = 3 * time.Second
	// packageUploadMaxAttempts is the first POST plus retries after 404/500/502/503,
	// a connection reset, or a 201 that left the empty-file hash and no size.
	// Pauses apply only between attempts.
	packageUploadMaxAttempts = 6
	// packageUploadStoredConfirmTimeout is how long to poll after a 201 before
	// treating “SHA3-512 of empty bytes, no size” as a failed upload.
	packageUploadStoredConfirmTimeout = 15 * time.Second
)

// packageEmptyFileSHA3 is SHA3-512 of zero bytes. Jamf sets hashValue to this
// (with no size and no cloudTransferStatus) when the upload POST is accepted
// but no file bytes are stored.
const packageEmptyFileSHA3 = "a69f73cca23a9ac5c8b567dc185a756e97c982164fe25859e0d1dcc1475c80a615b2123af1f5f94c11e3e9402c3ac558f500199d95b6d3e301758586281dcd26"

// errPackageUploadNotStored means POST .../upload returned success and the
// package record still has no file.
var errPackageUploadNotStored = errors.New("upload accepted but Jamf stored no file")

// packageUploadRetryPauses are the waits after a failed upload attempt (1+2+4+8+16).
var packageUploadRetryPauses = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
}

// packageWaitReadable polls until GET /api/v1/packages/{id} succeeds.
// Jamf Cloud create is often async: POST can return an ID before the record is readable.
// Returns the number of GET probes and wall time until the first successful read.
func packageWaitReadable(credName, id string) (gets int, elapsed time.Duration, err error) {
	return waitUntilPackageReadable(id, func() (bool, error) {
		return packageProbeReadable(credName, id)
	}, packageWaitTimeout, packageWaitInitialBackoff, packageWaitMaxBackoff, time.Sleep)
}

// waitUntilPackageReadable runs probe until ready, deadline exceeded, or probe error.
// sleep is injectable for tests. gets counts every probe call (including the successful one).
func waitUntilPackageReadable(
	id string,
	probe func() (bool, error),
	timeout, initialBackoff, maxBackoff time.Duration,
	sleep func(time.Duration),
) (gets int, elapsed time.Duration, err error) {
	start := time.Now()
	deadline := start.Add(timeout)
	backoff := initialBackoff
	var lastAnnounce time.Time
	for {
		gets++
		ready, err := probe()
		if err != nil {
			return gets, time.Since(start), err
		}
		if ready {
			return gets, time.Since(start), nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return gets, time.Since(start), fmt.Errorf("package record %s not readable within %s (Jamf create may still be propagating)", id, timeout)
		}
		now := time.Now()
		if lastAnnounce.IsZero() || now.Sub(lastAnnounce) >= packageWaitAnnounceEvery {
			fmt.Fprintf(os.Stderr, "Waiting for package record %s…\n", id)
			lastAnnounce = now
		}
		d := backoff
		if d > remaining {
			d = remaining
		}
		sleep(d)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// confirmPackageUploadStored polls until the record shows a stored file.
// A 201 alone is not enough: Jamf can accept the POST and leave hashValue as
// SHA3-512 of an empty file, with no size. That error is retryable.
func confirmPackageUploadStored(credName, id string) error {
	var announced bool
	return waitUntilPackageFileStored(id, func() (bool, error) {
		pkg, status, err := packageGetStatus(credName, id)
		if err != nil {
			return false, err
		}
		if status == http.StatusOK && packageRecordHasFile(pkg) {
			return true, nil
		}
		if !announced {
			fmt.Fprintf(os.Stderr, "Upload accepted; Jamf has not stored the file yet…\n")
			announced = true
		}
		return false, nil
	}, packageUploadStoredConfirmTimeout, packageWaitInitialBackoff, packageWaitMaxBackoff, time.Sleep)
}

// waitUntilPackageFileStored runs probe until it reports a stored file, the
// deadline passes, or probe returns an error. sleep is injectable for tests.
func waitUntilPackageFileStored(
	id string,
	probe func() (bool, error),
	timeout, initialBackoff, maxBackoff time.Duration,
	sleep func(time.Duration),
) error {
	deadline := time.Now().Add(timeout)
	backoff := initialBackoff
	for {
		ok, err := probe()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("package %s: %w", id, errPackageUploadNotStored)
		}
		d := backoff
		if d > remaining {
			d = remaining
		}
		if sleep != nil {
			sleep(d)
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// progressReader wraps io.Reader and invokes onProgress as bytes are read.
type progressReader struct {
	r          io.Reader
	total      int64
	read       int64
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

// formatProgressBar returns a line like "Uploading: [####----] 75% (1.1 GB / 1.5 GB)".
// label may be empty for an unlabeled bar.
func formatProgressBar(label string, read, total int64) string {
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
	body := fmt.Sprintf("[%s] %.0f%% (%s / %s)", bar, pct*100, formatBytes(read), formatBytes(total))
	if label == "" {
		return body
	}
	return label + ": " + body
}

// minProgressSize is the minimum file size (bytes) to show progress bar.
const minProgressSize = 100 * 1024 // 100 KB

// errPackageUploadNotFound is returned when POST .../upload gets HTTP 404
// (package record not yet visible — Jamf Cloud create race).
var errPackageUploadNotFound = errors.New("package upload target not found")

func isPackageUploadNotFound(err error) bool {
	return errors.Is(err, errPackageUploadNotFound)
}

// packageUploadStatusError is an HTTP status from POST .../upload.
// 404 unwraps to errPackageUploadNotFound.
type packageUploadStatusError struct {
	Code int
	Body string
}

func (e *packageUploadStatusError) Error() string {
	if e == nil {
		return "upload package file status"
	}
	msg := strings.TrimSpace(e.Body)
	if msg == "" {
		msg = "(empty response body)"
	}
	return fmt.Sprintf("upload package file status %d: %s", e.Code, msg)
}

func (e *packageUploadStatusError) Unwrap() error {
	if e != nil && e.Code == http.StatusNotFound {
		return errPackageUploadNotFound
	}
	return nil
}

// isPackageUploadRetryable reports failures that mean the package record exists
// but JCDS was not ready for the file POST yet. It does not cover 401 (token
// refresh) or a local file read error.
func isPackageUploadRetryable(err error) bool {
	if err == nil || isLocalFileReadError(err) {
		return false
	}
	var st *packageUploadStatusError
	if errors.As(err, &st) {
		switch st.Code {
		case http.StatusNotFound, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
			return true
		default:
			return false
		}
	}
	if isPackageUploadNotFound(err) || errors.Is(err, errPackageUploadNotStored) {
		return true
	}
	return isRetryableUploadTransport(err)
}

func isLocalFileReadError(err error) bool {
	var pe *os.PathError
	return errors.As(err, &pe) && pe.Op == "read"
}

func isRetryableUploadTransport(err error) bool {
	if err == nil || isLocalFileReadError(err) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNABORTED) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") || strings.Contains(msg, "unexpected eof") || strings.Contains(msg, "broken pipe")
}

func uploadRetryReason(err error) string {
	var st *packageUploadStatusError
	if errors.As(err, &st) {
		return fmt.Sprintf("%d", st.Code)
	}
	if isPackageUploadNotFound(err) {
		return "404"
	}
	if errors.Is(err, errPackageUploadNotStored) {
		return "no file stored"
	}
	return "connection error"
}

// packageHashIsEmptyFile reports whether hash is SHA3-512 of zero bytes.
func packageHashIsEmptyFile(hash string) bool {
	return strings.EqualFold(strings.TrimSpace(hash), packageEmptyFileSHA3)
}

// packageRecordHasFile reports whether the record shows Jamf stored file bytes.
// A real hashValue, a non-zero size, or md5/sha256 counts. SHA3-512 of an empty
// file with no size does not.
func packageRecordHasFile(pkg *PackageRecord) bool {
	if pkg == nil {
		return false
	}
	if packageHasComparableChecksum(pkg) {
		return true
	}
	size := strings.TrimSpace(pkg.Size)
	if size != "" && size != "0" {
		return true
	}
	hash := strings.TrimSpace(pkg.HashValue)
	return hash != "" && !packageHashIsEmptyFile(hash)
}

// multipartFormFileLength returns Content-Type and Content-Length for a
// multipart body that contains a single form file of fileSize bytes.
// boundary must match the writer used to stream the real body.
func multipartFormFileLength(boundary, filename string, fileSize int64) (contentType string, contentLength int64, err error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.SetBoundary(boundary); err != nil {
		return "", 0, err
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", 0, err
	}
	_ = part // empty probe body
	if err := w.Close(); err != nil {
		return "", 0, err
	}
	return w.FormDataContentType(), int64(buf.Len()) + fileSize, nil
}

// formatUploadDuration returns a compact duration for upload stats (e.g. "320ms", "12.3s").
func formatUploadDuration(d time.Duration) string {
	if d < time.Second {
		ms := d.Milliseconds()
		if ms < 1 && d > 0 {
			ms = 1
		}
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// formatPackageGETCount returns "1 GET" or "N GETs".
func formatPackageGETCount(n int) string {
	if n == 1 {
		return "1 GET"
	}
	return fmt.Sprintf("%d GETs", n)
}

// formatUploadRate returns a human transfer rate like "45.2 MB/s".
func formatUploadRate(n int64, d time.Duration) string {
	sec := d.Seconds()
	if sec <= 0 {
		return formatBytes(n) + "/s"
	}
	return formatBytes(int64(float64(n)/sec)) + "/s"
}

// uploadStreamBody closes the pipe when the HTTP client finishes reading, so a
// mid-upload failure unblocks the writer goroutine.
type uploadStreamBody struct {
	io.Reader
	close func() error
}

func (b uploadStreamBody) Close() error {
	if b.close == nil {
		return nil
	}
	return b.close()
}

// packageUploadFile streams the file as multipart/form-data to /api/v1/packages/{id}/upload.
// Progress tracks bytes the HTTP client reads to send (the body is not buffered first).
// On success it returns the file byte count and wall time of the HTTP transfer.
// A 401 refreshes the token once and resends; that retry is not a JCDS attempt.
func packageUploadFile(credName, id string, file *os.File, opts Opts) (uploaded int64, elapsed time.Duration, err error) {
	_ = opts
	info, err := file.Stat()
	if err != nil {
		return 0, 0, fmt.Errorf("stat file: %w", err)
	}
	fileSize := info.Size()
	filename := path.Base(file.Name())
	uploadPath := uriPackages + "/" + id + "/upload"
	endpoint, err := ResolveEndpoint(credName, uploadPath)
	if err != nil {
		return 0, 0, err
	}
	op := opClassForPath(http.MethodPost, uploadPath)

	for authTry := 0; authTry < 2; authTry++ {
		resp, elapsed, err := postPackageUpload(endpoint, op, credName, file, filename, fileSize)
		if err != nil {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			return 0, elapsed, err
		}
		if resp.StatusCode == http.StatusUnauthorized && authTry == 0 {
			refreshed, rerr := refreshAuthAfter401(credName, op)
			if rerr != nil {
				resp.Body.Close()
				return 0, elapsed, rerr
			}
			if refreshed {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				fmt.Fprintf(os.Stderr, "Auth expired; refreshing and retrying upload…\n")
				continue
			}
		}
		statusErr := packageUploadStatusFromResponse(resp)
		resp.Body.Close()
		if statusErr != nil {
			return 0, elapsed, statusErr
		}
		if err := confirmPackageUploadStored(credName, id); err != nil {
			return 0, elapsed, err
		}
		return fileSize, elapsed, nil
	}
	return 0, 0, fmt.Errorf("upload package file: auth refresh did not complete")
}

func packageUploadStatusFromResponse(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("upload package file: empty response")
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return &packageUploadStatusError{Code: resp.StatusCode, Body: strings.TrimSpace(httpErrorBody(resp))}
}

// postPackageUpload sends one multipart POST. The body is streamed; progress counts
// bytes as the transport reads them. resp may be nil when err is a transport failure.
func postPackageUpload(endpoint string, op OpClass, credName string, file *os.File, filename string, fileSize int64) (resp *http.Response, elapsed time.Duration, err error) {
	if _, err := file.Seek(0, 0); err != nil {
		return nil, 0, fmt.Errorf("seek file for upload: %w", err)
	}
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	contentType, contentLength, err := multipartFormFileLength(w.Boundary(), filename, fileSize)
	if err != nil {
		_ = pr.Close()
		return nil, 0, fmt.Errorf("multipart length: %w", err)
	}

	writeErrCh := make(chan error, 1)
	go func() {
		defer close(writeErrCh)
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(err)
			writeErrCh <- err
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			writeErrCh <- err
			return
		}
		if err := w.Close(); err != nil {
			_ = pw.CloseWithError(err)
			writeErrCh <- err
			return
		}
		_ = pw.Close()
		writeErrCh <- nil
	}()

	var body io.Reader = pr
	showProgress := fileSize >= minProgressSize && fileSize > 0
	var waitingShown atomic.Bool
	if showProgress {
		fmt.Fprintf(os.Stderr, "Uploading %s (%s)…\n", filename, formatBytes(fileSize))
		var lastWrite time.Time
		body = &progressReader{
			r:     pr,
			total: contentLength,
			onProgress: func(read, tot int64) {
				now := time.Now()
				if !(lastWrite.IsZero() || now.Sub(lastWrite) >= 200*time.Millisecond || read >= tot) {
					return
				}
				lastWrite = now
				if read >= tot {
					if waitingShown.CompareAndSwap(false, true) {
						fmt.Fprintf(os.Stderr, "\r%s\nWaiting for server response…\n", formatProgressBar("Uploading", tot, tot))
					}
					return
				}
				fmt.Fprintf(os.Stderr, "\r%s", formatProgressBar("Uploading", read, tot))
			},
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, uploadStreamBody{Reader: body, close: pr.Close})
	if err != nil {
		_ = pr.Close()
		<-writeErrCh
		return nil, 0, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = contentLength

	start := time.Now()
	resp, err = doAuthedStream(credName, op, req)
	elapsed = time.Since(start)
	if err != nil && req.Body != nil {
		// Auth can fail before the body is read. Close the pipe so the writer
		// goroutine is not stuck on a full buffer.
		_ = req.Body.Close()
	}
	writeErr := <-writeErrCh
	if err != nil {
		if showProgress && !waitingShown.Load() {
			fmt.Fprintln(os.Stderr)
		}
		if isLocalFileReadError(writeErr) {
			return nil, elapsed, fmt.Errorf("read package file: %w", writeErr)
		}
		return nil, elapsed, fmt.Errorf("upload package file: %w", err)
	}
	if showProgress && !waitingShown.Load() {
		fmt.Fprintf(os.Stderr, "\r%s\n", formatProgressBar("Uploading", contentLength, contentLength))
	}
	if isLocalFileReadError(writeErr) {
		return resp, elapsed, fmt.Errorf("read package file: %w", writeErr)
	}
	return resp, elapsed, nil
}

// PackageUploadOptions controls optional post-upload behavior.
type PackageUploadOptions struct {
	// WaitReady polls until cloudTransferStatus is READY and sha256 is set.
	WaitReady bool
	// WaitReadyTimeout caps the wait; ≤0 uses the PackageWaitReady default (10 minutes).
	WaitReadyTimeout time.Duration
	// Force re-uploads even when checksums match or the record looks mid-transfer
	// (overrides skip-if-unchanged and the not-ready wait guard).
	Force bool
}

// PackageUpload creates a new package record and uploads the file to Jamf Pro.
// If a package with the same fileName already exists, the file is treated as an
// update: the existing record is fetched, summarised, and the upload is skipped
// when a server checksum matches the local file. attrs set metadata on create
// and are applied to an existing record when they differ (even if the file is skipped).
// When upOpts.WaitReady is set, polls until the package is READY (and sha256 set)
// after a successful upload, skip-if-unchanged, or a mid-transfer record that
// already has the file (a real hashValue, sha256 empty) — that case does not re-upload.
// SHA3-512 of an empty file is not mid-transfer; that record is uploaded again.
// On create, waits until GET /packages/{id} succeeds, then streams the multipart
// upload. A 404/500/502/503, a connection reset, or a 201 that leaves the empty-file
// hash and no size retries the whole POST with pauses 1s, 2s, 4s, 8s, 16s (six
// attempts). That retry does not depend on WaitReady.
func PackageUpload(credName string, opts Opts, filePath string, attrs PackageAttrs, upOpts PackageUploadOptions, logger *slog.Logger) error {
	if err := attrs.Validate(); err != nil {
		return err
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
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	packageName := fileName
	if attrs.Name != nil && *attrs.Name != "" {
		packageName = *attrs.Name
	}
	categoryID := "-1"
	if attrs.CategoryID != nil && *attrs.CategoryID != "" {
		categoryID = *attrs.CategoryID
	}
	priority := 10
	if attrs.Priority != nil {
		priority = *attrs.Priority
	}
	reboot := false
	if attrs.Reboot != nil {
		reboot = *attrs.Reboot
	}
	notes := ""
	if attrs.Notes != nil {
		notes = *attrs.Notes
	}
	infoStr := ""
	if attrs.Info != nil {
		infoStr = *attrs.Info
	}

	payload := &packageCreatePayload{
		PackageName:          packageName,
		FileName:             fileName,
		CategoryID:           categoryID,
		Priority:             priority,
		Notes:                notes,
		Info:                 infoStr,
		FillUserTemplate:     false,
		RebootRequired:       reboot,
		OsInstall:            false,
		SuppressUpdates:      false,
		SuppressFromDock:     false,
		SuppressEula:         false,
		SuppressRegistration: false,
	}

	fmt.Printf("Checking for existing package with fileName %q…\n", fileName)
	existingID, err := packageFindByFileName(credName, fileName, logger)
	if err != nil {
		return err
	}

	var id string
	created := false
	updating := false
	if existingID != "" {
		updating = true
		id = existingID
		doUpload, err := packageUploadExisting(credName, opts, file, info, id, attrs, upOpts, logger)
		if err != nil || !doUpload {
			return err
		}
	} else {
		id, err = packageUploadCreate(credName, fileName, payload)
		if err != nil {
			return err
		}
		created = true
	}

	if created {
		if err := packageReportReadable(credName, id); err != nil {
			return err
		}
	}
	uploaded, elapsed, err := packageUploadFileWithRetry(credName, id, file, opts)
	if err != nil {
		return err
	}
	action := "Uploaded"
	if updating {
		action = "Updated"
	}
	fmt.Printf("%s package %q (ID %s) — %s in %s (%s)\n",
		action, packageName, id, formatBytes(uploaded), formatUploadDuration(elapsed), formatUploadRate(uploaded, elapsed))
	return maybeWaitPackageReady(credName, id, upOpts, logger)
}

func packageUploadCreate(credName, fileName string, payload *packageCreatePayload) (string, error) {
	fmt.Printf("No existing package with fileName %q — create path\n", fileName)
	id, err := packageCreate(credName, payload)
	if err != nil {
		return "", err
	}
	fmt.Printf("Created package record (ID %s)\n", id)
	return id, nil
}

// packageUploadExisting handles the update path. doUpload is true when the caller
// should proceed to multipart upload.
func packageUploadExisting(credName string, opts Opts, file *os.File, info os.FileInfo, id string, attrs PackageAttrs, upOpts PackageUploadOptions, logger *slog.Logger) (doUpload bool, err error) {
	fmt.Printf("Found existing package record (ID %s) — update path\n", id)
	pkg, err := packageGet(credName, id)
	if err != nil {
		return false, err
	}
	printPackageRecordSummary(pkg)

	metaNeeded := attrs.AnySet() && attrs.DiffersFrom(pkg)
	if packageUploadWaitOnly(upOpts, pkg) {
		status := pkg.CloudTransferStatus
		if status == "" {
			status = "(empty)"
		}
		fmt.Printf("Package is not READY yet (cloudTransferStatus=%s) — skipping file upload, waiting until READY\n", status)
		if metaNeeded {
			fmt.Printf("Updating metadata: %s\n", strings.Join(attrs.summaryLines(pkg), ", "))
			ok, err := packageConfirmApplyAttrs(credName, id, opts, attrs, pkg, logger)
			if err != nil || !ok {
				return false, err
			}
		}
		return false, maybeWaitPackageReady(credName, id, upOpts, logger)
	}

	fmt.Printf("Hashing local file (%s)…\n", formatBytes(info.Size()))
	localHashes, err := hashPackageReader(file, info.Size())
	if err != nil {
		return false, fmt.Errorf("hash local file: %w", err)
	}
	fmt.Printf("  local md5:    %s\n", localHashes.MD5)
	fmt.Printf("  local sha256: %s\n", localHashes.SHA256)

	fileMatch, how := packageFileMatches(pkg, localHashes)

	if fileMatch && !upOpts.Force {
		if !metaNeeded {
			fmt.Printf("File unchanged (%s matches) — skipping upload\n", how)
			return false, maybeWaitPackageReady(credName, id, upOpts, logger)
		}
		fmt.Printf("File unchanged (%s matches) — updating metadata only\n", how)
		ok, err := packageConfirmApplyAttrs(credName, id, opts, attrs, pkg, logger)
		if err != nil || !ok {
			return false, err
		}
		return false, maybeWaitPackageReady(credName, id, upOpts, logger)
	}
	// Mid-transfer: sha256/md5 empty but a real hashValue is present (usually SHA3) → wait.
	// Failed upload: no hashValue, or SHA3-512 of an empty file → allow re-upload.
	if !upOpts.Force && packageUploadShouldWaitNotReady(pkg) {
		status := pkg.CloudTransferStatus
		if status == "" {
			status = "(empty)"
		}
		return false, fmt.Errorf("package %s (%q) is not ready yet (cloudTransferStatus=%s, sha256 empty, hashValue set); wait with package_wait_ready / 'buddy package wait-ready' before re-uploading or attaching (or pass --force / force=True to upload anyway)",
			id, pkg.PackageName, status)
	}
	switch {
	case upOpts.Force && fileMatch:
		fmt.Printf("Force: re-uploading even though %s matches\n", how)
	case upOpts.Force && packageUploadShouldWaitNotReady(pkg):
		fmt.Println("Force: re-uploading even though package looks mid-transfer (hashValue set, sha256 empty)")
	case !packageHasComparableChecksum(pkg) && !packageHasPendingTransferEvidence(pkg):
		fmt.Println("Package record has no checksums (previous upload may have failed) — file upload needed")
	default:
		fmt.Println("Local file differs from server checksums — file upload needed")
	}
	if metaNeeded {
		fmt.Printf("Metadata will also be updated: %s\n", strings.Join(attrs.summaryLines(pkg), ", "))
	}

	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nReplace file for package %s (%q)?", id, pkg.PackageName), logger) {
			return false, nil
		}
	}
	if metaNeeded {
		if err := packageApplyAttrs(credName, id, opts, attrs, pkg, logger); err != nil {
			return false, err
		}
	}
	return true, nil
}

func packageConfirmApplyAttrs(credName, id string, opts Opts, attrs PackageAttrs, pkg *PackageRecord, logger *slog.Logger) (ok bool, err error) {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpdate metadata for package %s (%q)?", id, pkg.PackageName), logger) {
			return false, nil
		}
	}
	if err := packageApplyAttrs(credName, id, opts, attrs, pkg, logger); err != nil {
		return false, err
	}
	return true, nil
}

func packageReportReadable(credName, id string) error {
	gets, waitElapsed, err := packageWaitReadable(credName, id)
	if err != nil {
		return err
	}
	fmt.Printf("Package record ready after %s in %s\n",
		formatPackageGETCount(gets), formatUploadDuration(waitElapsed))
	return nil
}

func packageUploadFileWithRetry(credName, id string, file *os.File, opts Opts) (int64, time.Duration, error) {
	return retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, time.Sleep, func() (int64, time.Duration, error) {
		return packageUploadFile(credName, id, file, opts)
	})
}

// retryPackageUpload runs upload until it succeeds, the error is not a JCDS race,
// or max attempts are used. Pauses are applied only after a retryable failure.
func retryPackageUpload(max int, pauses []time.Duration, sleep func(time.Duration), upload func() (int64, time.Duration, error)) (int64, time.Duration, error) {
	var last error
	for attempt := 1; attempt <= max; attempt++ {
		n, elapsed, err := upload()
		if err == nil {
			return n, elapsed, nil
		}
		last = err
		if !isPackageUploadRetryable(err) || attempt == max || attempt-1 >= len(pauses) {
			return 0, 0, err
		}
		pause := pauses[attempt-1]
		fmt.Fprintf(os.Stderr, "Upload returned %s; retrying in %s (attempt %d/%d)\n",
			uploadRetryReason(err), formatUploadDuration(pause), attempt+1, max)
		if sleep != nil {
			sleep(pause)
		}
	}
	return 0, 0, last
}

// maybeWaitPackageReady runs PackageWaitReady when upOpts.WaitReady is set.
// Ctrl-C during this wait stops the poll and returns success: the file transfer
// already finished. buddy package wait-ready does not use this path.
func maybeWaitPackageReady(credName, id string, upOpts PackageUploadOptions, logger *slog.Logger) error {
	if !upOpts.WaitReady || strings.TrimSpace(id) == "" {
		return nil
	}
	timeout := upOpts.WaitReadyTimeout
	if timeout <= 0 {
		timeout = packageWaitReadyDefaultTimeout
	}
	fmt.Printf("Waiting until package %s is READY (timeout %s)…\n", id, formatUploadDuration(timeout))
	fmt.Fprintf(os.Stderr, "Ctrl-C stops waiting; the file is already on Jamf.\n")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	done := make(chan struct{})
	defer close(done)
	interrupt := make(chan struct{})
	go func() {
		select {
		case <-sig:
			close(interrupt)
		case <-done:
		}
	}()

	if _, err := packageWaitUntil(credName, id, timeout, true, logger, interrupt); err != nil {
		if packageWaitInterruptOK(err) {
			fmt.Fprintf(os.Stderr, "Stopped waiting for READY (interrupted).\nPackage %s upload is complete; cloud availability may still be pending.\nResume with: buddy package wait-ready %s\n", id, id)
			return nil
		}
		return err
	}
	fmt.Printf("Package %s is READY\n", id)
	return nil
}

// packageFileHashes holds local checksums used to decide whether an upload is needed.
type packageFileHashes struct {
	MD5    string
	SHA256 string
}

// hashPackageReader computes MD5 and SHA-256 of r in a single pass.
// When totalSize >= minProgressSize, writes a labeled progress bar to stderr
// (reading the file for checksums — not the network upload).
func hashPackageReader(r io.Reader, totalSize int64) (packageFileHashes, error) {
	md5Hash := md5.New()
	sha256Hash := sha256.New()
	var src io.Reader = io.TeeReader(r, io.MultiWriter(md5Hash, sha256Hash))
	showProgress := totalSize >= minProgressSize && totalSize > 0
	if showProgress {
		var lastWrite time.Time
		src = &progressReader{
			r:     src,
			total: totalSize,
			onProgress: func(read, tot int64) {
				now := time.Now()
				if lastWrite.IsZero() || now.Sub(lastWrite) >= 200*time.Millisecond || read >= tot {
					fmt.Fprintf(os.Stderr, "\r%s", formatProgressBar("Hashing", read, tot))
					lastWrite = now
				}
			},
		}
	}
	if _, err := io.Copy(io.Discard, src); err != nil {
		if showProgress {
			fmt.Fprintln(os.Stderr)
		}
		return packageFileHashes{}, err
	}
	if showProgress {
		fmt.Fprintf(os.Stderr, "\r%s\n", formatProgressBar("Hashing", totalSize, totalSize))
	}
	return packageFileHashes{
		MD5:    hex.EncodeToString(md5Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
	}, nil
}

func hexEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// packageFileMatches reports whether the Jamf package record's checksums match local.
// Prefers sha256, then md5, then hashType/hashValue when those are MD5 or SHA-256.
// Returns false when the server has no comparable checksum yet — callers must also
// check packageUploadShouldWaitNotReady (blank hashes with a real hashValue while
// not READY means transfer pending; no hashValue, or SHA3-512 of an empty file,
// means a failed upload).
func packageFileMatches(pkg *PackageRecord, local packageFileHashes) (matched bool, field string) {
	if pkg == nil {
		return false, ""
	}
	if pkg.Sha256 != "" && hexEqualFold(pkg.Sha256, local.SHA256) {
		return true, "sha256"
	}
	if pkg.Md5 != "" && hexEqualFold(pkg.Md5, local.MD5) {
		return true, "md5"
	}
	if pkg.HashValue == "" {
		return false, ""
	}
	switch strings.ToUpper(strings.ReplaceAll(pkg.HashType, "-", "_")) {
	case "SHA256", "SHA_256":
		if hexEqualFold(pkg.HashValue, local.SHA256) {
			return true, "hashValue(SHA256)"
		}
	case "MD5":
		if hexEqualFold(pkg.HashValue, local.MD5) {
			return true, "hashValue(MD5)"
		}
	}
	return false, ""
}

// printPackageRecordSummary prints key fields from an existing package before update/skip.
func printPackageRecordSummary(pkg *PackageRecord) {
	if pkg == nil {
		return
	}
	fmt.Printf("  packageName: %s\n", pkg.PackageName)
	fmt.Printf("  fileName:    %s\n", pkg.FileName)
	fmt.Printf("  categoryId:  %s\n", pkg.CategoryID)
	fmt.Printf("  priority:    %d\n", pkg.Priority)
	status := pkg.CloudTransferStatus
	if status == "" {
		status = "(none)"
	}
	fmt.Printf("  status:      %s\n", status)
	if pkg.Md5 != "" {
		fmt.Printf("  md5:         %s\n", pkg.Md5)
	} else {
		fmt.Printf("  md5:         (none)\n")
	}
	if pkg.Sha256 != "" {
		fmt.Printf("  sha256:      %s\n", pkg.Sha256)
	} else {
		fmt.Printf("  sha256:      (none)\n")
	}
	if pkg.Sha3512 != "" {
		fmt.Printf("  sha3512:     %s\n", pkg.Sha3512)
	} else {
		fmt.Printf("  sha3512:     (none)\n")
	}
	fmt.Printf("  hashType:    %s\n", emptyDash(pkg.HashType))
	fmt.Printf("  hashValue:   %s\n", emptyDash(pkg.HashValue))
}

func emptyDash(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// Package check classes. Issue classes (pending, failed-upload, unusable-ready)
// are reused when the Jamf record is incomplete.
const (
	PackageCheckOK       = "ok"
	PackageCheckMismatch = "mismatch"
	PackageCheckMissing  = "missing"
)

// PackageCheckResult is a read-only comparison of a local package file to a Jamf record.
type PackageCheckResult struct {
	OK                  bool   `json:"ok"`
	Class               string `json:"class"`
	ID                  string `json:"id,omitempty"`
	PackageName         string `json:"packageName,omitempty"`
	FileName            string `json:"fileName,omitempty"`
	RecordFileName      string `json:"recordFileName,omitempty"`
	CloudTransferStatus string `json:"cloudTransferStatus,omitempty"`
	Match               bool   `json:"match"`
	MatchedBy           string `json:"matchedBy,omitempty"`
	LocalMD5            string `json:"localMd5,omitempty"`
	LocalSHA256         string `json:"localSha256,omitempty"`
	FileNameDiffers     bool   `json:"fileNameDiffers,omitempty"`
	CategoryID          string `json:"categoryId,omitempty"`
	Priority            int    `json:"priority,omitempty"`
	Md5                 string `json:"md5,omitempty"`
	Sha256              string `json:"sha256,omitempty"`
	Sha3512             string `json:"sha3512,omitempty"`
	HashType            string `json:"hashType,omitempty"`
	HashValue           string `json:"hashValue,omitempty"`
}

// classifyPackageCheck decides the check class without calling Jamf.
// A nil record is missing. ok is true only when a checksum matches and the
// package is usable (READY with sha256).
func classifyPackageCheck(pkg *PackageRecord, local packageFileHashes) (class string, ok bool, matched bool, field string) {
	if pkg == nil {
		return PackageCheckMissing, false, false, ""
	}
	matched, field = packageFileMatches(pkg, local)
	if matched && packageIsUsable(pkg) {
		return PackageCheckOK, true, true, field
	}
	if c := packageIssueClass(pkg); c != "" {
		return c, false, matched, field
	}
	if !matched {
		return PackageCheckMismatch, false, false, ""
	}
	return PackageIssuePending, false, true, field
}

// PackageCheck hashes a local .pkg or .dmg and compares it to a Jamf package.
// id empty looks up by fileName. A missing record is a result (class missing),
// not an error. I/O failures and unexpected API statuses are errors.
func PackageCheck(credName, filePath, id string, logger *slog.Logger) (PackageCheckResult, error) {
	fileName := path.Base(filePath)
	ext := strings.ToLower(path.Ext(fileName))
	if ext != ".pkg" && ext != ".dmg" {
		return PackageCheckResult{}, fmt.Errorf("unsupported file type %q: only .pkg and .dmg are supported (bundle packages must be zipped first)", ext)
	}
	f, err := os.Open(filePath)
	if err != nil {
		return PackageCheckResult{}, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return PackageCheckResult{}, fmt.Errorf("stat file: %w", err)
	}
	fmt.Printf("Hashing local file (%s)…\n", formatBytes(info.Size()))
	local, err := hashPackageReader(f, info.Size())
	if err != nil {
		return PackageCheckResult{}, fmt.Errorf("hash local file: %w", err)
	}

	var pkg *PackageRecord
	id = strings.TrimSpace(id)
	if id != "" {
		got, status, err := packageGetStatus(credName, id)
		if err != nil {
			return PackageCheckResult{}, err
		}
		if status == http.StatusNotFound {
			pkg = nil
		} else if status != http.StatusOK {
			return PackageCheckResult{}, fmt.Errorf("get package %s status %d", id, status)
		} else {
			pkg = got
		}
	} else {
		found, err := packageFindByFileName(credName, fileName, logger)
		if err != nil {
			return PackageCheckResult{}, err
		}
		if found == "" {
			class, ok, matched, field := classifyPackageCheck(nil, local)
			return PackageCheckResult{
				OK: ok, Class: class, FileName: fileName, Match: matched, MatchedBy: field,
				LocalMD5: local.MD5, LocalSHA256: local.SHA256,
			}, nil
		}
		got, err := packageGet(credName, found)
		if err != nil {
			return PackageCheckResult{}, err
		}
		pkg = got
	}

	class, ok, matched, field := classifyPackageCheck(pkg, local)
	res := PackageCheckResult{
		OK: ok, Class: class, FileName: fileName, Match: matched, MatchedBy: field,
		LocalMD5: local.MD5, LocalSHA256: local.SHA256,
	}
	if pkg != nil {
		res.ID = pkg.ID
		res.PackageName = pkg.PackageName
		res.RecordFileName = pkg.FileName
		res.CloudTransferStatus = pkg.CloudTransferStatus
		res.FileNameDiffers = pkg.FileName != "" && pkg.FileName != fileName
		res.CategoryID = pkg.CategoryID
		res.Priority = pkg.Priority
		res.Md5 = pkg.Md5
		res.Sha256 = pkg.Sha256
		res.Sha3512 = pkg.Sha3512
		res.HashType = pkg.HashType
		res.HashValue = pkg.HashValue
	}
	return res, nil
}

// PackageCheckPrint hashes and compares, prints the report, and returns an error
// when the file is not a checksum match on a usable package. API and I/O errors
// are returned before a report.
func PackageCheckPrint(credName, filePath, id string, logger *slog.Logger) error {
	res, err := PackageCheck(credName, filePath, id, logger)
	if err != nil {
		return err
	}
	printPackageCheck(res)
	if !res.OK {
		return fmt.Errorf("package check: %s", res.Class)
	}
	return nil
}

func printPackageCheck(res PackageCheckResult) {
	fmt.Printf("Package check: %s\n", res.Class)
	if res.ID != "" {
		fmt.Printf("  id:           %s\n", res.ID)
	}
	if res.PackageName != "" {
		fmt.Printf("  packageName:  %s\n", res.PackageName)
	}
	fmt.Printf("  fileName:     %s\n", res.FileName)
	if res.FileNameDiffers {
		fmt.Printf("  record file:  %s\n", res.RecordFileName)
	}
	if res.Class != PackageCheckMissing {
		fmt.Printf("  categoryId:   %s\n", res.CategoryID)
		fmt.Printf("  priority:     %d\n", res.Priority)
		status := res.CloudTransferStatus
		if status == "" {
			status = "(none)"
		}
		fmt.Printf("  status:       %s\n", status)
		fmt.Printf("  md5:          %s\n", emptyDash(res.Md5))
		fmt.Printf("  sha256:       %s\n", emptyDash(res.Sha256))
		fmt.Printf("  sha3512:      %s\n", emptyDash(res.Sha3512))
		fmt.Printf("  hashType:     %s\n", emptyDash(res.HashType))
		fmt.Printf("  hashValue:    %s\n", emptyDash(res.HashValue))
	}
	fmt.Printf("  local md5:    %s\n", res.LocalMD5)
	fmt.Printf("  local sha256: %s\n", res.LocalSHA256)
	if res.Match {
		fmt.Printf("  match:        %s\n", res.MatchedBy)
	} else {
		fmt.Printf("  match:        no\n")
	}
}

// packageApplyAttrs applies attrs onto existing (or fetches) and PUTs.
func packageApplyAttrs(credName, id string, opts Opts, attrs PackageAttrs, existing *PackageRecord, logger *slog.Logger) error {
	_ = opts
	_ = logger
	pkg := existing
	var err error
	if pkg == nil {
		pkg, err = packageGet(credName, id)
		if err != nil {
			return err
		}
	}
	attrs.Apply(pkg)
	updated, err := packagePut(credName, id, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("Updated package %s metadata: %s\n", id, strings.Join(attrs.summaryLines(updated), ", "))
	return nil
}

// PackageSetAttrs updates one or more package metadata fields (same attrs as upload).
func PackageSetAttrs(credName, id string, opts Opts, attrs PackageAttrs, logger *slog.Logger) error {
	if err := attrs.Validate(); err != nil {
		return err
	}
	if !attrs.AnySet() {
		return fmt.Errorf("no package attributes to set (use --name=, --notes=, --info=, --category=, --priority=, --reboot=)")
	}
	pkg, err := packageGet(credName, id)
	if err != nil {
		return err
	}
	if !attrs.DiffersFrom(pkg) {
		fmt.Printf("Package %s (%q): metadata already matches — nothing to do\n", id, pkg.PackageName)
		return nil
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpdate package %s (%q): %s?", id, pkg.PackageName, strings.Join(attrs.summaryLines(pkg), ", ")), logger) {
			return nil
		}
	}
	return packageApplyAttrs(credName, id, opts, attrs, pkg, logger)
}

// packageGetLatestHistory fetches the single most-recent history entry for a package.
// Returns nil (no error) when no history entries exist.
func packageGetLatestHistory(credName, id string) (*PackageHistoryEntry, error) {
	body, err := jamfGET(credName, uriPackages+"/"+id+"/history?sort=date:desc&page-size=1")
	if err != nil {
		return nil, fmt.Errorf("get package history %s: %w", id, err)
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
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s/history?sort=date:desc&page-size=%d", uriPackages, id, limit))
	if err != nil {
		return fmt.Errorf("get package history %s: %w", id, err)
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
