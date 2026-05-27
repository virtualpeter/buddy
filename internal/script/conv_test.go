package script

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestGoValueToStarlark_Nil(t *testing.T) {
	got, err := GoValueToStarlark(nil)
	if err != nil {
		t.Fatalf("GoValueToStarlark(nil): %v", err)
	}
	if got != starlark.None {
		t.Errorf("got %v, want None", got)
	}
}

func TestGoValueToStarlark_String(t *testing.T) {
	got, err := GoValueToStarlark("hello")
	if err != nil {
		t.Fatalf("GoValueToStarlark(string): %v", err)
	}
	if s, ok := starlark.AsString(got); !ok || s != "hello" {
		t.Errorf("got %v, want \"hello\"", got)
	}
}

func TestGoValueToStarlark_Int(t *testing.T) {
	got, err := GoValueToStarlark(42)
	if err != nil {
		t.Fatalf("GoValueToStarlark(int): %v", err)
	}
	n, ok := got.(starlark.Int)
	if !ok {
		t.Fatalf("got %T, want starlark.Int", got)
	}
	i, _ := n.Int64()
	if i != 42 {
		t.Errorf("got %d, want 42", i)
	}
}

func TestGoValueToStarlark_Int64(t *testing.T) {
	got, err := GoValueToStarlark(int64(100))
	if err != nil {
		t.Fatalf("GoValueToStarlark(int64): %v", err)
	}
	n, ok := got.(starlark.Int)
	if !ok {
		t.Fatalf("got %T, want starlark.Int", got)
	}
	i, _ := n.Int64()
	if i != 100 {
		t.Errorf("got %d, want 100", i)
	}
}

func TestGoValueToStarlark_Bool(t *testing.T) {
	got, err := GoValueToStarlark(true)
	if err != nil {
		t.Fatalf("GoValueToStarlark(bool): %v", err)
	}
	if b, ok := got.(starlark.Bool); !ok || b != starlark.True {
		t.Errorf("got %v, want True", got)
	}
}

func TestGoValueToStarlark_Float64Whole(t *testing.T) {
	// JSON decodes numbers as float64; whole numbers become starlark.Int
	got, err := GoValueToStarlark(3.0)
	if err != nil {
		t.Fatalf("GoValueToStarlark(3.0): %v", err)
	}
	n, ok := got.(starlark.Int)
	if !ok {
		t.Fatalf("got %T, want starlark.Int for whole float", got)
	}
	i, _ := n.Int64()
	if i != 3 {
		t.Errorf("got %d, want 3", i)
	}
}

func TestGoValueToStarlark_Float64Fractional(t *testing.T) {
	got, err := GoValueToStarlark(3.14)
	if err != nil {
		t.Fatalf("GoValueToStarlark(3.14): %v", err)
	}
	f, ok := got.(starlark.Float)
	if !ok {
		t.Fatalf("got %T, want starlark.Float", got)
	}
	if float64(f) != 3.14 {
		t.Errorf("got %v, want 3.14", f)
	}
}

func TestGoValueToStarlark_Slice(t *testing.T) {
	got, err := GoValueToStarlark([]interface{}{"a", 1, true})
	if err != nil {
		t.Fatalf("GoValueToStarlark(slice): %v", err)
	}
	l, ok := got.(*starlark.List)
	if !ok {
		t.Fatalf("got %T, want *starlark.List", got)
	}
	if l.Len() != 3 {
		t.Errorf("len = %d, want 3", l.Len())
	}
}

func TestGoValueToStarlark_Map(t *testing.T) {
	got, err := GoValueToStarlark(map[string]interface{}{
		"k": "v",
		"n": 42,
	})
	if err != nil {
		t.Fatalf("GoValueToStarlark(map): %v", err)
	}
	d, ok := got.(*starlark.Dict)
	if !ok {
		t.Fatalf("got %T, want *starlark.Dict", got)
	}
	if d.Len() != 2 {
		t.Errorf("len = %d, want 2", d.Len())
	}
}

func TestGoValueToStarlark_Nested(t *testing.T) {
	got, err := GoValueToStarlark(map[string]interface{}{
		"a": 1,
		"b": []interface{}{2, "x"},
		"c": map[string]interface{}{"nested": true},
	})
	if err != nil {
		t.Fatalf("GoValueToStarlark(nested): %v", err)
	}
	d, ok := got.(*starlark.Dict)
	if !ok {
		t.Fatalf("got %T, want *starlark.Dict", got)
	}
	if d.Len() != 3 {
		t.Errorf("len = %d, want 3", d.Len())
	}
}

