package buddy

import (
	"fmt"
	"sort"
	"strings"
)

// orgLoc is where a workplace name sits in the nested orgs: tree.
type orgLoc struct {
	Kind string // organization | environment | instance | tenant
	Org  string
	Env  string // empty for organization
	Ten  string // set only for nested product tenants
}

func (l orgLoc) directChildren(orgs map[string]OrgConfig) []string {
	if orgs == nil {
		return nil
	}
	org, ok := orgs[l.Org]
	if !ok {
		return nil
	}
	switch l.Kind {
	case "organization":
		names := make([]string, 0, len(org.Environments))
		for n := range org.Environments {
			names = append(names, n)
		}
		sort.Strings(names)
		return names
	case "environment":
		env, ok := org.Environments[l.Env]
		if !ok {
			return nil
		}
		names := make([]string, 0, len(env.Tenants))
		for n := range env.Tenants {
			names = append(names, n)
		}
		sort.Strings(names)
		return names
	default:
		return nil
	}
}

func locateInOrgs(orgs map[string]OrgConfig, name string) (orgLoc, bool) {
	name = strings.TrimSpace(name)
	if name == "" || orgs == nil {
		return orgLoc{}, false
	}
	if _, ok := orgs[name]; ok {
		return orgLoc{Kind: "organization", Org: name}, true
	}
	for orgName, org := range orgs {
		for envName, env := range org.Environments {
			if envName == name {
				kind := "environment"
				if env.EffectiveID() == "" && env.EffectiveURL() != "" && len(env.Tenants) == 0 {
					kind = "instance"
				}
				return orgLoc{Kind: kind, Org: orgName, Env: envName}, true
			}
			for tenName := range env.Tenants {
				if tenName == name {
					return orgLoc{Kind: "tenant", Org: orgName, Env: envName, Ten: tenName}, true
				}
			}
		}
	}
	return orgLoc{}, false
}

func claimedOrgNames(cfg *Config) map[string]struct{} {
	taken := map[string]struct{}{}
	if cfg == nil {
		return taken
	}
	for n := range cfg.Tenants {
		taken[strings.TrimSpace(n)] = struct{}{}
	}
	for orgName, org := range cfg.Orgs {
		taken[strings.TrimSpace(orgName)] = struct{}{}
		for envName, env := range org.Environments {
			taken[strings.TrimSpace(envName)] = struct{}{}
			for tenName := range env.Tenants {
				taken[strings.TrimSpace(tenName)] = struct{}{}
			}
		}
	}
	delete(taken, "")
	return taken
}

func uniqueWorkplaceName(taken map[string]struct{}, preferred ...string) string {
	try := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if _, ok := taken[s]; !ok {
			return s
		}
		return ""
	}
	for _, p := range preferred {
		if s := try(p); s != "" {
			return s
		}
	}
	base := "org"
	if len(preferred) > 0 && strings.TrimSpace(preferred[0]) != "" {
		base = strings.TrimSpace(preferred[0])
	}
	for i := 2; i < 10000; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if _, ok := taken[cand]; !ok {
			return cand
		}
	}
	return base + "-x"
}

func pickOrInventOrgName(cfg *Config, avoid string) string {
	avoid = strings.TrimSpace(avoid)
	if cfg != nil && len(cfg.Orgs) == 1 {
		for n := range cfg.Orgs {
			if strings.TrimSpace(n) != avoid {
				return strings.TrimSpace(n)
			}
		}
	}
	var orgWP []string
	if cfg != nil {
		for n, t := range cfg.Tenants {
			if t.IsOrganizationWorkplace() && strings.TrimSpace(n) != avoid {
				orgWP = append(orgWP, strings.TrimSpace(n))
			}
		}
	}
	sort.Strings(orgWP)
	if len(cfg.Orgs) == 0 && len(orgWP) == 1 {
		return orgWP[0]
	}
	taken := claimedOrgNames(cfg)
	if avoid != "" {
		taken[avoid] = struct{}{}
	}
	return uniqueWorkplaceName(taken, DefaultTenant, "org")
}

func ensureOrg(cfg *Config, name string) {
	name = strings.TrimSpace(name)
	if cfg == nil || name == "" {
		return
	}
	if cfg.Orgs == nil {
		cfg.Orgs = map[string]OrgConfig{}
	}
	if _, ok := cfg.Orgs[name]; !ok {
		cfg.Orgs[name] = OrgConfig{}
	}
}

