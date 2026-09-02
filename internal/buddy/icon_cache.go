package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const iconCatalogFile = "icons.json"

// iconScanTail is how far past max(cachedMax, lastHit) the walk continues (404s are holes).
var iconScanTail = 20

var iconHashPathRe = regexp.MustCompile(`(?i)/icon/hash_([0-9a-f]{64})`)

// IconCatalogCache is the on-disk JSON envelope for tenant icons.
type IconCatalogCache struct {
	FetchedAt time.Time `json:"fetched_at"`
	Tenant    string    `json:"tenant"`
	Auth      string    `json:"auth,omitempty"`
	MaxID     int       `json:"max_id"`
	Items     []Icon    `json:"items"`
}

// IconListOpts controls IconList.
type IconListOpts struct {
	Filter    string
	Refresh   bool
	DropCache bool // ignore (and replace) the on-disk catalog; walk from id 1
	Progress  bool // stderr: cache status, walk warning, live scan (\r)
}

// IconListResult is the filtered icon list plus cache/walk metadata for CLI stderr.
type IconListResult struct {
	Icons       []Icon
	Total       int
	FromCache   bool
	Age         time.Duration
	ScannedFrom int
	ScannedTo   int
}

// IconList returns tenant icons from the weekly catalog cache, walking
// GET /api/v1/icon/{id} when the cache is missing, stale, or Refresh is set.
// A first walk (or DropCache) starts at id 1. Later walks keep cached rows
// and only probe 20 ids past the previous high-water mark (grow-on-hit).
func IconList(credName string, opts IconListOpts, log *slog.Logger) (IconListResult, error) {
	cat, walked, scannedFrom, scannedTo, err := iconLoadOrRefreshCatalog(credName, opts, log)
	if err != nil {
		return IconListResult{}, err
	}
	items := cat.Items
	if items == nil {
		items = []Icon{}
	}
	out := IconListResult{
		Total:     len(items),
		FromCache: !walked,
	}
	if !cat.FetchedAt.IsZero() {
		out.Age = time.Since(cat.FetchedAt)
	}
	if walked {
		out.ScannedFrom = scannedFrom
		out.ScannedTo = scannedTo
	}
	out.Icons = filterIcons(items, opts.Filter)
	return out, nil
}

func iconLoadOrRefreshCatalog(credName string, opts IconListOpts, log *slog.Logger) (IconCatalogCache, bool, int, int, error) {
	existing, err := loadIconCatalog(credName)
	if err != nil {
		return IconCatalogCache{}, false, 0, 0, err
	}
	if opts.DropCache {
		existing = nil
	}
	now := time.Now()
	if !opts.DropCache && !opts.Refresh && existing != nil && !CatalogCacheStale(existing.FetchedAt, now, CatalogCacheTTL) {
		if opts.Progress {
			fmt.Fprintln(os.Stderr, iconCatalogStatusLine(existing, now))
		}
		return *existing, false, 0, 0, nil
	}
	cachedMax := 0
	if existing != nil {
		cachedMax = existing.MaxID
		for _, it := range existing.Items {
			if it.ID > cachedMax {
				cachedMax = it.ID
			}
		}
	}
	if opts.Progress {
		if cachedMax <= 0 {
			fmt.Fprintln(os.Stderr, "warning: Jamf has no icon list API; probing GET /api/v1/icon/{id} from 1 (one request per id; can take minutes)")
		} else {
			fmt.Fprintf(os.Stderr, "warning: Jamf has no icon list API and no published delete; refresh probes %d ids past max (use --dropcache to rescan from 1)\n", iconScanTail)
		}
		fmt.Fprintln(os.Stderr, iconWalkBanner(existing, now, cachedMax))
	}
	cat, scannedFrom, scannedTo, err := iconWalkCatalog(credName, existing, opts.Progress, log)
	if err != nil {
		return IconCatalogCache{}, false, 0, 0, err
	}
	if err := saveIconCatalog(credName, cat); err != nil {
		return IconCatalogCache{}, false, 0, 0, err
	}
	return cat, true, scannedFrom, scannedTo, nil
}

