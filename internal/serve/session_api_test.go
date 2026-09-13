package serve

import (
	"encoding/json"
	"testing"
)

func TestCreateSessionReqContextDualRead(t *testing.T) {
	var req createSessionReq
	if err := json.Unmarshal([]byte(`{"context":"sandbox","channel":"ide"}`), &req); err != nil {
		t.Fatal(err)
	}
	if !req.tenantSet || req.Tenant != "sandbox" {
		t.Fatalf("context: %+v", req)
	}

	req = createSessionReq{}
	if err := json.Unmarshal([]byte(`{"tenant":"prod"}`), &req); err != nil {
		t.Fatal(err)
	}
	if !req.tenantSet || req.Tenant != "prod" {
		t.Fatalf("tenant: %+v", req)
	}

	req = createSessionReq{}
	if err := json.Unmarshal([]byte(`{"tenant":"old","context":"new"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Tenant != "new" {
		t.Fatalf("context should win, got %q", req.Tenant)
	}
}
