package buddy

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// OrgConfig is one Jamf Account organization in optional nested orgs: YAML.
// id is the Account organization UUID for humans (and for correlating Account
// APIs such as licensing); it is never sent as an HTTP header. Enumerating
// Account environments / product tenants / integrations is still unpublished.
// Load expands this tree into the flat contexts: map (cfg.Tenants) session
// pick already uses.
type OrgConfig struct {
	ID           string                          `yaml:"id,omitempty" json:"id,omitempty"`
	Environments map[string]OrgEnvironmentConfig `yaml:"environments,omitempty" json:"environments,omitempty"`
}

// OrgEnvironmentConfig is an Account environment under an organization.
type OrgEnvironmentConfig struct {
	ID            string                      `yaml:"id,omitempty" json:"id,omitempty"`
	EnvironmentID string                      `yaml:"environmentId,omitempty" json:"environmentId,omitempty"`
	Region        string                      `yaml:"region,omitempty" json:"region,omitempty"`
	Tenants       map[string]OrgProductTenant `yaml:"tenants,omitempty" json:"tenants,omitempty"`
}

// EffectiveID returns id or environmentId.
func (e OrgEnvironmentConfig) EffectiveID() string {
	if s := strings.TrimSpace(e.ID); s != "" {
		return s
	}
	return strings.TrimSpace(e.EnvironmentID)
}

// OrgProductTenant is a Jamf product tenant (Jamf Pro, etc.) under an environment.
// Nested orgs.*.environments.*.tenants is Jamf’s product-tenant list — not the
// root dual-read tenants: map (that still means workplaces / contexts:).
type OrgProductTenant struct {
	TenantID string `yaml:"tenantId,omitempty" json:"tenantId,omitempty"`
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	JamfPro  struct {
		URL      string `yaml:"url,omitempty" json:"url,omitempty"`
		TenantID string `yaml:"tenantId,omitempty" json:"tenantId,omitempty"`
	} `yaml:"jamfpro,omitempty" json:"jamfpro,omitempty"`
}

// EffectiveTenantID returns tenantId or jamfpro.tenantId.
func (t OrgProductTenant) EffectiveTenantID() string {
	if s := strings.TrimSpace(t.TenantID); s != "" {
		return s
	}
	return strings.TrimSpace(t.JamfPro.TenantID)
}

// EffectiveURL returns url or jamfpro.url.
func (t OrgProductTenant) EffectiveURL() string {
	if s := strings.TrimSpace(t.URL); s != "" {
		return s
	}
	return strings.TrimSpace(t.JamfPro.URL)
}

// expandOrgs compiles optional orgs: into cfg.Tenants (overlay, same names).
// Org / environment / nested-tenant names share the workplace namespace.
func expandOrgs(cfg *Config) error {
	if cfg == nil || len(cfg.Orgs) == 0 {
		return nil
	}
	implied, err := workplacesFromOrgs(cfg.Orgs)
	if err != nil {
		return err
	}
	if cfg.Tenants == nil {
		cfg.Tenants = map[string]TenantConfig{}
	}
	for name, src := range implied {
		dst, ok := cfg.Tenants[name]
		if !ok {
			cfg.Tenants[name] = src
			continue
		}
		merged, err := overlayWorkplace(name, dst, src)
		if err != nil {
			return err
		}
		cfg.Tenants[name] = merged
	}
	return nil
}

