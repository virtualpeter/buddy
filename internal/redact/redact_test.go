package redact

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

const sampleJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"

func TestStringRedactsJSONAccessToken(t *testing.T) {
	in := `{"access_token":"super-secret-access-token-value","expires_in":1800,"token_type":"Bearer"}`
	out := String(in)
	if strings.Contains(out, "super-secret-access-token-value") {
		t.Fatalf("leaked access_token: %s", out)
	}
	if !strings.Contains(out, `"token_type":"Bearer"`) {
		t.Fatalf("token_type should stay: %s", out)
	}
	if !strings.Contains(out, Placeholder) {
		t.Fatalf("missing placeholder: %s", out)
	}
}

func TestStringRedactsJamfTokenField(t *testing.T) {
	in := `{"token":"tok-from-keep-alive","expires":"2026-01-01T00:00:00.000Z"}`
	out := String(in)
	if strings.Contains(out, "tok-from-keep-alive") {
		t.Fatalf("leaked token: %s", out)
	}
}

func TestStringRedactsFormClientSecret(t *testing.T) {
	in := "grant_type=client_credentials&client_id=cid&client_secret=s3cret-value"
	out := String(in)
	if strings.Contains(out, "s3cret-value") {
		t.Fatalf("leaked client_secret: %s", out)
	}
	if !strings.Contains(out, "client_id=cid") {
		t.Fatalf("client_id should stay: %s", out)
	}
}

func TestStringRedactsQueryAccessToken(t *testing.T) {
	in := "https://example.jamfcloud.com/callback?access_token=opaquetokenvalue1234&state=1"
	out := String(in)
	if strings.Contains(out, "opaquetokenvalue1234") {
		t.Fatalf("leaked query token: %s", out)
	}
}

func TestStringRedactsBearerAndJWT(t *testing.T) {
	in := "Authorization: Bearer " + sampleJWT
	out := String(in)
	if strings.Contains(out, sampleJWT) || strings.Contains(out, "eyJhbGci") {
		t.Fatalf("leaked JWT: %s", out)
	}
}

func TestStringLeavesTokenAcquired(t *testing.T) {
	in := "Bearer token acquired; probe not permitted"
	out := String(in)
	if out != in {
		t.Fatalf("got %q", out)
	}
}

func TestStringRedactsXMLToken(t *testing.T) {
	in := `<token>tok-from-xml</token>`
	out := String(in)
	if strings.Contains(out, "tok-from-xml") {
		t.Fatalf("leaked xml token: %s", out)
	}
}

func TestStringIdempotent(t *testing.T) {
	in := `{"access_token":"` + sampleJWT + `"}`
	once := String(in)
	twice := String(once)
	if once != twice {
		t.Fatalf("not idempotent:\n%s\n%s", once, twice)
	}
}

func TestWrapHandlerRedactsMessageAndAttr(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(WrapHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.Debug("got " + sampleJWT)
	log.Debug("oauth", "access_token", "super-secret-access-token-value")
	out := buf.String()
	if strings.Contains(out, sampleJWT) || strings.Contains(out, "super-secret-access-token-value") {
		t.Fatalf("handler leaked: %s", out)
	}
}