func foldFlatIntoOrgs(cfg *Config) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	implied, err := workplacesFromOrgs(cfg.Orgs)
	if err != nil {
		return false, err
	}
	var leftovers []string
	for name, t := range cfg.Tenants {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if src, ok := implied[name]; ok && workplaceIdentityEqual(t, src) {
			continue
		}
		leftovers = append(leftovers, name)
	}
	if len(leftovers) == 0 {
		return false, nil
	}
	sort.Slice(leftovers, func(i, j int) bool {
		ki := foldRank(cfg.Tenants[leftovers[i]])
		kj := foldRank(cfg.Tenants[leftovers[j]])
		if ki != kj {
			return ki < kj
		}
		return leftovers[i] < leftovers[j]
	})
	for _, name := range leftovers {
		t := cfg.Tenants[name]
		if err := upsertWorkplaceInOrgs(cfg, name, t); err != nil {
			return false, err
		}
	}
	if err := expandOrgs(cfg); err != nil {
		return false, err
	}
	return true, nil
}

func foldRank(t TenantConfig) int {
	switch t.WorkplaceKind() {
	case "organization":
		return 0
	case "environment":
		return 1
	case "tenant":
		return 2
	default:
		return 3
	}
}

func upsertWorkplaceInOrgs(cfg *Config, name string, t TenantConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("workplace name required")
	}
	if loc, ok := locateInOrgs(cfg.Orgs, name); ok {
		if sameOrgKind(loc.Kind, t) {
			return patchOrgNode(cfg, loc, t)
		}
		if kids := loc.directChildren(cfg.Orgs); len(kids) > 0 {
			return fmt.Errorf("cannot change kind of %s %q while it still has children (%s)", loc.Kind, name, strings.Join(kids, ", "))
		}
		removeOrgNode(cfg, loc)
	}
	return insertOrgNode(cfg, name, t)
}

func sameOrgKind(locKind string, t TenantConfig) bool {
	k := t.WorkplaceKind()
	switch locKind {
	case "organization":
		return k == "organization"
	case "instance":
		return k == "instance"
	case "environment":
		return k == "environment"
	case "tenant":
		return k == "tenant"
	default:
		return false
	}
}

func patchOrgNode(cfg *Config, loc orgLoc, t TenantConfig) error {
	org := cfg.Orgs[loc.Org]
	switch loc.Kind {
	case "organization":
		org.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		org.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		cfg.Orgs[loc.Org] = org
	case "instance":
		env := org.Environments[loc.Env]
		env.URL = strings.TrimRight(strings.TrimSpace(t.URL), "/")
		env.ID = ""
		env.EnvironmentID = ""
		env.Region = ""
		env.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		env.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		if org.Environments == nil {
			org.Environments = map[string]OrgEnvironmentConfig{}
		}
		org.Environments[loc.Env] = env
		cfg.Orgs[loc.Org] = org
	case "environment":
		env := org.Environments[loc.Env]
		env.ID = strings.TrimSpace(t.EnvironmentID)
		env.EnvironmentID = ""
		env.Region = strings.TrimSpace(t.Region)
		env.URL = ""
		env.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		env.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		if org.Environments == nil {
			org.Environments = map[string]OrgEnvironmentConfig{}
		}
		org.Environments[loc.Env] = env
		cfg.Orgs[loc.Org] = org
	case "tenant":
		env := org.Environments[loc.Env]
		if env.Tenants == nil {
			env.Tenants = map[string]OrgProductTenant{}
		}
		ten := env.Tenants[loc.Ten]
		ten.TenantID = strings.TrimSpace(t.TenantID)
		ten.URL = strings.TrimRight(strings.TrimSpace(t.URL), "/")
		ten.JamfPro = struct {
			URL      string `yaml:"url,omitempty" json:"url,omitempty"`
			TenantID string `yaml:"tenantId,omitempty" json:"tenantId,omitempty"`
		}{}
		ten.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		ten.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		env.Tenants[loc.Ten] = ten
		org.Environments[loc.Env] = env
		cfg.Orgs[loc.Org] = org
	}
	return nil
}