func workplacesFromOrgs(orgs map[string]OrgConfig) (map[string]TenantConfig, error) {
	if len(orgs) == 0 {
		return nil, nil
	}
	out := map[string]TenantConfig{}
	claimed := map[string]string{}
	claim := func(name, kind, path string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("orgs: empty %s name (%s)", kind, path)
		}
		if prev, ok := claimed[name]; ok {
			return fmt.Errorf("orgs: name %q used twice (%s and %s)", name, prev, kind)
		}
		claimed[name] = kind
		return nil
	}
	for orgName, org := range orgs {
		if err := claim(orgName, "organization", "orgs"); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(orgName)] = TenantConfig{}
		for envName, env := range org.Environments {
			path := fmt.Sprintf("orgs.%s.environments.%s", orgName, envName)
			if err := claim(envName, "environment", path); err != nil {
				return nil, err
			}
			eid := env.EffectiveID()
			id := strings.TrimSpace(env.ID)
			alt := strings.TrimSpace(env.EnvironmentID)
			if id != "" && alt != "" && id != alt {
				return nil, fmt.Errorf("%s: id and environmentId differ", path)
			}
			if eid == "" {
				return nil, fmt.Errorf("%s: requires id (environmentId)", path)
			}
			region := strings.TrimSpace(env.Region)
			if region == "" {
				return nil, fmt.Errorf("%s: requires region", path)
			}
			if norm, err := NormalizePlatformRegion(region); err == nil {
				region = norm
			}
			out[strings.TrimSpace(envName)] = TenantConfig{
				Org:           strings.TrimSpace(orgName),
				Region:        region,
				EnvironmentID: eid,
			}
			for tenName, ten := range env.Tenants {
				tpath := path + ".tenants." + tenName
				if err := claim(tenName, "tenant", tpath); err != nil {
					return nil, err
				}
				tid := ten.EffectiveTenantID()
				flat := strings.TrimSpace(ten.TenantID)
				nested := strings.TrimSpace(ten.JamfPro.TenantID)
				if flat != "" && nested != "" && flat != nested {
					return nil, fmt.Errorf("%s: tenantId and jamfpro.tenantId differ", tpath)
				}
				if tid == "" {
					return nil, fmt.Errorf("%s: requires tenantId", tpath)
				}
				url := strings.TrimRight(ten.EffectiveURL(), "/")
				flatURL := strings.TrimRight(strings.TrimSpace(ten.URL), "/")
				nestedURL := strings.TrimRight(strings.TrimSpace(ten.JamfPro.URL), "/")
				if flatURL != "" && nestedURL != "" && flatURL != nestedURL {
					return nil, fmt.Errorf("%s: url and jamfpro.url differ", tpath)
				}
				out[strings.TrimSpace(tenName)] = TenantConfig{
					Org:           strings.TrimSpace(orgName),
					Region:        region,
					EnvironmentID: eid,
					TenantID:      tid,
					URL:           url,
				}
			}
		}
	}
	return out, nil
}

func overlayWorkplace(name string, dst, src TenantConfig) (TenantConfig, error) {
	name = strings.TrimSpace(name)
	if orgExpansion(src) && !dst.IsOrganizationWorkplace() {
		return TenantConfig{}, fmt.Errorf("orgs: workplace %q is an organization in orgs: but existing context is not an organization workplace", name)
	}
	if dst.IsOrganizationWorkplace() && !orgExpansion(src) {
		return TenantConfig{}, fmt.Errorf("orgs: workplace %q is an organization workplace in contexts: but orgs: expands a different object", name)
	}
	var err error
	dst.URL, err = mergeWorkplaceString(name, "url", dst.URL, src.URL)
	if err != nil {
		return TenantConfig{}, err
	}
	dst.Region, err = mergeWorkplaceString(name, "region", dst.Region, src.Region)
	if err != nil {
		return TenantConfig{}, err
	}
	dst.EnvironmentID, err = mergeWorkplaceString(name, "environment id", dst.EnvironmentID, src.EnvironmentID)
	if err != nil {
		return TenantConfig{}, err
	}
	dst.TenantID, err = mergeWorkplaceString(name, "tenant id", dst.TenantID, src.TenantID)
	if err != nil {
		return TenantConfig{}, err
	}
	dst.Org, err = mergeWorkplaceString(name, "org", dst.Org, src.Org)
	if err != nil {
		return TenantConfig{}, err
	}
	return dst, nil
}

