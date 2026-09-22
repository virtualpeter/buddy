package buddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"buddy/internal/buddy/secrets"
)

const updateCheckInterval = 24 * time.Hour

const releasePagePrefix = "https://github.com/virtualpeter/buddy/releases/"

// updateReleaseURL is the GitHub latest-release endpoint. Tests point it at httptest.
var updateReleaseURL = "https://api.github.com/repos/virtualpeter/buddy/releases/latest"

var updateHTTPClient = &http.Client{Timeout: 8 * time.Second}

var releaseVersionRe = regexp.MustCompile(`^v\d+\.\d+(?:\.\d+)*$`)

// UpdateNotice is what Buddy.app and the CLI show after a release check.
// Current is the running version even when there is no update.
type UpdateNotice struct {
	Current string `json:"current"`
	Update  bool   `json:"update"`
	Checked bool   `json:"checked"`
	Version string `json:"version,omitempty"`
	URL     string `json:"url,omitempty"`
}

type updateCacheFile struct {
	CheckedAt string `json:"checked_at"`
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	URL       string `json:"url,omitempty"`
	OK        bool   `json:"ok"`
}

// PrintUpdateNotice writes one stderr line when the CLI is explicitly opted in
// and a newer release exists. It never returns an error to the caller.
func PrintUpdateNotice(w io.Writer, current string) {
	if !GetActiveConfig().CheckUpdatesCLI() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	n := RefreshUpdateCheck(ctx, current)
	if !n.Update || n.URL == "" {
		return
	}
	fmt.Fprintf(w, "buddy: %s is available: %s\n", n.Version, n.URL)
}

// CachedUpdateNotice reads the daily cache. Checked is false when the cache
// is missing, stale, or was written for a different running version.
func CachedUpdateNotice(current string) UpdateNotice {
	n := UpdateNotice{Current: current}
	file, ok := readUpdateCache()
	if !ok || file.Current != current || !updateCacheFresh(file.CheckedAt, time.Now()) {
		return n
	}
	n.Checked = true
	applyUpdateNotice(&n, file)
	return n
}

// RefreshUpdateCheck returns a fresh notice, hitting GitHub at most once per
// 24 hours for this running version. A failed attempt still counts for the day.
// Non-release versions (dev, dirty, short commit) stay quiet and do not fetch.
func RefreshUpdateCheck(ctx context.Context, current string) UpdateNotice {
	if cached := CachedUpdateNotice(current); cached.Checked {
		return cached
	}
	file := updateCacheFile{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Current:   current,
	}
	if isReleaseVersion(current) {
		tag, page, err := fetchLatestRelease(ctx, current)
		if err != nil {
			if prev, ok := readUpdateCache(); ok && prev.Current == current && prev.OK {
				file.Latest = prev.Latest
				file.URL = prev.URL
				file.OK = isReleaseVersion(prev.Latest) && validReleaseURL(prev.URL)
			}
		} else {
			file.Latest = tag
			file.URL = page
			file.OK = true
		}
	} else {
		file.OK = true
	}
	_ = writeUpdateCache(file)
	n := UpdateNotice{Current: current, Checked: true}
	applyUpdateNotice(&n, file)
	return n
}

func applyUpdateNotice(n *UpdateNotice, file updateCacheFile) {
	if !file.OK || !releaseNewer(n.Current, file.Latest) || !validReleaseURL(file.URL) {
		return
	}
	n.Update = true
	n.Version = file.Latest
	n.URL = file.URL
}

func isReleaseVersion(s string) bool {
	return releaseVersionRe.MatchString(strings.TrimSpace(s))
}

func releaseNewer(current, latest string) bool {
	if !isReleaseVersion(current) || !isReleaseVersion(latest) {
		return false
	}
	return CompareVersions(current, latest) < 0
}

func validReleaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil {
		return false
	}
	return strings.HasPrefix(u.Path, "/virtualpeter/buddy/releases/")
}

func updateCacheFresh(checkedAt string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return false
	}
	age := now.Sub(t)
	return age >= 0 && age < updateCheckInterval
}

func updateCheckPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("BUDDY_UPDATE_CHECK_PATH")); p != "" {
		return p, nil
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Buddy", "update-check.json"), nil
	}
	dir, err := secrets.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check.json"), nil
}

func readUpdateCache() (updateCacheFile, bool) {
	path, err := updateCheckPath()
	if err != nil {
		return updateCacheFile{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateCacheFile{}, false
	}
	var file updateCacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return updateCacheFile{}, false
	}
	return file, true
}

func writeUpdateCache(file updateCacheFile) error {
	path, err := updateCheckPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type gitHubLatestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestRelease(ctx context.Context, current string) (tag, page string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateReleaseURL, nil)
	if err != nil {
		return "", "", err
	}
	ua := "buddy"
	if strings.TrimSpace(current) != "" {
		ua = "buddy/" + strings.TrimSpace(current)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github release check: %s", resp.Status)
	}
	var rel gitHubLatestRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", err
	}
	tag = strings.TrimSpace(rel.TagName)
	if !isReleaseVersion(tag) {
		return "", "", fmt.Errorf("github release check: tag %q is not a release", tag)
	}
	page = strings.TrimSpace(rel.HTMLURL)
	if !validReleaseURL(page) {
		page = "https://github.com/virtualpeter/buddy/releases/tag/" + tag
	}
	if !validReleaseURL(page) {
		return "", "", fmt.Errorf("github release check: no release page")
	}
	return tag, page, nil
}
