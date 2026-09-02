package buddy

import (
	"fmt"
	"strings"

	"howett.net/plist"
)

// PayloadContentRow is one PayloadContent dict summarised for list/inspect.
// Identifier/Type/UUID are empty strings when the plist key is missing.
type PayloadContentRow struct {
	Identifier string
	Type       string
	UUID       string
}

func payloadDictString(dict map[string]interface{}, key string) string {
	s, _ := dict[key].(string)
	return strings.TrimSpace(s)
}

func payloadContentRowsFromArr(arr []interface{}) []PayloadContentRow {
	var rows []PayloadContentRow
	for _, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, PayloadContentRow{
			Identifier: payloadDictString(dict, "PayloadIdentifier"),
			Type:       payloadDictString(dict, "PayloadType"),
			UUID:       payloadDictString(dict, "PayloadUUID"),
		})
	}
	return rows
}

func formatPayloadContentRows(rows []PayloadContentRow) string {
	if len(rows) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, fmt.Sprintf("identifier=%q type=%q uuid=%q", r.Identifier, r.Type, r.UUID))
	}
	return strings.Join(parts, "; ")
}

// PlistPayloadContentRows lists PayloadContent entries as {identifier, type, uuid}.
// Returns a nil slice when the plist has no PayloadContent array.
func PlistPayloadContentRows(payloadsRaw string) ([]PayloadContentRow, error) {
	return plistPayloadContentRows(payloadsRaw)
}

func plistPayloadContentRows(payloadsRaw string) ([]PayloadContentRow, error) {
	if strings.TrimSpace(payloadsRaw) == "" {
		return nil, nil
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return nil, nil
	}
	arr, ok := content.([]interface{})
	if !ok {
		return nil, nil
	}
	return payloadContentRowsFromArr(arr), nil
}

// matchPayloadContentIndex resolves identifier= against PayloadContent:
// exact PayloadIdentifier, else unique PayloadType, else unique PayloadUUID.
func matchPayloadContentIndex(arr []interface{}, selector string) (int, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return -1, fmt.Errorf("identifier is required")
	}
	var idHits, typeHits, uuidHits []int
	for i, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if payloadDictString(dict, "PayloadIdentifier") == selector {
			idHits = append(idHits, i)
		}
		if payloadDictString(dict, "PayloadType") == selector {
			typeHits = append(typeHits, i)
		}
		if payloadDictString(dict, "PayloadUUID") == selector {
			uuidHits = append(uuidHits, i)
		}
	}
	rows := payloadContentRowsFromArr(arr)
	listed := formatPayloadContentRows(rows)
	if n := len(idHits); n == 1 {
		return idHits[0], nil
	} else if n > 1 {
		return -1, fmt.Errorf("identifier %q is ambiguous; payloads: %s", selector, listed)
	}
	if n := len(typeHits); n == 1 {
		return typeHits[0], nil
	} else if n > 1 {
		return -1, fmt.Errorf("PayloadType %q is not unique; payloads: %s", selector, listed)
	}
	if n := len(uuidHits); n == 1 {
		return uuidHits[0], nil
	} else if n > 1 {
		return -1, fmt.Errorf("PayloadUUID %q is not unique; payloads: %s", selector, listed)
	}
	return -1, fmt.Errorf("payload %q not found in PayloadContent; payloads: %s", selector, listed)
}

func preserveCanonicalIdentifier(merged map[string]interface{}, canonical, selector string) {
	newID := payloadDictString(merged, "PayloadIdentifier")
	if newID == "" && canonical != "" {
		merged["PayloadIdentifier"] = canonical
		return
	}
	if newID == selector && selector != canonical && canonical != "" {
		merged["PayloadIdentifier"] = canonical
	}
}

func mcxDomainNames(entries []MCXPayloadEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.TrimSpace(e.Domain) != "" {
			out = append(out, e.Domain)
		}
	}
	return out
}
