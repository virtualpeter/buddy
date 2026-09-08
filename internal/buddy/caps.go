package buddy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"buddy/internal/buddy/secrets"
)

// Live status values persisted after buddy auth test (and shown by cred status).
const (
	LiveNever         = "never"
	LiveOK            = "ok"
	LiveTokenOK       = "token-ok"       // token minted; probe 401/403
	LiveTokenFail     = "token-fail"     // token POST failed (bad/stale secret)
	LiveSecretMissing = "secret-missing" // no inline or store secret
)

const capsDiskVersion = 1
const capsDefaultTTL = 24 * time.Hour

// Compiled obfuscation material (not a user secret). Changing this invalidates
// existing caps files — they are deleted on the next load/sweep.
const capsCacheObfuscationMaterial = "buddy\x00caps-cache\x00v1\x00" +
	"a1c3e5f709b2d4e6f8a0c2d4e6f8091b3d5f7a9c"

func capsCacheAESKey() []byte {
	sum := sha256.Sum256([]byte(capsCacheObfuscationMaterial))
	return sum[:]
}

func sealCaps(plain []byte) (string, error) {
	block, err := aes.NewCipher(capsCacheAESKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func openCaps(sealed string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sealed))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(capsCacheAESKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, raw[:ns], raw[ns:], nil)
}

