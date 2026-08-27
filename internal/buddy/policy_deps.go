package buddy

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Self-closing must require the slash. `<category\s*/?>` also matches the opening
	// `<category>` tag, which duplicated the block and left a stray `<id>` (Classic POST 400).
	policyCategoryBlockRe        = regexp.MustCompile(`(?is)<category\s*/>|<category>.*?</category>`)
	policyScriptBlockRe          = regexp.MustCompile(`(?is)<script>(.*?)</script>`)
	policyPackageBlockRe         = regexp.MustCompile(`(?is)<package>(.*?)</package>`)
	policyChildIDRe              = regexp.MustCompile(`(?i)<id>\s*([^<]*?)\s*</id>`)
	policyChildNameRe            = regexp.MustCompile(`(?i)<name>\s*([^<]*?)\s*</name>`)
	policySelfServiceIconBlockRe = regexp.MustCompile(`(?is)<self_service_icon\s*/>|<self_service_icon>.*?</self_service_icon>`)
)

// RemapPolicyDepsOnDest rewrites category, script, package, and Self Service icon
// ids in prepared policy XML for the destination tenant. Same-session
// (src.Equal(dst)) keeps embedded ids. Missing script/package names fail hard;
// missing category becomes uncategorized. Cross-session icons are re-uploaded
// (Jamf has no hash-register API); empty/missing icon is left unset.
func RemapPolicyDepsOnDest(src, dst SessionRef, policyXML []byte, categoryOverride string, logger *slog.Logger) ([]byte, error) {
	same := src.Equal(dst)
	var out []byte
	err := WithSessionRef(dst, func(pin string) error {
		var err error
		out, err = RemapPolicyCategoryOnDest(pin, policyXML, categoryOverride, same)
		if err != nil {
			return err
		}
		if same {
			return nil
		}
		out, err = RemapPolicyScriptsByName(pin, out, logger)
		if err != nil {
			return err
		}
		out, err = RemapPolicyPackagesByName(pin, out, logger)
		return err
	})
	if err != nil {
		return nil, err
	}
	if same {
		return out, nil
	}
	return RemapPolicyIconOnDest(src, dst, out, logger)
}