func insertOrgNode(cfg *Config, name string, t TenantConfig) error {
	if t.IsOrganizationWorkplace() {
		ensureOrg(cfg, name)
		org := cfg.Orgs[name]
		org.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		org.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		cfg.Orgs[name] = org
		return nil
	}
	parent := strings.TrimSpace(t.Org)
	if parent == "" || parent == name {
		parent = pickOrInventOrgName(cfg, name)
	}
	if parent == name {
		parent = uniqueWorkplaceName(claimedOrgNames(cfg), "org")
	}
	ensureOrg(cfg, parent)
	org := cfg.Orgs[parent]
	if org.Environments == nil {
		org.Environments = map[string]OrgEnvironmentConfig{}
	}

	kind := t.WorkplaceKind()
	if kind == "instance" || (!t.HasScopeID() && t.HasInstanceURL()) {
		env := org.Environments[name]
		env.URL = strings.TrimRight(strings.TrimSpace(t.URL), "/")
		env.ID = ""
		env.EnvironmentID = ""
		env.Region = ""
		env.Tenants = nil
		env.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		env.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		org.Environments[name] = env
		cfg.Orgs[parent] = org
		return nil
	}
	if kind == "environment" {
		env := org.Environments[name]
		env.ID = strings.TrimSpace(t.EnvironmentID)
		env.EnvironmentID = ""
		env.Region = strings.TrimSpace(t.Region)
		env.URL = ""
		env.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		env.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		org.Environments[name] = env
		cfg.Orgs[parent] = org
		return nil
	}

	envName := findEnvNameByEnvironmentID(cfg, parent, t.EffectiveEnvironmentID())
	if envName == "" || envName == name || envName == parent {
		taken := claimedOrgNames(cfg)
		taken[name] = struct{}{}
		taken[parent] = struct{}{}
		envName = uniqueWorkplaceName(taken, "env", strings.TrimSpace(t.Region), name+"-env")
	}
	env := org.Environments[envName]
	if eid := strings.TrimSpace(t.EnvironmentID); eid != "" {
		env.ID = eid
	}
	if r := strings.TrimSpace(t.Region); r != "" {
		env.Region = r
	}
	env.URL = ""
	if env.Tenants == nil {
		env.Tenants = map[string]OrgProductTenant{}
	}
	env.Tenants[name] = OrgProductTenant{
		TenantID:      strings.TrimSpace(t.TenantID),
		URL:           strings.TrimRight(strings.TrimSpace(t.URL), "/"),
		DenyBuiltins:  append([]string(nil), t.DenyBuiltins...),
		AllowBuiltins: append([]string(nil), t.AllowBuiltins...),
	}
	org.Environments[envName] = env
	cfg.Orgs[parent] = org
	return nil
}

func findEnvNameByEnvironmentID(cfg *Config, preferOrg, eid string) string {
	eid = strings.TrimSpace(eid)
	if eid == "" || cfg == nil {
		return ""
	}
	if preferOrg != "" {
		if org, ok := cfg.Orgs[preferOrg]; ok {
			for envName, env := range org.Environments {
				if env.EffectiveID() == eid {
					return envName
				}
			}
		}
	}
	for _, org := range cfg.Orgs {
		for envName, env := range org.Environments {
			if env.EffectiveID() == eid {
				return envName
			}
		}
	}
	for n, t := range cfg.Tenants {
		if t.WorkplaceKind() == "environment" && t.EffectiveEnvironmentID() == eid {
			return n
		}
	}
	return ""
}

func removeOrgNode(cfg *Config, loc orgLoc) {
	if cfg == nil || cfg.Orgs == nil {
		return
	}
	org, ok := cfg.Orgs[loc.Org]
	if !ok {
		return
	}
	switch loc.Kind {
	case "organization":
		delete(cfg.Orgs, loc.Org)
	case "environment", "instance":
		delete(org.Environments, loc.Env)
		cfg.Orgs[loc.Org] = org
	case "tenant":
		env := org.Environments[loc.Env]
		delete(env.Tenants, loc.Ten)
		org.Environments[loc.Env] = env
		cfg.Orgs[loc.Org] = org
	}
}

func removeWorkplaceFromOrgs(cfg *Config, name string) error {
	loc, ok := locateInOrgs(cfg.Orgs, name)
	if !ok {
		return nil
	}
	kids := loc.directChildren(cfg.Orgs)
	if len(kids) > 0 {
		label := "children"
		switch loc.Kind {
		case "organization":
			label = "environments"
		case "environment":
			label = "product tenants"
		}
		return fmt.Errorf("cannot delete %s %q: %d %s still under it (%s)", loc.Kind, name, len(kids), label, strings.Join(kids, ", "))
	}
	removeOrgNode(cfg, loc)
	return nil
}

func renameOrgKey(cfg *Config, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if oldName == "" || newName == "" {
		return fmt.Errorf("rename requires from and to names")
	}
	if oldName == newName {
		return nil
	}
	taken := claimedOrgNames(cfg)
	delete(taken, oldName)
	if _, ok := taken[newName]; ok {
		return fmt.Errorf("name %q already exists", newName)
	}
	loc, ok := locateInOrgs(cfg.Orgs, oldName)
	if !ok {
		return fmt.Errorf("workplace %q not found in orgs:", oldName)
	}
	org := cfg.Orgs[loc.Org]
	switch loc.Kind {
	case "organization":
		cfg.Orgs[newName] = org
		delete(cfg.Orgs, oldName)
	case "environment", "instance":
		org.Environments[newName] = org.Environments[loc.Env]
		delete(org.Environments, loc.Env)
		cfg.Orgs[loc.Org] = org
	case "tenant":
		env := org.Environments[loc.Env]
		env.Tenants[newName] = env.Tenants[loc.Ten]
		delete(env.Tenants, loc.Ten)
		org.Environments[loc.Env] = env
		cfg.Orgs[loc.Org] = org
	}
	return nil
}
