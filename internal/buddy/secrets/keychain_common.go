package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	keychainServiceCLI = "buddy"
	// Matches the default codesign identifier (BUDDY_CODESIGN_IDENTIFIER).
	// Hardcoded: a custom identifier at sign time does not change this string.
	keychainServiceSigned = "virtualpete.buddy.cli"
	keychainLabelSigned   = "Buddy"
	// security -i truncates around this size (same limit go-keyring documents).
	securityInteractiveMax = 4096
)

func keychainAccount(credName string, kind Kind) string {
	return fmt.Sprintf("%s/%s", credName, kind)
}

// BuddyKeychainServices is the Keychain generic-password services buddy uses.
// CLI builds store under "buddy"; the signed binary uses "virtualpete.buddy.cli".
func BuddyKeychainServices() []string {
	return []string{keychainServiceCLI, keychainServiceSigned}
}

// securityQuote wraps s in single quotes for security(1) -i.
func securityQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func securityAddInteractiveLine(service, account, value string) (string, error) {
	line := fmt.Sprintf("add-generic-password -U -s %s -a %s -w %s\n",
		securityQuote(service), securityQuote(account), securityQuote(value))
	if len(line) > securityInteractiveMax {
		return "", fmt.Errorf("keychain store: secret too large for security -i (%d > %d bytes)", len(line), securityInteractiveMax)
	}
	return line, nil
}

// securityAddInteractiveCmd builds `security -i` with the add command on stdin.
// The secret is never placed in argv.
func securityAddInteractiveCmd(service, account, value string) (*exec.Cmd, error) {
	line, err := securityAddInteractiveLine(service, account, value)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("security", "-i")
	cmd.Stdin = strings.NewReader(line)
	return cmd, nil
}

// parseKeychainDump extracts generic-password accounts for service "buddy".
func parseKeychainDump(dump string) []Entry {
	return parseKeychainDumpForService(dump, keychainServiceCLI)
}

// parseKeychainDumpForService extracts generic-password accounts for service.
func parseKeychainDumpForService(dump, service string) []Entry {
	var out []Entry
	seen := map[string]struct{}{}
	var acct string
	var isService bool
	inGenp := false
	svcBlob := fmt.Sprintf(`"svce"<blob>="%s"`, service)
	svcHex := fmt.Sprintf(`0x00000007 <blob>="%s"`, service)
	flush := func() {
		if inGenp && isService && acct != "" {
			parts := strings.SplitN(acct, "/", 2)
			if len(parts) == 2 {
				if kind, err := ParseKind(parts[1]); err == nil {
					key := parts[0] + "/" + kind.String()
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						out = append(out, Entry{AuthName: parts[0], Kind: kind})
					}
				}
			}
		}
		acct = ""
		isService = false
		inGenp = false
	}
	for _, line := range strings.Split(dump, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "class:") {
			flush()
			inGenp = strings.Contains(trim, `"genp"`)
			continue
		}
		if !inGenp {
			continue
		}
		if strings.Contains(trim, svcBlob) || strings.Contains(trim, svcHex) {
			isService = true
		}
		const acctPrefix = `"acct"<blob>="`
		if i := strings.Index(trim, acctPrefix); i >= 0 {
			rest := trim[i+len(acctPrefix):]
			if j := strings.Index(rest, `"`); j >= 0 {
				acct = rest[:j]
			}
		}
	}
	flush()
	return out
}