func orgExpansion(t TenantConfig) bool {
	return strings.TrimSpace(t.Org) == "" &&
		strings.TrimSpace(t.Region) == "" &&
		strings.TrimSpace(t.URL) == "" &&
		strings.TrimSpace(t.EnvironmentID) == "" &&
		strings.TrimSpace(t.TenantID) == ""
}

func mergeWorkplaceString(name, field, a, b string) (string, error) {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if field == "url" {
		a = strings.TrimRight(a, "/")
		b = strings.TrimRight(b, "/")
	}
	if field == "region" {
		if na, err := NormalizePlatformRegion(a); err == nil && a != "" {
			a = na
		}
		if nb, err := NormalizePlatformRegion(b); err == nil && b != "" {
			b = nb
		}
	}
	if a == "" {
		return b, nil
	}
	if b == "" {
		return a, nil
	}
	if a == b {
		return a, nil
	}
	return "", fmt.Errorf("orgs: workplace %q conflict: %s %q vs %q", name, field, a, b)
}

func workplaceIdentityEqual(a, b TenantConfig) bool {
	normURL := func(s string) string {
		return strings.TrimRight(strings.TrimSpace(s), "/")
	}
	normRegion := func(s string) string {
		s = strings.TrimSpace(s)
		if n, err := NormalizePlatformRegion(s); err == nil && s != "" {
			return n
		}
		return s
	}
	return strings.TrimSpace(a.Org) == strings.TrimSpace(b.Org) &&
		normRegion(a.Region) == normRegion(b.Region) &&
		normURL(a.URL) == normURL(b.URL) &&
		strings.TrimSpace(a.EnvironmentID) == strings.TrimSpace(b.EnvironmentID) &&
		strings.TrimSpace(a.TenantID) == strings.TrimSpace(b.TenantID) &&
		stringSliceEqual(a.DenyBuiltins, b.DenyBuiltins) &&
		stringSliceEqual(a.AllowBuiltins, b.AllowBuiltins)
}

func cloneOrgConfig(o OrgConfig) OrgConfig {
	out := OrgConfig{ID: o.ID}
	if o.Environments == nil {
		return out
	}
	out.Environments = make(map[string]OrgEnvironmentConfig, len(o.Environments))
	for k, e := range o.Environments {
		out.Environments[k] = cloneOrgEnvironment(e)
	}
	return out
}

func cloneOrgEnvironment(e OrgEnvironmentConfig) OrgEnvironmentConfig {
	out := OrgEnvironmentConfig{
		ID:            e.ID,
		EnvironmentID: e.EnvironmentID,
		Region:        e.Region,
	}
	if e.Tenants == nil {
		return out
	}
	out.Tenants = make(map[string]OrgProductTenant, len(e.Tenants))
	for k, t := range e.Tenants {
		out.Tenants[k] = t
	}
	return out
}

func orgConfigEqual(a, b OrgConfig) bool {
	if strings.TrimSpace(a.ID) != strings.TrimSpace(b.ID) {
		return false
	}
	if len(a.Environments) != len(b.Environments) {
		return false
	}
	for name, ae := range a.Environments {
		be, ok := b.Environments[name]
		if !ok {
			return false
		}
		if !orgEnvironmentEqual(ae, be) {
			return false
		}
	}
	return true
}

func orgEnvironmentEqual(a, b OrgEnvironmentConfig) bool {
	if a.EffectiveID() != b.EffectiveID() {
		return false
	}
	ar, br := strings.TrimSpace(a.Region), strings.TrimSpace(b.Region)
	if na, err := NormalizePlatformRegion(ar); err == nil && ar != "" {
		ar = na
	}
	if nb, err := NormalizePlatformRegion(br); err == nil && br != "" {
		br = nb
	}
	if ar != br {
		return false
	}
	if len(a.Tenants) != len(b.Tenants) {
		return false
	}
	for name, at := range a.Tenants {
		bt, ok := b.Tenants[name]
		if !ok {
			return false
		}
		if at.EffectiveTenantID() != bt.EffectiveTenantID() {
			return false
		}
		au := strings.TrimRight(at.EffectiveURL(), "/")
		bu := strings.TrimRight(bt.EffectiveURL(), "/")
		if au != bu {
			return false
		}
	}
	return true
}

