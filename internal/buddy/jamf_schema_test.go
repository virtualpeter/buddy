package buddy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testOpenAPI = `{
  "openapi": "3.0.1",
  "info": {"title": "Jamf Pro API", "version": "11"},
  "paths": {
    "/v1/packages": {
      "get": {"summary": "List packages", "operationId": "getPackages", "tags": ["packages"]},
      "post": {"summary": "Create package", "operationId": "createPackage", "tags": ["packages"]}
    },
    "/v1/scripts": {
      "get": {"summary": "List scripts", "operationId": "getScripts", "tags": ["scripts"]}
    }
  }
}`

const testSwagger = `swagger: "2.0"
info:
  title: Classic API
  version: production
paths:
  /policies:
    get:
      summary: Finds all policies
      operationId: findPolicies
      tags: [policies]
  /policies/id/{id}:
    get:
      summary: Finds policies by id
      operationId: findPoliciesById
      tags: [policies]
    delete:
      summary: Deletes a policy by id
      operationId: deletePolicyById
      tags: [policies]
`

func TestResolveSchemaAPI(t *testing.T) {
	name, path, err := ResolveSchemaAPI("pro")
	if err != nil || name != "pro" || path != JamfSchemaProPath {
		t.Fatalf("%s %s %v", name, path, err)
	}
	name, path, err = ResolveSchemaAPI("classic")
	if err != nil || name != "classic" || path != JamfSchemaClassicPath {
		t.Fatalf("%s %s %v", name, path, err)
	}
	name, path, err = ResolveSchemaAPI("both")
	if err != nil || name != "both" || path != "" {
		t.Fatalf("%s %s %v", name, path, err)
	}
	apis, err := SchemaAPIList("all")
	if err != nil || len(apis) != 2 || apis[0] != "pro" || apis[1] != "classic" {
		t.Fatalf("%v %v", apis, err)
	}
	if _, _, err := ResolveSchemaAPI("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestListSchemaOperationsFilter(t *testing.T) {
	doc, err := parseSchemaDocument([]byte(testOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	ops := listSchemaOperations(doc, "packages get")
	if len(ops) != 1 || ops[0].Method != "get" || ops[0].Path != "/v1/packages" {
		t.Fatalf("%+v", ops)
	}
	ops = listSchemaOperations(doc, "")
	if len(ops) != 3 {
		t.Fatalf("count %d", len(ops))
	}
}

func TestFetchJamfSchemaClassic(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != JamfSchemaClassicPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = io.WriteString(w, testSwagger)
	}))
	defer srv.Close()

	SetActiveConfig(&Config{
		Cred: "t",
		Credentials: map[string]CredentialConfig{
			"t": {URL: srv.URL, Auth: AuthConfig{Type: "command", RunCommand: "printf tok"}},
		},
	})
	t.Cleanup(func() { SetActiveConfig(nil) })

	res, err := FetchJamfSchema("t", "classic", "delete policy", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.API != "classic" || res.Count != 1 {
		t.Fatalf("%+v", res)
	}
	if res.Operations[0].OperationID != "deletePolicyById" {
		t.Fatalf("%+v", res.Operations[0])
	}

	full, err := FetchJamfSchema("t", "classic", "policies id", true)
	if err != nil {
		t.Fatal(err)
	}
	if full.Document == nil {
		t.Fatal("expected document")
	}
	paths, _ := full.Document["paths"].(map[string]interface{})
	if _, ok := paths["/policies/id/{id}"]; !ok {
		t.Fatalf("paths=%v", paths)
	}
	if _, ok := paths["/policies"]; ok {
		t.Fatal("unfiltered path should be removed")
	}
}

func TestFetchJamfSchemaPro(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != JamfSchemaProPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, testOpenAPI)
	}))
	defer srv.Close()

	SetActiveConfig(&Config{
		Cred: "t",
		Credentials: map[string]CredentialConfig{
			"t": {URL: srv.URL, Auth: AuthConfig{Type: "command", RunCommand: "printf tok"}},
		},
	})
	t.Cleanup(func() { SetActiveConfig(nil) })

	res, err := FetchJamfSchema("t", "pro", "scripts", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Jamf Pro API" || res.Count != 1 {
		t.Fatalf("%+v", res)
	}
	if res.Operations[0].API != "pro" {
		t.Fatalf("api field %q", res.Operations[0].API)
	}
}

func TestFetchJamfSchemaBoth(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case JamfSchemaProPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, testOpenAPI)
		case JamfSchemaClassicPath:
			w.Header().Set("Content-Type", "application/yaml")
			_, _ = io.WriteString(w, testSwagger)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	SetActiveConfig(&Config{
		Cred: "t",
		Credentials: map[string]CredentialConfig{
			"t": {URL: srv.URL, Auth: AuthConfig{Type: "command", RunCommand: "printf tok"}},
		},
	})
	t.Cleanup(func() { SetActiveConfig(nil) })

	res, err := FetchJamfSchema("t", "both", "policy", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.API != "both" || res.Pro == nil || res.Classic == nil {
		t.Fatalf("%+v", res)
	}
	// Classic has policy ops; pro fixture has none for "policy".
	if res.Count < 1 {
		t.Fatalf("count %d", res.Count)
	}
	sawClassic := false
	for _, op := range res.Operations {
		if op.API == "classic" {
			sawClassic = true
		}
	}
	if !sawClassic {
		t.Fatalf("operations=%+v", res.Operations)
	}
}
