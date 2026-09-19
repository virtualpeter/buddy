package serve

import (
	"net/http"
	"strings"
)

// defaultEmbed is set from serve.Options (--embed) for hosts like Buddy.app.
var defaultEmbed bool

// SetDefaultEmbed configures whether pages omit in-page nav/logo unless ?embed=0.
func SetDefaultEmbed(v bool) {
	defaultEmbed = v
}

// RequestEmbed reports whether the page should use embedded chrome (no logo /
// docs·ide·config nav). True when --embed was passed or ?embed=1; ?embed=0 opts out.
func RequestEmbed(r *http.Request) bool {
	if r != nil {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("embed")))
		switch q {
		case "0", "false", "no":
			return false
		case "1", "true", "yes":
			return true
		}
	}
	return defaultEmbed
}
