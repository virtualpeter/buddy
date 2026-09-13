package buddy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// PastedClientCreds is what buddy can extract from a Jamf API-client JSON
// download or a Platform curl sample.
type PastedClientCreds struct {
	ClientID     string
	ClientSecret string
	ClientName   string // optional auth-name hint (Jamf Pro JSON client_name)
	Type         string // oauth (instance) or platform
	Scope        string // platform: organization when a non-resolving api.jamfcloud.com sample is pasted; us host is inferred at cred add
	Region       string // platform gateway region (environment/tenant; org YAML has none)
	InstanceURL  string // instance origin when the blob is a Jamf Pro token curl
}

var (
	reHTTPURL   = regexp.MustCompile(`https?://[^\s'"\\]+`)
	reCurlURL   = regexp.MustCompile(`(?i)--url(?:=|\s+)(?:'([^']+)'|"([^"]+)"|(\S+))`)
	reCurlData  = regexp.MustCompile(`(?i)(?:--data(?:-urlencode|-raw)?|-d)(?:=|\s+)(?:'([^']*)'|"([^"]*)"|(\S+))`)
	reCodeFence = regexp.MustCompile("(?s)^```(?:json|bash|sh|shell|curl)?\\s*(.*)\\s*```$")
)

// ParsePastedClientCreds accepts Jamf Pro API-client JSON or a curl sample
// (Platform or instance token POST).
func ParsePastedClientCreds(raw string) (*PastedClientCreds, error) {
	s := normalizePastedClientBlob(raw)
	if s == "" {
		return nil, fmt.Errorf("empty paste — provide Jamf client JSON or curl")
	}
	if strings.HasPrefix(s, "{") {
		return parsePastedClientJSON(s)
	}
	return parsePastedClientCurl(s)
}

func normalizePastedClientBlob(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "\ufeff")
	if m := reCodeFence.FindStringSubmatch(s); len(m) == 2 {
		s = strings.TrimSpace(m[1])
	}
	s = strings.ReplaceAll(s, "\\\r\n", " ")
	s = strings.ReplaceAll(s, "\\\n", " ")
	return strings.TrimSpace(s)
}

func parsePastedClientJSON(s string) (*PastedClientCreds, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("paste JSON: %w", err)
	}
	out := &PastedClientCreds{
		ClientID:     firstPastedString(raw, "client_id", "clientId"),
		ClientSecret: firstPastedString(raw, "client_secret", "clientSecret"),
		ClientName:   firstPastedString(raw, "client_name", "clientName"),
		Type:         AuthTypeOAuth,
	}
	if u := firstPastedString(raw, "url", "token_url", "tokenUrl"); u != "" {
		applyPastedURL(out, u)
	}
	if out.ClientID == "" || out.ClientSecret == "" {
		return nil, fmt.Errorf("paste JSON needs client_id and client_secret")
	}
	return out, nil
}

func firstPastedString(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := raw[k].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func parsePastedClientCurl(s string) (*PastedClientCreds, error) {
	out := &PastedClientCreds{Type: AuthTypeOAuth}
	if u := firstCurlGroup(reCurlURL, s); u != "" {
		applyPastedURL(out, u)
	} else if u := reHTTPURL.FindString(s); u != "" {
		applyPastedURL(out, u)
	}
	form := map[string]string{}
	for _, m := range reCurlData.FindAllStringSubmatch(s, -1) {
		chunk := firstNonEmpty(m[1], m[2], m[3])
		mergeFormPairs(form, chunk)
	}
	out.ClientID = strings.TrimSpace(form["client_id"])
	out.ClientSecret = strings.TrimSpace(form["client_secret"])
	if out.ClientID == "" {
		out.ClientID = strings.TrimSpace(form["clientId"])
	}
	if out.ClientSecret == "" {
		out.ClientSecret = strings.TrimSpace(form["clientSecret"])
	}
	if out.ClientID == "" || out.ClientSecret == "" {
		return nil, fmt.Errorf("paste curl needs client_id and client_secret (--data)")
	}
	if out.Type == "" {
		out.Type = AuthTypeOAuth
	}
	return out, nil
}

func firstCurlGroup(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 4 {
		return ""
	}
	return strings.TrimRight(firstNonEmpty(m[1], m[2], m[3]), "\\")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func mergeFormPairs(dst map[string]string, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	for _, part := range strings.Split(raw, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if dec, err := url.QueryUnescape(v); err == nil {
			v = dec
		}
		if k != "" {
			dst[k] = v
		}
	}
}

func applyPastedURL(out *PastedClientCreds, raw string) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "\\")
	if LooksLikeOrgPlatformGateway(raw) {
		out.Type = AuthTypePlatform
		out.Scope = PlatformScopeOrganization
		return
	}
	if LooksLikePlatformGateway(raw) || LooksLikeRetiredPlatformGateway(raw) {
		out.Type = AuthTypePlatform
		if r := RegionFromGatewayURL(raw); r != "" {
			out.Region = r
		}
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return
	}
	out.Type = AuthTypeOAuth
	out.InstanceURL = u.Scheme + "://" + u.Host
}

func applyPastedClientCreds(opts *CredAddOpts, p *PastedClientCreds) {
	if opts == nil || p == nil {
		return
	}
	set := func(dst *string, src string) {
		if strings.TrimSpace(*dst) == "" && strings.TrimSpace(src) != "" {
			*dst = strings.TrimSpace(src)
		}
	}
	set(&opts.ClientID, p.ClientID)
	set(&opts.ClientSecret, p.ClientSecret)
	set(&opts.Type, p.Type)
	set(&opts.Scope, p.Scope)
	set(&opts.Region, p.Region)
	set(&opts.URL, p.InstanceURL)
	set(&opts.AuthName, p.ClientName)
}

func resolveCredAddPaste(opts *CredAddOpts) error {
	if opts == nil {
		return nil
	}
	blob := strings.TrimSpace(opts.PasteBlob)
	if blob == "" && opts.WantPaste {
		var err error
		blob, err = readPastedClientBlob(true, opts.SkipConfirm)
		if err != nil {
			return err
		}
	}
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	if blob == "" && interactive && !opts.WantPaste && opts.ClientID == "" && opts.ClientSecret == "" {
		var err error
		blob, err = readPastedClientBlob(false, false)
		if err != nil {
			return err
		}
	}
	blob = strings.TrimSpace(blob)
	if blob == "" {
		if opts.WantPaste {
			return fmt.Errorf("empty paste — provide Jamf client JSON or curl")
		}
		return nil
	}
	parsed, err := ParsePastedClientCreds(blob)
	if err != nil {
		return err
	}
	applyPastedClientCreds(opts, parsed)
	return nil
}

func readPastedClientBlob(required, skipConfirm bool) (string, error) {
	if StdinIsTerminal() {
		if skipConfirm {
			return "", fmt.Errorf("--paste with -y needs --paste=<json-or-curl> or piped stdin")
		}
		return promptPastedClientBlob(required)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func promptPastedClientBlob(required bool) (string, error) {
	if required {
		fmt.Fprintln(os.Stderr, "Paste Jamf API client JSON or curl (end with a blank line):")
	} else {
		fmt.Fprintln(os.Stderr, "Paste Jamf API client JSON or curl (end with a blank line), or press Enter to type fields:")
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var b strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	s := strings.TrimSpace(b.String())
	if required && s == "" {
		return "", fmt.Errorf("empty paste — provide Jamf client JSON or curl")
	}
	return s, nil
}