func iconWalkCatalog(credName string, existing *IconCatalogCache, progress bool, log *slog.Logger) (IconCatalogCache, int, int, error) {
	sess, err := ResolveSessionForCred(credName, OpRead)
	if err != nil {
		return IconCatalogCache{}, 0, 0, err
	}
	items := make([]Icon, 0)
	cachedMax := 0
	if existing != nil {
		cachedMax = existing.MaxID
		for _, it := range existing.Items {
			if it.ID <= 0 {
				continue
			}
			items = append(items, it)
			if it.ID > cachedMax {
				cachedMax = it.ID
			}
		}
	}
	lastHit := cachedMax
	scannedTo := 0
	started := time.Now()
	start := cachedMax + 1
	if start < 1 {
		start = 1
	}
	for id := start; ; id++ {
		if id > iconWalkStopAt(cachedMax, lastHit) {
			break
		}
		scannedTo = id
		icon, gerr := iconGetLive(credName, id)
		if gerr != nil && !jamfErrorHasStatus(gerr, 404) {
			if progress {
				fmt.Fprintln(os.Stderr)
			}
			return IconCatalogCache{}, 0, 0, gerr
		}
		if gerr == nil {
			lastHit = icon.ID
			iconHashOnRefresh(&icon, log)
			items = append(items, icon)
		}
		if progress {
			fmt.Fprintf(os.Stderr, "\r%s", iconScanProgressLine(id, iconWalkStopAt(cachedMax, lastHit), len(items), lastHit, time.Since(started)))
		}
	}
	if progress {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "scanned %d–%d, %d icons\n", start, scannedTo, len(items))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	cat := IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     lastHit,
		Items:     items,
	}
	if sess != nil {
		cat.Tenant = sess.TenantName
		cat.Auth = sess.AuthName
	}
	return cat, start, scannedTo, nil
}

func iconCatalogStatusLine(cat *IconCatalogCache, now time.Time) string {
	if cat == nil || cat.FetchedAt.IsZero() {
		return "icon cache missing"
	}
	if now.IsZero() {
		now = time.Now()
	}
	n := len(cat.Items)
	return fmt.Sprintf("icon cache %s old (%d icons, max id %d)", FormatCatalogCacheAge(now.Sub(cat.FetchedAt)), n, cat.MaxID)
}

func iconWalkBanner(cat *IconCatalogCache, now time.Time, cachedMax int) string {
	status := iconCatalogStatusLine(cat, now)
	if cachedMax <= 0 {
		return fmt.Sprintf("%s; scanning 1–%d", status, iconScanTail)
	}
	start := cachedMax + 1
	end := cachedMax + iconScanTail
	return fmt.Sprintf("%s; scanning %d–%d", status, start, end)
}

func iconScanProgressLine(id, stopAt, hits, lastHit int, elapsed time.Duration) string {
	return fmt.Sprintf("icon scan: %d/%d (%d hits, last %d) %s", id, stopAt, hits, lastHit, iconScanElapsed(elapsed))
}

func iconScanElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Second {
		return "0s"
	}
	return d.String()
}

func iconWalkStopAt(cachedMax, lastHit int) int {
	if lastHit > cachedMax {
		return lastHit + iconScanTail
	}
	return cachedMax + iconScanTail
}

func iconHashOnRefresh(icon *Icon, log *slog.Logger) {
	iconFillSHAFromURL(icon)
	if icon.SHA256 != "" || strings.TrimSpace(icon.URL) == "" {
		return
	}
	data, err := fetchIconURL(icon.URL)
	if err != nil {
		if log != nil {
			log.Debug("icon catalog: hash fetch failed", "id", icon.ID, "error", err.Error())
		}
		return
	}
	icon.SHA256 = IconSHA256Hex(data)
}

