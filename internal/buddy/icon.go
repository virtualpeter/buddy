package buddy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	uriIcon         = "/api/v1/icon"
	uriIconDownload = "/api/v1/icon/download"

	iconICSCanonicalHost = "https://ics.services.jamfcloud.com"
	iconICSUse2Host      = "https://use2.ics.services.jamfcloud.com"
)

// iconICSHosts is the ICS bases probed by IconCheck (canonical first, then use2).
// Tests may replace this slice.
var iconICSHosts = []string{iconICSCanonicalHost, iconICSUse2Host}

// Icon is a Jamf Pro icon record (GET/POST /api/v1/icon). Live responses include
// name even when OpenAPI only documents id+url.
type Icon struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// IconCheckResult is a local SHA-256 plus unauthenticated ICS HEAD/GET probe.
// Present is true when ICS has the hash, or (when IconID is set) when the
// tenant icon's bytes match. ICS does not imply a tenant icon id.
type IconCheckResult struct {
	URL     string `json:"url"`
	Present bool   `json:"present"`
	SHA256  string `json:"sha256"`
	ICS     bool   `json:"ics"`
	IconID  int    `json:"icon_id,omitempty"`
}

// IconGet loads one icon by tenant-local id.
func IconGet(credName string, id int) (Icon, error) {
	if id <= 0 {
		return Icon{}, fmt.Errorf("icon id must be positive")
	}
	body, err := jamfGET(credName, uriIcon+"/"+strconv.Itoa(id))
	if err != nil {
		return Icon{}, err
	}
	icon, err := parseIconJSON(body)
	if err != nil {
		return Icon{}, err
	}
	icon.URL = normalizeIconURL(icon.URL)
	return icon, nil
}

// IconUpload POSTs a local image file and returns the tenant icon record.
func IconUpload(credName, filePath string) (Icon, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Icon{}, fmt.Errorf("read icon file: %w", err)
	}
	return IconUploadBytes(credName, path.Base(filePath), data)
}

// IconUploadBytes POSTs image bytes as multipart field "file".
func IconUploadBytes(credName, filename string, data []byte) (Icon, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "icon.png"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return Icon{}, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return Icon{}, fmt.Errorf("write file to form: %w", err)
	}
	if err := w.Close(); err != nil {
		return Icon{}, fmt.Errorf("close multipart: %w", err)
	}
	body, err := jamfPOST(credName, uriIcon, buf.Bytes(), w.FormDataContentType())
	if err != nil {
		return Icon{}, err
	}
	icon, err := parseIconJSON(body)
	if err != nil {
		return Icon{}, err
	}
	icon.URL = normalizeIconURL(icon.URL)
	if icon.ID <= 0 {
		return Icon{}, fmt.Errorf("icon upload: missing id in response")
	}
	return icon, nil
}

// IconDownload fetches image bytes. Prefer the metadata url (ICS or instance
// download path) with no Jamf token — GET /api/v1/icon/download/{id} returns 500
// for ICS-backed icons, and a bearer token on the instance url can 401.
func IconDownload(credName string, id int) ([]byte, error) {
	if id <= 0 {
		return nil, fmt.Errorf("icon id must be positive")
	}
	rec, err := IconGet(credName, id)
	if err != nil {
		return nil, err
	}
	return IconFetch(credName, rec)
}

// IconFetch loads image bytes for an already-fetched icon record. Prefers
// rec.URL (unauthenticated ICS or instance download path); falls back to
// authenticated GET /api/v1/icon/download/{id}.
func IconFetch(credName string, rec Icon) ([]byte, error) {
	var urlErr error
	if u := strings.TrimSpace(rec.URL); u != "" {
		data, err := fetchIconURL(u)
		if err == nil {
			return data, nil
		}
		urlErr = err
	}
	data, err := jamfGETWithAccept(credName, uriIconDownload+"/"+strconv.Itoa(rec.ID), "*/*")
	if err == nil {
		return data, nil
	}
	if urlErr != nil {
		return nil, fmt.Errorf("icon %d: metadata url: %v; download API: %w", rec.ID, urlErr, err)
	}
	return nil, err
}