// RemapPolicyIconOnDest downloads the source Self Service icon and POSTs it on dest.
func RemapPolicyIconOnDest(src, dst SessionRef, policyXML []byte, logger *slog.Logger) ([]byte, error) {
	srcID := extractPolicySelfServiceIconID(policyXML)
	if srcID <= 0 {
		return policyXML, nil
	}
	var rec Icon
	var data []byte
	err := WithSessionRef(src, func(pin string) error {
		var err error
		rec, err = IconGet(pin, srcID)
		if err != nil {
			return fmt.Errorf("source icon %d: %w", srcID, err)
		}
		data, err = IconFetch(pin, rec)
		if err != nil {
			return fmt.Errorf("download source icon %d: %w", srcID, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(rec.Name)
	if filename == "" {
		filename = "icon.png"
	}
	var dest Icon
	err = WithSessionRef(dst, func(pin string) error {
		var upErr error
		dest, upErr = IconUploadBytes(pin, filename, data)
		if upErr != nil {
			return fmt.Errorf("upload icon %q to destination: %w", filename, upErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Debug("Remapped policy icon", "src_id", srcID, "dest_id", dest.ID, "name", filename)
	}
	return replacePolicySelfServiceIconID(policyXML, dest.ID), nil
}

func extractPolicySelfServiceIconID(policyXML []byte) int {
	m := policySelfServiceIconBlockRe.FindSubmatch(policyXML)
	if m == nil {
		return 0
	}
	block := m[0]
	im := policyChildIDRe.FindSubmatch(block)
	if len(im) < 2 {
		return 0
	}
	id, err := strconv.Atoi(strings.TrimSpace(string(im[1])))
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func replacePolicySelfServiceIconID(policyXML []byte, destID int) []byte {
	replacement := []byte(fmt.Sprintf("<self_service_icon><id>%d</id></self_service_icon>", destID))
	if policySelfServiceIconBlockRe.Match(policyXML) {
		return policySelfServiceIconBlockRe.ReplaceAll(policyXML, replacement)
	}
	return policyXML
}

// RemapPolicyCategoryOnDest replaces the general <category> block using ResolveCategoryOnDest.
func RemapPolicyCategoryOnDest(dstCred string, policyXML []byte, categoryOverride string, sameSession bool) ([]byte, error) {
	srcName, srcID := extractPolicyCategory(policyXML)
	keepID := ""
	if sameSession {
		keepID = srcID
	}
	resolved, err := ResolveCategoryOnDest(dstCred, srcName, categoryOverride, keepID)
	if err != nil {
		return nil, err
	}
	return replacePolicyCategoryBlock(policyXML, resolved)
}

func extractPolicyCategory(policyXML []byte) (name, id string) {
	m := policyCategoryBlockRe.FindSubmatch(policyXML)
	if m == nil {
		return "", ""
	}
	block := m[0]
	if nm := policyChildNameRe.FindSubmatch(block); len(nm) == 2 {
		name = strings.TrimSpace(XMLUnescape(string(nm[1])))
	}
	if im := policyChildIDRe.FindSubmatch(block); len(im) == 2 {
		id = strings.TrimSpace(string(im[1]))
	}
	return name, id
}

func replacePolicyCategoryBlock(policyXML []byte, cat CategoryResolved) ([]byte, error) {
	replacement := []byte(cat.CategoryClassicXML())
	if policyCategoryBlockRe.Match(policyXML) {
		return policyCategoryBlockRe.ReplaceAll(policyXML, replacement), nil
	}
	// Insert after general <name> when missing.
	nameEnd := bytesIndexNameEnd(policyXML)
	if nameEnd < 0 {
		return policyXML, nil
	}
	return append(append(append([]byte{}, policyXML[:nameEnd]...), '\n'), append(replacement, policyXML[nameEnd:]...)...), nil
}

func bytesIndexNameEnd(policyXML []byte) int {
	idx := strings.Index(string(policyXML), "</name>")
	if idx < 0 {
		return -1
	}
	return idx + len("</name>")
}

// RemapPolicyScriptsByName rewrites each <script><id> using destination ScriptGetID(name).
func RemapPolicyScriptsByName(dstCred string, policyXML []byte, logger *slog.Logger) ([]byte, error) {
	return remapNamedIDBlocks(policyXML, policyScriptBlockRe, "script", func(name string) (string, error) {
		return ScriptGetID(dstCred, name, logger)
	})
}

// RemapPolicyPackagesByName rewrites each <package><id> using destination PackageFindIDByExactName.
func RemapPolicyPackagesByName(dstCred string, policyXML []byte, logger *slog.Logger) ([]byte, error) {
	return remapNamedIDBlocks(policyXML, policyPackageBlockRe, "package", func(name string) (string, error) {
		return PackageFindIDByExactName(dstCred, name, logger)
	})
}

func remapNamedIDBlocks(policyXML []byte, blockRe *regexp.Regexp, kind string, resolve func(name string) (string, error)) ([]byte, error) {
	var missing []string
	out := blockRe.ReplaceAllFunc(policyXML, func(block []byte) []byte {
		nm := policyChildNameRe.FindSubmatch(block)
		if len(nm) < 2 {
			return block
		}
		name := strings.TrimSpace(XMLUnescape(string(nm[1])))
		if name == "" {
			return block
		}
		id, err := resolve(name)
		if err != nil || strings.TrimSpace(id) == "" {
			missing = append(missing, name)
			return block
		}
		// Normalize numeric id for Classic XML.
		id = strings.TrimSpace(id)
		if _, err := strconv.Atoi(id); err != nil {
			// keep as-is (string ids rare in Classic)
		}
		return policyChildIDRe.ReplaceAll(block, []byte("<id>"+id+"</id>"))
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("destination missing %s(s) by name: %s — migrate/copy them first, then retry",
			kind, strings.Join(uniqueStrings(missing), ", "))
	}
	return out, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