func TestGoValueToStarlark_UnsupportedType(t *testing.T) {
	_, err := GoValueToStarlark(struct{}{})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported type for Starlark") {
		t.Errorf("error = %v", err)
	}
}

func TestStarlarkValueToGo_Dict(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("a"), starlark.MakeInt64(1))
	_ = d.SetKey(starlark.String("b"), starlark.String("two"))
	got, err := StarlarkValueToGo(d)
	if err != nil {
		t.Fatalf("StarlarkValueToGo(dict): %v", err)
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("got %T, want map", got)
	}
	if m["a"].(int64) != 1 || m["b"].(string) != "two" {
		t.Errorf("got %v", m)
	}
}

func TestStarlarkValueToGo_List(t *testing.T) {
	l := starlark.NewList([]starlark.Value{starlark.MakeInt64(1), starlark.String("x")})
	got, err := StarlarkValueToGo(l)
	if err != nil {
		t.Fatalf("StarlarkValueToGo(list): %v", err)
	}
	sl, ok := got.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", got)
	}
	if len(sl) != 2 {
		t.Errorf("len = %d, want 2", len(sl))
	}
}

func TestStarlarkValueToGo_String(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.String("hi"))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(string): %v", err)
	}
	if s, ok := got.(string); !ok || s != "hi" {
		t.Errorf("got %v, want \"hi\"", got)
	}
}

func TestStarlarkValueToGo_Int(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.MakeInt64(99))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(int): %v", err)
	}
	if i, ok := got.(int64); !ok || i != 99 {
		t.Errorf("got %v, want 99", got)
	}
}

func TestStarlarkValueToGo_IntPrefersInt64WhenFits(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.MakeUint64(42))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(MakeUint64(42)): %v", err)
	}
	if i, ok := got.(int64); !ok || i != 42 {
		t.Errorf("want int64(42), got %v (%T)", got, got)
	}
}

func TestStarlarkValueToGo_IntFallbackUint64BeyondInt64(t *testing.T) {
	const v uint64 = math.MaxUint64
	got, err := StarlarkValueToGo(starlark.MakeUint64(v))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(math.MaxUint64): %v", err)
	}
	if u, ok := got.(uint64); !ok || u != v {
		t.Errorf("want uint64(%d), got %v (%T)", v, got, got)
	}
}

func TestGoValueToStarlarkStarlarkValueToGo_LargeUint64RoundTrip(t *testing.T) {
	n := uint64(math.MaxInt64) + 123
	in := map[string]interface{}{"n": n}
	toStarlark, err := GoValueToStarlark(in)
	if err != nil {
		t.Fatalf("GoValueToStarlark: %v", err)
	}
	back, err := StarlarkValueToGo(toStarlark)
	if err != nil {
		t.Fatalf("StarlarkValueToGo: %v", err)
	}
	m, ok := back.(map[string]interface{})
	if !ok {
		t.Fatalf("got %T, want map", back)
	}
	if u, ok := m["n"].(uint64); !ok || u != n {
		t.Errorf(`m["n"] = %v (%T)`, m["n"], m["n"])
	}
}

func TestStarlarkValueToGo_Float(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.Float(1.5))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(float): %v", err)
	}
	if f, ok := got.(float64); !ok || f != 1.5 {
		t.Errorf("got %v, want 1.5", got)
	}
}

func TestStarlarkValueToGo_Bool(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.True)
	if err != nil {
		t.Fatalf("StarlarkValueToGo(bool): %v", err)
	}
	if b, ok := got.(bool); !ok || !b {
		t.Errorf("got %v, want true", got)
	}
}

