package script

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestEncodeJamfBody(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("a"), starlark.MakeInt(1))
	b, ct, err := encodeJamfBody(d)
	if err != nil {
		t.Fatal(err)
	}
	if ct != "application/json" || string(b) != `{"a":1}` {
		t.Fatalf("got %q ct %q", b, ct)
	}
	b, ct, err = encodeJamfBody(starlark.String("<policy/>"))
	if err != nil || ct != "" || string(b) != "<policy/>" {
		t.Fatalf("str body: %q %q %v", b, ct, err)
	}
}

func TestJamfGetBuiltin(t *testing.T) {
	buddy.InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/packages" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"totalCount":1,"results":[{"id":"9"}]}`)
	}))
	defer srv.Close()

	buddy.SetActiveConfig(&buddy.Config{
		Cred: "t",
		Credentials: map[string]buddy.CredentialConfig{
			"t": {
				URL: srv.URL,
				Auth: buddy.AuthConfig{
					Type:       "command",
					RunCommand: "printf tok",
				},
			},
		},
	})
	t.Cleanup(func() { buddy.SetActiveConfig(nil) })

	thread, predeclared, err := newThread("t", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := `
r = jamf_get("/api/v1/packages")
ok = r["ok"]
status = r["value"]["status"]
body = r["value"]["body"]
`
	globals, err := starlark.ExecFile(thread, "jamf_get_test.star", src, predeclared)
	if err != nil {
		t.Fatal(err)
	}
	if globals["ok"] != starlark.True {
		t.Fatalf("ok=%v", globals["ok"])
	}
	if globals["status"] != starlark.MakeInt(200) {
		t.Fatalf("status=%v", globals["status"])
	}
	body, ok := globals["body"].(*starlark.Dict)
	if !ok {
		t.Fatalf("body type %T", globals["body"])
	}
	total, found, _ := body.Get(starlark.String("totalCount"))
	if !found || total != starlark.MakeInt(1) {
		t.Fatalf("body=%v", body)
	}
}

func TestJamfGetClassicDefaultAccept(t *testing.T) {
	buddy.InitLogger()
	var accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<policies><size>0</size></policies>`)
	}))
	defer srv.Close()

	buddy.SetActiveConfig(&buddy.Config{
		Cred: "t",
		Credentials: map[string]buddy.CredentialConfig{
			"t": {
				URL:  srv.URL,
				Auth: buddy.AuthConfig{Type: "command", RunCommand: "printf tok"},
			},
		},
	})
	t.Cleanup(func() { buddy.SetActiveConfig(nil) })

	thread, predeclared, err := newThread("t", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = starlark.ExecFile(thread, "classic.star", `
r = jamf_get("/JSSResource/policies")
`, predeclared)
	if err != nil {
		t.Fatal(err)
	}
	if accept != "application/xml" {
		t.Fatalf("Accept=%q", accept)
	}
}
