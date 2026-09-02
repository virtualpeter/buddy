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

// SmartGroupMigrateSpec is input for cross-session smart group migrate (upsert by name).
type SmartGroupMigrateSpec struct {
	SourceGroupID string
	Name          string // empty => keep source name
	SiteID        *string
}

// SmartGroupMigrateOutcome is returned after smart_group_copy upsert.
type SmartGroupMigrateOutcome struct {
	ID      string
	Href    string
	Name    string
	Created bool
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

// SmartGroupCopy fetches from src and POSTs on dst.
func SmartGroupCopy(src, dst SessionRef, opts Opts, spec SmartGroupCopySpec, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var payload []byte
	var name any
	err = WithSessionRef(src, func(pin string) error {
		s, err := SmartGroupFetchValue(pin, spec.SourceGroupID)
		if err != nil {
			return err
		}
		prepared, err := PrepareSmartGroupForCreate(s, SmartGroupPrepareSpec{Name: spec.Name, SiteID: spec.SiteID})
		if err != nil {
			return err
		}
		name = prepared["name"]
		payload, err = json.Marshal(prepared)
		return err
	})
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCopy smart group (ID %s) as %q to destination?", spec.SourceGroupID, name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	var created *GroupCreateResult
	err = WithSessionRef(dst, func(pin string) error {
		var postErr error
		created, postErr = GroupCreateSmart(pin, optsCopy, payload, logger)
		return postErr
	})
	return created, err
}

// SmartGroupFindByExactName returns a smart group ID for an exact name match (case-insensitive).
func SmartGroupFindByExactName(credName, name string, logger *slog.Logger) (id string, found bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false, fmt.Errorf("smart group name is required")
	}
	records, err := GroupSearchSmart(credName, name, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", false, err
	}
	for _, g := range records {
		if strings.EqualFold(strings.TrimSpace(g.Name), name) {
			return g.ID, true, nil
		}
	}
	return "", false, nil
}

// SmartGroupMigrate fetches from src and upserts on dst by exact name.
func SmartGroupMigrate(src, dst SessionRef, opts Opts, spec SmartGroupMigrateSpec, logger *slog.Logger) (*SmartGroupMigrateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var prepared map[string]any
	err = WithSessionRef(src, func(pin string) error {
		s, err := SmartGroupFetchValue(pin, spec.SourceGroupID)
		if err != nil {
			return err
		}
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			if n, ok := s["name"].(string); ok {
				name = n
			}
		}
		prepared, err = PrepareSmartGroupForCreate(s, SmartGroupPrepareSpec{Name: name, SiteID: spec.SiteID})
		return err
	})
	if err != nil {
		return nil, err
	}
	name, _ := prepared["name"].(string)
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nMigrate smart group (ID %s) as %q to destination (upsert by name)?", spec.SourceGroupID, name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	var outcome *SmartGroupMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		existingID, found, err := SmartGroupFindByExactName(pin, name, logger)
		if err != nil {
			return err
		}
		if found {
			prepared["id"] = existingID
			payload, err := json.Marshal(prepared)
			if err != nil {
				return err
			}
			if err := GroupPutSmartRaw(pin, optsCopy, existingID, payload, logger); err != nil {
				return err
			}
			outcome = &SmartGroupMigrateOutcome{ID: existingID, Name: name, Created: false}
			return nil
		}
		payload, err := json.Marshal(prepared)
		if err != nil {
			return err
		}
		created, err := GroupCreateSmart(pin, optsCopy, payload, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &SmartGroupMigrateOutcome{ID: created.ID, Href: created.Href, Name: name, Created: true}
		return nil
	})
	return outcome, err
}
