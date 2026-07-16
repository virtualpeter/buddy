package buddy

import (
	"strings"
	"testing"
	"time"
)

func TestEscapeAndQuoteRSQLString(t *testing.T) {
	if got := EscapeRSQLString(`a\b"c`); got != `a\\b\"c` {
		t.Fatalf("EscapeRSQLString = %q", got)
	}
	if got := QuoteRSQLString(`hi"there`); got != `"hi\"there"` {
		t.Fatalf("QuoteRSQLString = %q", got)
	}
}

func TestFormatRSQLDateTime(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := FormatRSQLDateTime(ts); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("got %q", got)
	}
	// Non-UTC input is converted.
	loc := time.FixedZone("UTC+10", 10*3600)
	ts2 := time.Date(2024, 1, 2, 13, 4, 5, 0, loc)
	if got := FormatRSQLDateTime(ts2); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("got %q", got)
	}
}

func TestRSQLDateTime(t *testing.T) {
	got, err := RSQLDateTime(2024, 6, 15, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2024-06-15T00:00:00Z" {
		t.Fatalf("got %q", got)
	}
	if _, err := RSQLDateTime(2024, 2, 31, 0, 0, 0); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestRSQLCompareBuilders(t *testing.T) {
	eq, err := RSQLEq("packageName", "Chrome")
	if err != nil {
		t.Fatal(err)
	}
	if eq != `packageName=="Chrome"` {
		t.Fatalf("eq = %q", eq)
	}
	ge, err := RSQLGe("dateCompleted", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if ge != "dateCompleted=ge=2024-01-01T00:00:00Z" {
		t.Fatalf("ge = %q", ge)
	}
	idEq, err := RSQLEq("id", int64(42))
	if err != nil {
		t.Fatal(err)
	}
	if idEq != "id==42" {
		t.Fatalf("idEq = %q", idEq)
	}
}

func TestRSQLContains(t *testing.T) {
	got, err := RSQLContains("general.name", `lab "a"`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `general.name=="*lab \"a\"*"` {
		t.Fatalf("got %q", got)
	}
}

func TestRSQLIn(t *testing.T) {
	got, err := RSQLIn("id", []interface{}{int64(1), int64(2), "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `id=in=(1,2,"x")` {
		t.Fatalf("got %q", got)
	}
}

func TestRSQLAndOr(t *testing.T) {
	a, err := RSQLAnd([]string{`status=="Pending"`, `clientType=="COMPUTER"`})
	if err != nil {
		t.Fatal(err)
	}
	if a != `status=="Pending";clientType=="COMPUTER"` {
		t.Fatalf("and = %q", a)
	}
	o, err := RSQLOr([]string{`name=="a"`, `name=="b"`})
	if err != nil {
		t.Fatal(err)
	}
	if o != `name=="a",name=="b"` {
		t.Fatalf("or = %q", o)
	}
	if _, err := RSQLAnd(nil); err == nil {
		t.Fatal("expected error for empty and")
	}
}

func TestRSQLDaysAgoShape(t *testing.T) {
	got := RSQLDaysAgo(0)
	if !strings.HasSuffix(got, "Z") || !strings.Contains(got, "T") {
		t.Fatalf("unexpected format %q", got)
	}
}
