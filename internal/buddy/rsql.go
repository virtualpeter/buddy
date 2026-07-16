package buddy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EscapeRSQLString escapes a string for use inside an RSQL double-quoted value.
// Backslash and double-quote are escaped; other characters (including *) are left as-is.
func EscapeRSQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// QuoteRSQLString escapes s and wraps it in double quotes.
func QuoteRSQLString(s string) string {
	return `"` + EscapeRSQLString(s) + `"`
}

// FormatRSQLDateTime formats t as UTC ISO-8601 with second precision for Jamf RSQL date fields
// (e.g. 2024-01-01T00:00:00Z).
func FormatRSQLDateTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// RSQLDateTime builds a UTC datetime from calendar components.
func RSQLDateTime(year, month, day, hour, minute, second int) (string, error) {
	if month < 1 || month > 12 {
		return "", fmt.Errorf("month must be 1-12, got %d", month)
	}
	if day < 1 || day > 31 {
		return "", fmt.Errorf("day must be 1-31, got %d", day)
	}
	if hour < 0 || hour > 23 {
		return "", fmt.Errorf("hour must be 0-23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return "", fmt.Errorf("minute must be 0-59, got %d", minute)
	}
	if second < 0 || second > 59 {
		return "", fmt.Errorf("second must be 0-59, got %d", second)
	}
	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	// Reject overflow (e.g. Feb 31 → March).
	if int(t.Month()) != month || t.Day() != day {
		return "", fmt.Errorf("invalid date %04d-%02d-%02d", year, month, day)
	}
	return FormatRSQLDateTime(t), nil
}

// RSQLDaysAgo returns FormatRSQLDateTime(now.UTC().Add(-days)).
func RSQLDaysAgo(days float64) string {
	return FormatRSQLDateTime(time.Now().UTC().Add(-time.Duration(days * float64(24*time.Hour))))
}

// RSQLHoursAgo returns FormatRSQLDateTime(now.UTC().Add(-hours)).
func RSQLHoursAgo(hours float64) string {
	return FormatRSQLDateTime(time.Now().UTC().Add(-time.Duration(hours * float64(time.Hour))))
}

// RSQLNow returns the current UTC time formatted for RSQL date fields.
func RSQLNow() string {
	return FormatRSQLDateTime(time.Now().UTC())
}

// RSQLEq builds field==value. Strings are quoted/escaped; numbers are unquoted.
func RSQLEq(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "==", value, true)
}

// RSQLNe builds field!=value.
func RSQLNe(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "!=", value, true)
}

// RSQLLt builds field=lt=value (numbers/datetimes typically unquoted).
func RSQLLt(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "=lt=", value, false)
}

// RSQLGt builds field=gt=value.
func RSQLGt(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "=gt=", value, false)
}

// RSQLLe builds field=le=value.
func RSQLLe(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "=le=", value, false)
}

// RSQLGe builds field=ge=value.
func RSQLGe(field string, value interface{}) (string, error) {
	return rsqlCompare(field, "=ge=", value, false)
}

// RSQLContains builds field=="*substring*" with the substring escaped for quotes/backslashes.
func RSQLContains(field, substring string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("field is required")
	}
	return field + `=="` + EscapeRSQLString("*"+substring+"*") + `"`, nil
}

// RSQLIn builds field=in=(v1,v2,...). Strings are quoted; numbers are not.
func RSQLIn(field string, values []interface{}) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("field is required")
	}
	if len(values) == 0 {
		return "", fmt.Errorf("values must be non-empty")
	}
	parts := make([]string, 0, len(values))
	for i, v := range values {
		s, err := formatRSQLValue(v, true)
		if err != nil {
			return "", fmt.Errorf("values[%d]: %w", i, err)
		}
		parts = append(parts, s)
	}
	return field + "=in=(" + strings.Join(parts, ",") + ")", nil
}

// RSQLAnd joins clauses with ";" (AND). Empty clauses are skipped.
func RSQLAnd(clauses []string) (string, error) {
	return rsqlJoin(clauses, ";")
}

// RSQLOr joins clauses with "," (OR). Empty clauses are skipped.
func RSQLOr(clauses []string) (string, error) {
	return rsqlJoin(clauses, ",")
}

func rsqlJoin(clauses []string, sep string) (string, error) {
	var parts []string
	for _, c := range clauses {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		parts = append(parts, c)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("at least one non-empty clause is required")
	}
	return strings.Join(parts, sep), nil
}

// quoteStrings controls equality-style quoting. Range operators pass false so
// ISO datetimes and numbers stay unquoted (Jamf style: dateCompleted=ge=2024-01-01T00:00:00Z).
func rsqlCompare(field, op string, value interface{}, quoteStrings bool) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", fmt.Errorf("field is required")
	}
	formatted, err := formatRSQLValue(value, quoteStrings)
	if err != nil {
		return "", err
	}
	return field + op + formatted, nil
}

func formatRSQLValue(value interface{}, quoteStrings bool) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", fmt.Errorf("value is nil")
	case string:
		if !quoteStrings {
			return v, nil
		}
		return QuoteRSQLString(v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}