func loadIconCatalog(credName string) (*IconCatalogCache, error) {
	path, err := catalogCacheFilePath(credName, iconCatalogFile)
	if err != nil {
		return nil, err
	}
	var cat IconCatalogCache
	err = loadCatalogCacheJSON(path, &cat)
	if err != nil && os.IsNotExist(err) {
		legacy, lerr := catalogCacheNewestLegacyFile(credName, iconCatalogFile)
		if lerr != nil {
			return nil, lerr
		}
		if legacy == "" {
			return nil, nil
		}
		path = legacy
		err = loadCatalogCacheJSON(path, &cat)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range cat.Items {
		cat.Items[i].URL = normalizeIconURL(cat.Items[i].URL)
	}
	return &cat, nil
}

func saveIconCatalog(credName string, cat IconCatalogCache) error {
	path, err := catalogCacheFilePath(credName, iconCatalogFile)
	if err != nil {
		return err
	}
	if err := saveCatalogCacheJSON(path, cat); err != nil {
		return err
	}
	removeCatalogCacheLegacyFiles(credName, iconCatalogFile)
	return nil
}

func iconCatalogUpsert(credName string, icon Icon) error {
	if icon.ID <= 0 {
		return nil
	}
	iconFillSHAFromURL(&icon)
	cat, err := loadIconCatalog(credName)
	if err != nil {
		return err
	}
	if cat == nil {
		cat = &IconCatalogCache{}
		if sess, serr := ResolveSessionForCred(credName, OpRead); serr == nil && sess != nil {
			cat.Tenant = sess.TenantName
			cat.Auth = sess.AuthName
		}
	}
	replaced := false
	for i, it := range cat.Items {
		if it.ID == icon.ID {
			cat.Items[i] = icon
			replaced = true
			break
		}
	}
	if !replaced {
		cat.Items = append(cat.Items, icon)
	}
	sort.Slice(cat.Items, func(i, j int) bool { return cat.Items[i].ID < cat.Items[j].ID })
	if icon.ID > cat.MaxID {
		cat.MaxID = icon.ID
	}
	return saveIconCatalog(credName, *cat)
}

func iconCatalogGetByID(credName string, id int) (Icon, bool) {
	cat, err := loadIconCatalog(credName)
	if err != nil || cat == nil {
		return Icon{}, false
	}
	for _, it := range cat.Items {
		if it.ID == id {
			return it, true
		}
	}
	return Icon{}, false
}

func iconCatalogFindBySHA(credName, sha256hex string) (Icon, bool) {
	all := iconCatalogBySHA(credName, sha256hex)
	if len(all) == 0 {
		return Icon{}, false
	}
	return all[0], true
}

func iconCatalogBySHA(credName, sha256hex string) []Icon {
	sha256hex = strings.ToLower(strings.TrimSpace(sha256hex))
	if sha256hex == "" {
		return nil
	}
	cat, err := loadIconCatalog(credName)
	if err != nil || cat == nil {
		return nil
	}
	var out []Icon
	for _, it := range cat.Items {
		if strings.ToLower(strings.TrimSpace(it.SHA256)) == sha256hex {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// iconReuseFromCache returns the lowest-id live catalog row whose sha256 matches.
// 404s are skipped; other GET errors abort. Missing cache or no match → ok=false.
func iconReuseFromCache(credName, sha256hex string) (Icon, bool, error) {
	for _, rec := range iconCatalogBySHA(credName, sha256hex) {
		live, err := iconGetLive(credName, rec.ID)
		if jamfErrorHasStatus(err, 404) {
			continue
		}
		if err != nil {
			return Icon{}, false, err
		}
		iconFillSHAFromURL(&live)
		if live.SHA256 == "" {
			live.SHA256 = sha256hex
		}
		return live, true, nil
	}
	return Icon{}, false, nil
}

// IconGetByName loads one icon from the catalog cache by exact name.
func IconGetByName(credName, name string) (Icon, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Icon{}, fmt.Errorf("icon name is required")
	}
	cat, err := loadIconCatalog(credName)
	if err != nil {
		return Icon{}, err
	}
	if cat == nil {
		return Icon{}, fmt.Errorf("icon catalog cache missing; run buddy icon list or icons()")
	}
	if CatalogCacheStale(cat.FetchedAt, time.Now(), CatalogCacheTTL) && len(cat.Items) == 0 {
		return Icon{}, fmt.Errorf("icon catalog cache empty; run buddy icon list --dropcache or icons(dropcache=True)")
	}
	var matches []Icon
	for _, it := range cat.Items {
		if it.Name == name {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 0:
		return Icon{}, fmt.Errorf("icon %q not found in catalog cache", name)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, strconv.Itoa(m.ID))
		}
		return Icon{}, fmt.Errorf("multiple icons named %q (ids %s); use id=", name, strings.Join(ids, ", "))
	}
}

func filterIcons(items []Icon, filter string) []Icon {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		out := make([]Icon, len(items))
		copy(out, items)
		return out
	}
	needle := strings.ToLower(filter)
	out := make([]Icon, 0)
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), needle) || strings.Contains(strconv.Itoa(it.ID), needle) {
			out = append(out, it)
		}
	}
	return out
}

func iconSHAFromURL(raw string) string {
	m := iconHashPathRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return strings.ToLower(m[1])
}

func iconURLHasICSHash(raw string) bool {
	return iconSHAFromURL(raw) != ""
}

func iconFillSHAFromURL(icon *Icon) {
	if icon == nil || icon.SHA256 != "" {
		return
	}
	if h := iconSHAFromURL(icon.URL); h != "" {
		icon.SHA256 = h
	}
}
