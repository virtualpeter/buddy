package buddy

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

var (
	policyCategoryBlockRe = regexp.MustCompile(`(?is)<category\s*/?>|<category>.*?</category>`)
	policyScriptBlockRe   = regexp.MustCompile(`(?is)<script>(.*?)</script>`)
	policyPackageBlockRe  = regexp.MustCompile(`(?is)<package>(.*?)</package>`)
	policyChildIDRe       = regexp.MustCompile(`(?i)<id>\s*([^<]*?)\s*</id>`)
	policyChildNameRe     = regexp.MustCompile(`(?i)<name>\s*([^<]*?)\s*</name>`)
)

// RemapPolicyDepsOnDest rewrites category, script, and package ids in prepared policy XML
// for the destination tenant. Same-session (srcCred == dstCred) keeps embedded ids.
// Missing script/package names fail hard; missing category becomes uncategorized.
func RemapPolicyDepsOnDest(srcCred, dstCred string, policyXML []byte, categoryOverride string, logger *slog.Logger) ([]byte, error) {
	same := strings.TrimSpace(srcCred) == strings.TrimSpace(dstCred)
	out := policyXML
	var err error
	out, err = RemapPolicyCategoryOnDest(dstCred, out, categoryOverride, same)
	if err != nil {
		return nil, err
	}
	if same {
		return out, nil
	}
	out, err = RemapPolicyScriptsByName(dstCred, out, logger)
	if err != nil {
		return nil, err
	}
	out, err = RemapPolicyPackagesByName(dstCred, out, logger)
	if err != nil {
		return nil, err
	}
	return out, nil
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
