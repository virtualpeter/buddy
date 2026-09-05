package buddy

import (
	"strings"
	"testing"

	"buddy/internal/redact"
)

func TestFormatTraceBodyRedactsAccessToken(t *testing.T) {
	const secret = "super-secret-access-token-value"
	out := formatTraceBody([]byte(`{"access_token":"` + secret + `","expires_in":1800}`))
	if strings.Contains(out, secret) {
		t.Fatalf("trace body leaked access_token: %s", out)
	}
	if !strings.Contains(out, redact.Placeholder) {
		t.Fatalf("missing placeholder: %s", out)
	}
}

func TestRedactHeaderValueAuthorization(t *testing.T) {
	got := redactHeaderValue("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaa.bbb")
	if got != redact.Placeholder {
		t.Fatalf("got %q", got)
	}
}
