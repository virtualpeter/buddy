package buddy

import (
	"encoding/json"
	"testing"
)

func TestPageOptsNormalize(t *testing.T) {
	o := PageOpts{Page: PageAll, PageSize: 0}.Normalize()
	if o.PageSize != DefaultPageSize {
		t.Fatalf("pageSize = %d", o.PageSize)
	}
	if o.Page != PageAll {
		t.Fatalf("page = %d", o.Page)
	}
	o2 := PageOpts{Page: 3, PageSize: 50}.Normalize()
	if o2.Page != 3 || o2.PageSize != 50 {
		t.Fatalf("got %+v", o2)
	}
}

func TestJamfPageEnvelopeUnmarshal(t *testing.T) {
	raw := []byte(`{"totalCount":2,"results":[{"id":"1"},{"id":"2"}]}`)
	var env jamfPageEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.TotalCount != 2 {
		t.Fatalf("total = %d", env.TotalCount)
	}
	var items []map[string]string
	if err := json.Unmarshal(env.Results, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d", len(items))
	}
}