// IconCheck SHA-256s a local file and HEADs ICS (no Jamf auth). First 200/206
// wins. If no host has the object, Present is false and URL is the canonical
// ics.services URL. Instance-only icons (url is /api/v1/icon/download/{id}, no
// hash_) are not on ICS — use IconCheckFile with an icon id to compare bytes.
// Jamf may re-encode PNGs, so ICS can false-negative after upload.
func IconCheck(filePath string) (IconCheckResult, error) {
	return IconCheckFile("", filePath, 0)
}

// IconCheckFile is IconCheck plus an optional tenant icon id. When iconID > 0,
// the local SHA-256 is also compared to that icon's downloaded bytes. Matching
// instance-only icons set Present (ICS stays false; URL becomes the metadata url).
func IconCheckFile(credName, filePath string, iconID int) (IconCheckResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return IconCheckResult{}, fmt.Errorf("read icon file: %w", err)
	}
	out, err := iconCheckBytes(data)
	if err != nil {
		return IconCheckResult{}, err
	}
	if iconID <= 0 {
		return out, nil
	}
	rec, err := IconGet(credName, iconID)
	if err != nil {
		return IconCheckResult{}, err
	}
	got, err := IconFetch(credName, rec)
	if err != nil {
		return IconCheckResult{}, fmt.Errorf("download icon %d: %w", iconID, err)
	}
	out.IconID = iconID
	if IconSHA256Hex(got) == out.SHA256 {
		out.Present = true
		if !out.ICS && strings.TrimSpace(rec.URL) != "" {
			out.URL = rec.URL
		}
	}
	return out, nil
}

func iconCheckBytes(data []byte) (IconCheckResult, error) {
	sum := IconSHA256Hex(data)
	canonical := IconHashURL(iconICSCanonicalHost, sum)
	out := IconCheckResult{URL: canonical, SHA256: sum}
	hosts := iconICSHosts
	if len(hosts) == 0 {
		hosts = []string{iconICSCanonicalHost}
	}
	for _, host := range hosts {
		u := IconHashURL(host, sum)
		present, err := iconICSPresent(u)
		if err != nil {
			return IconCheckResult{}, err
		}
		if present {
			out.URL = u
			out.Present = true
			out.ICS = true
			return out, nil
		}
	}
	return out, nil
}

// IconSHA256Hex returns lowercase hex SHA-256 of data.
func IconSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IconHashURL builds https://host/icon/hash_<sha256hex>.
func IconHashURL(host, sha256hex string) string {
	return strings.TrimRight(host, "/") + "/icon/hash_" + sha256hex
}

func parseIconJSON(body []byte) (Icon, error) {
	var icon Icon
	if err := json.Unmarshal(body, &icon); err != nil {
		return Icon{}, fmt.Errorf("parse icon response: %w", err)
	}
	icon.URL = normalizeIconURL(icon.URL)
	return icon, nil
}

// normalizeIconURL collapses extra slashes in the path (Jamf often returns
// https://tenant.jamfcloud.com//api/v1/icon/download/{id}).
func normalizeIconURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	p := u.Path
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	u.Path = p
	return u.String()
}

func fetchIconURL(rawURL string) ([]byte, error) {
	rawURL = normalizeIconURL(rawURL)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create icon URL request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	resp, err := PerformRequestNoAuth(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read icon URL: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s status %d", rawURL, resp.StatusCode)
	}
	if iconBodyIsAPIError(body) {
		return nil, fmt.Errorf("GET %s: API error body", rawURL)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("GET %s: empty body", rawURL)
	}
	return body, nil
}

func iconBodyIsAPIError(body []byte) bool {
	s := strings.TrimSpace(string(body))
	return strings.HasPrefix(s, "{") && strings.Contains(s, `"httpStatus"`)
}

func iconICSPresent(rawURL string) (bool, error) {
	ok, status, err := iconICSProbe(http.MethodHead, rawURL, false)
	if err != nil {
		return false, err
	}
	if status == http.StatusMethodNotAllowed {
		ok, _, err = iconICSProbe(http.MethodGet, rawURL, true)
		if err != nil {
			return false, err
		}
		return ok, nil
	}
	return ok, nil
}

func iconICSProbe(method, rawURL string, rangeGet bool) (ok bool, status int, err error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return false, 0, fmt.Errorf("create ICS request: %w", err)
	}
	if rangeGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := PerformRequestNoAuth(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 64)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return true, resp.StatusCode, nil
	case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
		return false, resp.StatusCode, nil
	default:
		return false, resp.StatusCode, nil
	}
}