// CapsRecord is the last live auth check and (when available) instance privileges.
type CapsRecord struct {
	Auth           string    `json:"auth"`
	Tenant         string    `json:"tenant"`
	Surface        string    `json:"surface"`
	TenantID       string    `json:"tenantId,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Live           string    `json:"live"` // ok | token-ok | token-fail | secret-missing
	TokenOk        bool      `json:"tokenOk"`
	ProbeOk        bool      `json:"probeOk"`
	Probe          string    `json:"probe,omitempty"`
	ProbeStatus    int       `json:"probeStatus,omitempty"`
	Error          string    `json:"error,omitempty"`
	Username       string    `json:"username,omitempty"`
	AccessLevel    string    `json:"accessLevel,omitempty"`
	PrivilegeSet   string    `json:"privilegeSet,omitempty"`
	PrivilegeCount int       `json:"privilegeCount,omitempty"`
	Privileges     []string  `json:"privileges,omitempty"`
}

type diskCapsFile struct {
	V       int       `json:"v"`
	Payload string    `json:"payload"`
	Expiry  time.Time `json:"expiry"`
}

var (
	capsCacheDirMu       sync.Mutex
	capsCacheDirOverride string
	capsMem              struct {
		mu    sync.Mutex
		byKey map[string]*CapsRecord
	}
)

func init() {
	capsMem.byKey = make(map[string]*CapsRecord)
}

// SetCapsCacheDirForTest overrides the caps cache directory (tests only).
func SetCapsCacheDirForTest(dir string) (restore func()) {
	capsCacheDirMu.Lock()
	prev := capsCacheDirOverride
	capsCacheDirOverride = dir
	capsCacheDirMu.Unlock()
	clearCapsMem()
	return func() {
		capsCacheDirMu.Lock()
		capsCacheDirOverride = prev
		capsCacheDirMu.Unlock()
		clearCapsMem()
	}
}

func clearCapsMem() {
	capsMem.mu.Lock()
	capsMem.byKey = make(map[string]*CapsRecord)
	capsMem.mu.Unlock()
}

func capsCacheDir() (string, error) {
	capsCacheDirMu.Lock()
	override := capsCacheDirOverride
	capsCacheDirMu.Unlock()
	if override != "" {
		return override, nil
	}
	return secrets.CapsDir()
}

func sanitizeCapsKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_")
	return replacer.Replace(key)
}

// CapsCacheKey is auth@tenant, or auth@tenant@platform:{tenantId} on platform.
func CapsCacheKey(authName, tenantName, surface, tenantID string) string {
	authName = strings.TrimSpace(authName)
	tenantName = strings.TrimSpace(tenantName)
	if authName == "" || tenantName == "" {
		return ""
	}
	key := authName + "@" + tenantName
	if strings.EqualFold(strings.TrimSpace(surface), SurfacePlatform) {
		tid := strings.TrimSpace(tenantID)
		if tid != "" {
			return key + "@platform:" + tid
		}
		return key + "@platform"
	}
	return key
}

func capsCachePath(cacheKey string) (string, error) {
	key := sanitizeCapsKey(cacheKey)
	if key == "" {
		return "", fmt.Errorf("empty caps cache key")
	}
	dir, err := capsCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

func capsUsable(expiry time.Time) bool {
	if expiry.IsZero() {
		return false
	}
	return time.Until(expiry) > 0
}

// LoadCaps returns the last-check record for auth×tenant×surface, or nil if missing/stale.
func LoadCaps(authName, tenantName, surface, tenantID string) *CapsRecord {
	key := CapsCacheKey(authName, tenantName, surface, tenantID)
	if key == "" {
		return nil
	}
	capsMem.mu.Lock()
	if rec, ok := capsMem.byKey[key]; ok {
		cp := *rec
		capsMem.mu.Unlock()
		if !capsUsable(cp.ExpiresAt) {
			InvalidateCaps(authName, tenantName, surface, tenantID)
			return nil
		}
		return &cp
	}
	capsMem.mu.Unlock()

	path, err := capsCachePath(key)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	rec, ok := parseDiskCapsFile(data)
	if !ok || rec == nil || !capsUsable(rec.ExpiresAt) {
		_ = os.Remove(path)
		return nil
	}
	capsMem.mu.Lock()
	capsMem.byKey[key] = rec
	capsMem.mu.Unlock()
	cp := *rec
	return &cp
}

func parseDiskCapsFile(data []byte) (*CapsRecord, bool) {
	var f diskCapsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, false
	}
	if f.V != capsDiskVersion || f.Payload == "" {
		return nil, false
	}
	plain, err := openCaps(f.Payload)
	if err != nil {
		return nil, false
	}
	var rec CapsRecord
	if err := json.Unmarshal(plain, &rec); err != nil {
		return nil, false
	}
	if rec.Live == "" {
		return nil, false
	}
	if rec.ExpiresAt.IsZero() && !f.Expiry.IsZero() {
		rec.ExpiresAt = f.Expiry
	}
	return &rec, true
}

// SaveCaps writes a last-check record. No-op for empty keys (command/direct auth).
func SaveCaps(rec CapsRecord) error {
	if rec.Auth == "" || rec.Tenant == "" {
		return nil
	}
	if rec.CheckedAt.IsZero() {
		rec.CheckedAt = time.Now().UTC()
	} else {
		rec.CheckedAt = rec.CheckedAt.UTC()
	}
	if rec.ExpiresAt.IsZero() {
		rec.ExpiresAt = rec.CheckedAt.Add(capsDefaultTTL)
	} else {
		rec.ExpiresAt = rec.ExpiresAt.UTC()
	}
	pathKey := CapsCacheKey(rec.Auth, rec.Tenant, rec.Surface, rec.TenantID)
	path, err := capsCachePath(pathKey)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	plain, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	sealed, err := sealCaps(plain)
	if err != nil {
		return fmt.Errorf("seal caps: %w", err)
	}
	f := diskCapsFile{V: capsDiskVersion, Payload: sealed, Expiry: rec.ExpiresAt}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	capsMem.mu.Lock()
	cp := rec
	capsMem.byKey[pathKey] = &cp
	capsMem.mu.Unlock()
	_, _ = CleanExpiredCapsCache()
	return nil
}

// InvalidateCaps deletes the last-check record for this cell (401 / stale secret).
func InvalidateCaps(authName, tenantName, surface, tenantID string) {
	key := CapsCacheKey(authName, tenantName, surface, tenantID)
	if key == "" {
		return
	}
	capsMem.mu.Lock()
	delete(capsMem.byKey, key)
	capsMem.mu.Unlock()
	path, err := capsCachePath(key)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

func invalidateCapsForSession(sess *Session) {
	if sess == nil {
		return
	}
	InvalidateCaps(sess.AuthName, sess.TenantName, sess.Surface, sess.Credential.EffectiveTenantID())
}

// CapsCacheCleanResult summarizes a caps/ directory sweep.
type CapsCacheCleanResult struct {
	Deleted int
	Kept    int
	Dir     string
}

// CleanExpiredCapsCache removes expired, empty, or undecodable caps files.
func CleanExpiredCapsCache() (CapsCacheCleanResult, error) {
	dir, err := capsCacheDir()
	if err != nil {
		return CapsCacheCleanResult{}, err
	}
	out := CapsCacheCleanResult{Dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			if os.Remove(path) == nil {
				out.Deleted++
			}
			continue
		}
		rec, ok := parseDiskCapsFile(data)
		if !ok || rec == nil || !capsUsable(rec.ExpiresAt) {
			if os.Remove(path) == nil {
				out.Deleted++
			}
			continue
		}
		out.Kept++
	}
	return out, nil
}

// ListCapsRecords loads every still-valid caps file (for auth authorization --all).
func ListCapsRecords() []CapsRecord {
	dir, err := capsCacheDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []CapsRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		rec, ok := parseDiskCapsFile(data)
		if !ok || rec == nil || !capsUsable(rec.ExpiresAt) {
			continue
		}
		out = append(out, *rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tenant != out[j].Tenant {
			return out[i].Tenant < out[j].Tenant
		}
		if out[i].Surface != out[j].Surface {
			return out[i].Surface < out[j].Surface
		}
		return out[i].Auth < out[j].Auth
	})
	return out
}

func liveStatusFromCaps(rec *CapsRecord) string {
	if rec == nil {
		return LiveNever
	}
	if rec.Live != "" {
		return rec.Live
	}
	return LiveNever
}

func capsTokenOK(rec *CapsRecord) bool {
	return rec != nil && rec.TokenOk
}

func capsTokenFail(rec *CapsRecord) bool {
	if rec == nil {
		return false
	}
	return rec.Live == LiveTokenFail || rec.Live == LiveSecretMissing
}

// Coarse Pro resource class → Jamf privilege names (classic + "in Jamf Pro API").
var resourceReadPrivileges = map[string][]string{
	"computers": {
		"Read Computers",
		"Read computer inventory in Jamf Pro API",
	},
	"policies": {
		"Read Policies",
		"Read policies in Jamf Pro API",
	},
	"scripts": {
		"Read Scripts",
		"Read scripts in Jamf Pro API",
	},
	"packages": {
		"Read Packages",
		"Read packages in Jamf Pro API",
	},
	"groups": {
		"Read Smart Computer Groups",
		"Read Static Computer Groups",
		"Read Computer Groups",
		"Read computer groups in Jamf Pro API",
	},
	"profiles": {
		"Read macOS Configuration Profiles",
		"Read OS X Configuration Profiles",
		"Read configuration profiles in Jamf Pro API",
	},
	"categories": {
		"Read Categories",
		"Read categories in Jamf Pro API",
	},
}

func resourceClassForPath(path string) string {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "categor"):
		return "categories"
	case strings.Contains(p, "package"):
		return "packages"
	case strings.Contains(p, "script"):
		return "scripts"
	case strings.Contains(p, "os-x-configuration-profile"),
		strings.Contains(p, "configuration-profile"),
		strings.Contains(p, "osxconfigurationprofile"),
		strings.Contains(p, "macosconfigurationprofile"):
		return "profiles"
	case strings.Contains(p, "computer-group"),
		strings.Contains(p, "computergroup"),
		strings.Contains(p, "smart-group"),
		strings.Contains(p, "static-group"):
		return "groups"
	case strings.Contains(p, "polic"):
		return "policies"
	case strings.Contains(p, "computer"):
		return "computers"
	default:
		return ""
	}
}

var activeResourceClass struct {
	mu    sync.RWMutex
	class string
}

func setResourceClass(class string) (restore func()) {
	activeResourceClass.mu.Lock()
	prev := activeResourceClass.class
	activeResourceClass.class = class
	activeResourceClass.mu.Unlock()
	return func() {
		activeResourceClass.mu.Lock()
		activeResourceClass.class = prev
		activeResourceClass.mu.Unlock()
	}
}

// ResourceClassPreference is the coarse Pro resource for the in-flight HTTP path.
func ResourceClassPreference() string {
	activeResourceClass.mu.RLock()
	defer activeResourceClass.mu.RUnlock()
	return activeResourceClass.class
}

func capsHasResourcePrivilege(rec *CapsRecord, resourceClass string) (known bool, has bool) {
	if rec == nil || resourceClass == "" {
		return false, false
	}
	set := strings.ToUpper(strings.TrimSpace(rec.PrivilegeSet))
	if set != "" && set != "CUSTOM" && set != "CUSTOMSET" {
		// ADMINISTRATOR / ALL / etc. — treat as full.
		return true, true
	}
	needed := resourceReadPrivileges[resourceClass]
	if len(needed) == 0 || len(rec.Privileges) == 0 {
		return false, false
	}
	have := map[string]bool{}
	for _, p := range rec.Privileges {
		have[strings.ToLower(strings.TrimSpace(p))] = true
	}
	for _, n := range needed {
		if have[strings.ToLower(n)] {
			return true, true
		}
	}
	return true, false
}

func privilegeNamesFromAuthPayload(info map[string]interface{}) []string {
	acct, _ := info["account"].(map[string]interface{})
	if acct == nil {
		return nil
	}
	bySite, ok := acct["privilegesBySite"].(map[string]interface{})
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, v := range bySite {
		list, ok := v.([]interface{})
		if !ok {
			continue
		}
		for _, item := range list {
			s, ok := item.(string)
			s = strings.TrimSpace(s)
			if !ok || s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