func marshalOrgsNode(orgs map[string]OrgConfig, existing []byte) *yaml.Node {
	if len(orgs) == 0 {
		return nil
	}
	orgSet := make(map[string]struct{}, len(orgs))
	for n := range orgs {
		orgSet[n] = struct{}{}
	}
	orgOrder := mapKeyOrder(yamlMapKeyOrder(existing, "orgs"), orgSet)
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, orgName := range orgOrder {
		org := orgs[orgName]
		entry := &yaml.Node{Kind: yaml.MappingNode}
		if id := strings.TrimSpace(org.ID); id != "" {
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id},
			)
		}
		if len(org.Environments) > 0 {
			envSet := make(map[string]struct{}, len(org.Environments))
			for n := range org.Environments {
				envSet[n] = struct{}{}
			}
			envPrefer := yamlNestedMapKeyOrder(existing, "orgs", orgName, "environments")
			envOrder := mapKeyOrder(envPrefer, envSet)
			envsNode := &yaml.Node{Kind: yaml.MappingNode}
			for _, envName := range envOrder {
				env := org.Environments[envName]
				envEntry := &yaml.Node{Kind: yaml.MappingNode}
				if eid := env.EffectiveID(); eid != "" {
					envEntry.Content = append(envEntry.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
						&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: eid},
					)
				}
				if r := strings.TrimSpace(env.Region); r != "" {
					if norm, err := NormalizePlatformRegion(r); err == nil {
						r = norm
					}
					envEntry.Content = append(envEntry.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "region"},
						&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r},
					)
				}
				if len(env.Tenants) > 0 {
					tenSet := make(map[string]struct{}, len(env.Tenants))
					for n := range env.Tenants {
						tenSet[n] = struct{}{}
					}
					tenPrefer := yamlNestedMapKeyOrder(existing, "orgs", orgName, "environments", envName, "tenants")
					tenOrder := mapKeyOrder(tenPrefer, tenSet)
					tensNode := &yaml.Node{Kind: yaml.MappingNode}
					for _, tenName := range tenOrder {
						ten := env.Tenants[tenName]
						tenEntry := &yaml.Node{Kind: yaml.MappingNode}
						if tid := ten.EffectiveTenantID(); tid != "" {
							tenEntry.Content = append(tenEntry.Content,
								&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenantId"},
								&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tid},
							)
						}
						if u := strings.TrimRight(ten.EffectiveURL(), "/"); u != "" {
							tenEntry.Content = append(tenEntry.Content,
								&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "url"},
								&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: u},
							)
						}
						tensNode.Content = append(tensNode.Content,
							&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tenName},
							tenEntry,
						)
					}
					envEntry.Content = append(envEntry.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenants"},
						tensNode,
					)
				}
				envsNode.Content = append(envsNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: envName},
					envEntry,
				)
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "environments"},
				envsNode,
			)
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: orgName},
			entry,
		)
	}
	return root
}

func yamlNestedMapKeyOrder(data []byte, path ...string) []string {
	if len(path) == 0 {
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	cur := root
	for _, key := range path {
		if cur == nil || cur.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(cur.Content); i += 2 {
			if cur.Content[i].Value == key {
				next = cur.Content[i+1]
				break
			}
		}
		cur = next
	}
	if cur == nil || cur.Kind != yaml.MappingNode {
		return nil
	}
	names := make([]string, 0, len(cur.Content)/2)
	for j := 0; j+1 < len(cur.Content); j += 2 {
		names = append(names, cur.Content[j].Value)
	}
	return names
}
