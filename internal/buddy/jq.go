package buddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/itchyny/gojq"
)

// jqTimeout caps query execution so infinite iterators cannot hang MCP/CLI.
var jqTimeout = 5 * time.Second

func jqNormalizeInput(input any) (any, error) {
	if input == nil {
		return nil, nil
	}
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("jq: encode input: %w", err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("jq: decode input: %w", err)
	}
	return v, nil
}

func jqNormalizeValue(v any) (any, error) {
	b, err := gojq.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("jq: encode result: %w", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("jq: decode result: %w", err)
	}
	return out, nil
}

func collapseJQStream(stream []any) any {
	switch len(stream) {
	case 0:
		return []any{}
	case 1:
		return stream[0]
	default:
		return stream
	}
}

func jqIterError(err error, ctx context.Context) (halt bool, out error) {
	if herr, ok := err.(*gojq.HaltError); ok && herr.Value() == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, fmt.Errorf("jq: timed out")
	}
	return false, fmt.Errorf("jq: %w", err)
}

// ApplyJQStream runs a jq expression and returns every iterator value, in order.
func ApplyJQStream(input any, expr string) ([]any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("jq: expression is required")
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	in, err := jqNormalizeInput(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), jqTimeout)
	defer cancel()
	iter := query.RunWithContext(ctx, in)
	out := make([]any, 0)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			halt, e := jqIterError(err, ctx)
			if e != nil {
				return nil, e
			}
			if halt {
				break
			}
			continue
		}
		nv, err := jqNormalizeValue(v)
		if err != nil {
			return nil, err
		}
		out = append(out, nv)
	}
	return out, nil
}

// ApplyJQ runs a jq expression and collapses the stream: 0 values → empty list,
// 1 value → that value, N values → list.
func ApplyJQ(input any, expr string) (any, error) {
	stream, err := ApplyJQStream(input, expr)
	if err != nil {
		return nil, err
	}
	return collapseJQStream(stream), nil
}

func printJQRaw(w io.Writer, stream []any) error {
	for _, v := range stream {
		if s, ok := v.(string); ok {
			if _, err := fmt.Fprintln(w, s); err != nil {
				return err
			}
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return err
		}
	}
	return nil
}

func searchRowsToAny(rows []map[string]interface{}) []any {
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r
	}
	return out
}

func jqValueAsRows(v any) ([]map[string]interface{}, bool) {
	if m, ok := jsonAsMap(v); ok {
		return []map[string]interface{}{m}, true
	}
	switch t := v.(type) {
	case []any:
		rows := make([]map[string]interface{}, 0, len(t))
		for _, el := range t {
			m, ok := jsonAsMap(el)
			if !ok {
				return nil, false
			}
			rows = append(rows, m)
		}
		return rows, true
	case []map[string]interface{}:
		return t, true
	default:
		return nil, false
	}
}
