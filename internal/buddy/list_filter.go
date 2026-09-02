package buddy

import (
	"fmt"
	"strings"
)

// ListFilterMatch selects how a simple (non-RSQL) filter term is wrapped.
type ListFilterMatch int

const (
	// ListFilterContains wraps as field=="*term*" (default for name-like fields).
	ListFilterContains ListFilterMatch = iota
	// ListFilterEq wraps as field==term (exact; used for MDM status enums).
	ListFilterEq
)

// ListFilterSpec describes the default simple-term mapping for a list endpoint
// that accepts Jamf RSQL filter=.
type ListFilterSpec struct {
	DefaultField string
	Match        ListFilterMatch
}

// Per-resource defaults for NormalizeListFilter (server RSQL lists).
var (
	FilterSpecPackages     = ListFilterSpec{DefaultField: "packageName", Match: ListFilterContains}
	FilterSpecComputers    = ListFilterSpec{DefaultField: "general.name", Match: ListFilterContains}
	FilterSpecScripts      = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecCategories   = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecGroups       = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecMDM          = ListFilterSpec{DefaultField: "status", Match: ListFilterEq}
	FilterSpecDevices      = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecDeviceGroups = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecDDM          = ListFilterSpec{DefaultField: "name", Match: ListFilterContains}
	FilterSpecMobileGroups = ListFilterSpec{DefaultField: "groupName", Match: ListFilterContains}
	FilterSpecRoles        = ListFilterSpec{DefaultField: "displayName", Match: ListFilterContains}
	FilterSpecClients      = ListFilterSpec{DefaultField: "displayName", Match: ListFilterContains}
)

// isRSQLFilter reports whether s contains any RSQL comparison operator.
// Plain name fragments (e.g. "install") contain none of these.
func isRSQLFilter(s string) bool {
	for _, op := range []string{"==", "!=", "=lt=", "=gt=", "=le=", "=ge=", "=in=", "=out="} {
		if strings.Contains(s, op) {
			return true
		}
	}
	return false
}

// NormalizeListFilter applies the one-rule list filter UX:
//   - empty → "" (caller omits filter=; MDM still requires non-empty separately)
//   - contains an RSQL operator → pass through unchanged
//   - otherwise → wrap with spec.DefaultField (contains or eq)
func NormalizeListFilter(filter string, spec ListFilterSpec) (string, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return "", nil
	}
	if isRSQLFilter(filter) {
		return filter, nil
	}
	field := strings.TrimSpace(spec.DefaultField)
	if field == "" {
		return "", fmt.Errorf("list filter: DefaultField is required for simple terms")
	}
	switch spec.Match {
	case ListFilterEq:
		return RSQLEq(field, filter)
	default:
		return RSQLContains(field, filter)
	}
}
