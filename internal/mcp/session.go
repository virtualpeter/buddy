package mcp

import (
	"buddy/internal/buddy"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionIn is the shared optional tenant/surface/auth override on MCP tools.
type sessionIn struct {
	Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
	Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias for auth auto-pick"`
	Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional; prefer tenant=)"`
}

func (s sessionIn) overrides() buddy.SessionOverrides {
	return buddy.SessionOverrides{
		Tenant:  s.Tenant,
		Auth:    s.Auth,
		Surface: s.Surface,
	}
}

// withSession runs fn under SessionOverrides. authPin is the pin for buddy APIs
// (explicit Auth, else defaultAuth, else empty for auto-pick when tenant is set).
func withSession(defaultAuth string, in sessionIn, fn func(authPin string) (*mcpsdk.CallToolResult, any, error)) (*mcpsdk.CallToolResult, any, error) {
	var (
		res *mcpsdk.CallToolResult
		val any
	)
	err := buddy.WithSessionOverrides(in.overrides(), func() error {
		pin := in.Auth
		if pin == "" && in.Tenant == "" {
			pin = defaultAuth
		}
		var callErr error
		res, val, callErr = fn(pin)
		return callErr
	})
	if err != nil {
		return toolErr(err)
	}
	return res, val, nil
}