func TestStarlarkValueToGo_None(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.None)
	if err != nil {
		t.Fatalf("StarlarkValueToGo(None): %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestStarlarkValueToGo_Bytes(t *testing.T) {
	got, err := StarlarkValueToGo(starlark.Bytes("abc"))
	if err != nil {
		t.Fatalf("StarlarkValueToGo(bytes): %v", err)
	}
	if b, ok := got.([]byte); !ok || string(b) != "abc" {
		t.Errorf("got %v, want []byte(\"abc\")", got)
	}
}

func TestStarlarkValueToGo_Unsupported(t *testing.T) {
	// starlark.Float is supported; use something like a set or builtin if we had one. Use a custom type by passing a tuple (tuple might be supported in Starlark but not in our switch).
	tup := starlark.Tuple{starlark.String("x")}
	_, err := StarlarkValueToGo(tup)
	if err == nil {
		t.Error("expected error for unsupported Starlark type")
	}
	if !strings.Contains(err.Error(), "unsupported Starlark type for Go") {
		t.Errorf("error = %v", err)
	}
}

func TestRoundTrip_GoToStarlarkToGo(t *testing.T) {
	in := map[string]interface{}{
		"a": 1,
		"b": []interface{}{2, "x"},
		"c": map[string]interface{}{"nested": true},
	}
	star, err := GoValueToStarlark(in)
	if err != nil {
		t.Fatalf("GoValueToStarlark: %v", err)
	}
	out, err := StarlarkValueToGo(star)
	if err != nil {
		t.Fatalf("StarlarkValueToGo: %v", err)
	}
	// Compare via JSON for simplicity (float vs int in JSON)
	jIn, _ := json.Marshal(in)
	jOut, _ := json.Marshal(out)
	if string(jIn) != string(jOut) {
		t.Errorf("round-trip mismatch: in %s vs out %s", jIn, jOut)
	}
}

func TestJSONDecodeToStarlark_Valid(t *testing.T) {
	got, err := JSONDecodeToStarlark(`{"a":1,"b":[2]}`)
	if err != nil {
		t.Fatalf("JSONDecodeToStarlark: %v", err)
	}
	d, ok := got.(*starlark.Dict)
	if !ok {
		t.Fatalf("got %T, want dict", got)
	}
	// a=1 (JSON number -> float64 -> starlark.Int), b=[2]
	if d.Len() != 2 {
		t.Errorf("len = %d, want 2", d.Len())
	}
}

func TestJSONDecodeToStarlark_Invalid(t *testing.T) {
	_, err := JSONDecodeToStarlark(`{invalid`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestStarlarkToJSON_RoundTrip(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("a"), starlark.MakeInt64(1))
	_ = d.SetKey(starlark.String("b"), starlark.NewList([]starlark.Value{starlark.MakeInt64(2)}))
	jsonStr, err := StarlarkToJSON(d)
	if err != nil {
		t.Fatalf("StarlarkToJSON: %v", err)
	}
	back, err := JSONDecodeToStarlark(jsonStr)
	if err != nil {
		t.Fatalf("JSONDecodeToStarlark: %v", err)
	}
	// Compare via canonical JSON — Dict.String reflects insertion order, but JSON
	// unmarshaling into map loses key order, so decoding + GoValueToStarlark yields
	// a dict whose repr may differ though the structure is identical.
	gotJSON, err := StarlarkToJSON(back)
	if err != nil {
		t.Fatalf("StarlarkToJSON(back): %v", err)
	}
	if gotJSON != jsonStr {
		t.Errorf("round-trip JSON: got %s, want %s", gotJSON, jsonStr)
	}
}

func TestPlistDecodeToStarlark_Minimal(t *testing.T) {
	plistStr := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>PayloadIdentifier</key><string>com.test</string></dict></plist>`
	got, err := PlistDecodeToStarlark(plistStr)
	if err != nil {
		t.Fatalf("PlistDecodeToStarlark: %v", err)
	}
	d, ok := got.(*starlark.Dict)
	if !ok {
		t.Fatalf("got %T, want dict", got)
	}
	val, found, _ := d.Get(starlark.String("PayloadIdentifier"))
	if !found {
		t.Fatal("PayloadIdentifier not found")
	}
	if s, ok := starlark.AsString(val); !ok || s != "com.test" {
		t.Errorf("PayloadIdentifier = %v", val)
	}
}

func TestStarlarkToPlist_ContainsKey(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("PayloadIdentifier"), starlark.String("com.example.app"))
	plistStr, err := StarlarkToPlist(d)
	if err != nil {
		t.Fatalf("StarlarkToPlist: %v", err)
	}
	if !strings.Contains(plistStr, "PayloadIdentifier") || !strings.Contains(plistStr, "com.example.app") {
		t.Errorf("plist should contain PayloadIdentifier and value, got %s", plistStr)
	}
}
