package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const (
	uriBenchmarks = "/api/compliance-benchmarks/v1/benchmarks"
	uriBaselines  = "/api/compliance-benchmarks/v1/baselines"
	uriCBRules    = "/api/compliance-benchmarks/v1/rules"
)

// BenchmarkListValues lists tenant compliance benchmarks.
func BenchmarkListValues(credName string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, uriBenchmarks)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse benchmarks: %w", err)
	}
	return out, nil
}

// BenchmarkList prints benchmarks JSON.
func BenchmarkList(credName string, logger *slog.Logger) error {
	out, err := BenchmarkListValues(credName, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// BenchmarkGetValue fetches one benchmark by id.
func BenchmarkGetValue(credName, benchmarkID string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(benchmarkID) == "" {
		return nil, fmt.Errorf("benchmark-id is required")
	}
	body, err := jamfGET(credName, uriBenchmarks+"/"+url.PathEscape(benchmarkID))
	if err != nil {
		return nil, err
	}
	out, err := decodeJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("parse benchmark: %w", err)
	}
	return out, nil
}

// BenchmarkGet prints one benchmark as JSON.
func BenchmarkGet(credName, benchmarkID string, logger *slog.Logger) error {
	out, err := BenchmarkGetValue(credName, benchmarkID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// BenchmarkCreateValue creates a benchmark from JSON.
func BenchmarkCreateValue(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformMutate(credName); err != nil {
		return nil, err
	}
	if len(bodyJSON) == 0 {
		return nil, fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("Create compliance benchmark?", logger) {
			return nil, nil
		}
	}
	respBody, err := jamfPOST(credName, uriBenchmarks, bodyJSON, "application/json")
	if err != nil {
		return nil, err
	}
	out, err := decodeJSONObject(respBody)
	if err != nil {
		return map[string]interface{}{"raw": string(respBody)}, nil
	}
	return out, nil
}

// BenchmarkCreate creates a benchmark and prints the response.
func BenchmarkCreate(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	out, err := BenchmarkCreateValue(credName, opts, bodyJSON, logger)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return printJSON(out)
}

// BenchmarkDeleteValue deletes a benchmark.
func BenchmarkDeleteValue(credName, benchmarkID string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(benchmarkID) == "" {
		return fmt.Errorf("benchmark-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete compliance benchmark %s?", benchmarkID), logger) {
			return nil
		}
	}
	return jamfDELETE(credName, uriBenchmarks+"/"+url.PathEscape(benchmarkID))
}

// BenchmarkDelete deletes a benchmark.
func BenchmarkDelete(credName, benchmarkID string, opts Opts, logger *slog.Logger) error {
	return BenchmarkDeleteValue(credName, benchmarkID, opts, logger)
}

// BenchmarkBaselinesValue lists mSCP baselines.
func BenchmarkBaselinesValue(credName string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, uriBaselines)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse baselines: %w", err)
	}
	return out, nil
}

// BenchmarkBaselines prints baselines JSON.
func BenchmarkBaselines(credName string, logger *slog.Logger) error {
	out, err := BenchmarkBaselinesValue(credName, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// BenchmarkCatalogRulesValue lists rules for a baseline (catalog).
func BenchmarkCatalogRulesValue(credName, baseline string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	path := uriCBRules
	if baseline != "" {
		path = path + "?baseline=" + url.QueryEscape(baseline)
	}
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	return out, nil
}

// BenchmarkCatalogRules prints catalog rules JSON.
func BenchmarkCatalogRules(credName, baseline string, logger *slog.Logger) error {
	out, err := BenchmarkCatalogRulesValue(credName, baseline, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

func benchmarkSubresource(credName, benchmarkID, suffix string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(benchmarkID) == "" {
		return nil, fmt.Errorf("benchmark-id is required")
	}
	path := uriBenchmarks + "/" + url.PathEscape(benchmarkID) + "/" + suffix
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse benchmark %s: %w", suffix, err)
	}
	return out, nil
}

// BenchmarkCompliancePercentageValue returns compliance percentage for a benchmark.
func BenchmarkCompliancePercentageValue(credName, benchmarkID string, logger *slog.Logger) (interface{}, error) {
	return benchmarkSubresource(credName, benchmarkID, "compliance-percentage", logger)
}

// BenchmarkReportDevicesValue returns devices for a benchmark report.
func BenchmarkReportDevicesValue(credName, benchmarkID string, logger *slog.Logger) (interface{}, error) {
	return benchmarkSubresource(credName, benchmarkID, "devices", logger)
}

// BenchmarkReportRulesValue returns rules stats for a benchmark.
func BenchmarkReportRulesValue(credName, benchmarkID string, logger *slog.Logger) (interface{}, error) {
	return benchmarkSubresource(credName, benchmarkID, "rules", logger)
}

// BenchmarkCompliancePercentage prints compliance percentage JSON.
func BenchmarkCompliancePercentage(credName, benchmarkID string, logger *slog.Logger) error {
	out, err := BenchmarkCompliancePercentageValue(credName, benchmarkID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// BenchmarkReportDevices prints report devices JSON.
func BenchmarkReportDevices(credName, benchmarkID string, logger *slog.Logger) error {
	out, err := BenchmarkReportDevicesValue(credName, benchmarkID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// BenchmarkReportRules prints report rules JSON.
func BenchmarkReportRules(credName, benchmarkID string, logger *slog.Logger) error {
	out, err := BenchmarkReportRulesValue(credName, benchmarkID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}
