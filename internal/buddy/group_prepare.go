package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// SmartGroupPrepareSpec controls transform before smart group create/copy.
type SmartGroupPrepareSpec struct {
	Name   string
	SiteID *string // nil = clear siteId; non-nil sets siteId (empty string clears)
}

// SmartGroupCopySpec is input for cross-session smart group copy.
type SmartGroupCopySpec struct {
	SourceGroupID string
	Name          string
	SiteID        *string
}

// PrepareSmartGroupForCreate builds a create payload from a source smart group JSON map.
func PrepareSmartGroupForCreate(src map[string]any, spec SmartGroupPrepareSpec) (map[string]any, error) {
	if src == nil {
		return nil, fmt.Errorf("source smart group is required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("smart group name is required")
	}
	out := map[string]any{}
	for k, v := range src {
		out[k] = v
	}
	out["id"] = -1
	out["name"] = name
	if spec.SiteID != nil {
		if strings.TrimSpace(*spec.SiteID) == "" {
			delete(out, "siteId")
		} else {
			out["siteId"] = strings.TrimSpace(*spec.SiteID)
		}
	} else {
		delete(out, "siteId")
	}
	// Drop read-only / membership fields that break create if present.
	for _, k := range []string{"membershipCount", "smartGroupMembershipPercentage", "users"} {
		delete(out, k)
	}
	if _, ok := out["criteria"]; !ok {
		out["criteria"] = []any{}
	}
	return out, nil
}

// SmartGroupClone duplicates a smart group in the same session with a new name.
func SmartGroupClone(credName string, opts Opts, groupID, name string, siteID *string, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := SmartGroupFetchValue(credName, groupID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareSmartGroupForCreate(src, SmartGroupPrepareSpec{Name: name, SiteID: siteID})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(prepared)
	if err != nil {
		return nil, fmt.Errorf("marshal smart group: %w", err)
	}
	return GroupCreateSmart(credName, opts, payload, logger)
}

// SmartGroupCopy fetches from srcCred and POSTs on dstCred.
func SmartGroupCopy(srcCred, dstCred string, opts Opts, spec SmartGroupCopySpec, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := SmartGroupFetchValue(srcCred, spec.SourceGroupID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareSmartGroupForCreate(src, SmartGroupPrepareSpec{Name: spec.Name, SiteID: spec.SiteID})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(prepared)
	if err != nil {
		return nil, fmt.Errorf("marshal smart group: %w", err)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCopy smart group (ID %s) as %q to destination?", spec.SourceGroupID, prepared["name"])
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	return GroupCreateSmart(dstCred, optsCopy, payload, logger)
}
