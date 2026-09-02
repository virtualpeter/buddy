package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// ScriptPrepareSpec controls transform before script create/copy/migrate.
type ScriptPrepareSpec struct {
	Name               string
	CategoryID         string // explicit override on destination (wins)
	SameSession        bool   // keep source categoryId when no override
	DestCred           string // destination credential for category name resolve
	SourceCategoryID   string
	SourceCategoryName string
}

// ScriptCopySpec is input for cross-session script copy.
type ScriptCopySpec struct {
	SourceScriptID string
	Name           string
	CategoryID     string // optional override on destination
}

// ScriptMigrateSpec is input for cross-session script migrate (upsert by name).
type ScriptMigrateSpec struct {
	SourceScriptID string
	Name           string // empty => keep source name
	CategoryID     string
}

// ScriptCreateOutcome is returned after script create/copy.
type ScriptCreateOutcome struct {
	Script ScriptDetail
}

// ScriptMigrateOutcome is returned after script_migrate upsert.
type ScriptMigrateOutcome struct {
	Script  ScriptDetail
	Created bool
}

// PrepareScriptForCreate builds a create/put payload from a source script.
func PrepareScriptForCreate(src *ScriptDetail, spec ScriptPrepareSpec) (ScriptDetail, error) {
	if src == nil {
		return ScriptDetail{}, fmt.Errorf("source script is required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return ScriptDetail{}, fmt.Errorf("script name is required")
	}
	out := *src
	out.ID = ""
	out.Name = name
	out.CategoryName = ""

	keepID := ""
	if spec.SameSession {
		keepID = strings.TrimSpace(spec.SourceCategoryID)
		if keepID == "" {
			keepID = strings.TrimSpace(src.CategoryID)
		}
	}
	catName := strings.TrimSpace(spec.SourceCategoryName)
	if catName == "" {
		catName = strings.TrimSpace(src.CategoryName)
	}
	resolved, err := ResolveCategoryOnDest(spec.DestCred, catName, spec.CategoryID, keepID)
	if err != nil {
		return ScriptDetail{}, err
	}
	if resolved.ID == "" || resolved.ID == "-1" {
		out.CategoryID = "-1"
		out.CategoryName = ""
	} else {
		out.CategoryID = resolved.ID
		out.CategoryName = resolved.Name
	}
	return out, nil
}

// ScriptClone duplicates a script in the same session with a new name.
func ScriptClone(credName string, opts Opts, scriptID, name, categoryID string, logger *slog.Logger) (*ScriptCreateOutcome, error) {
	src, err := ScriptFetchFull(credName, scriptID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareScriptForCreate(src, ScriptPrepareSpec{
		Name:               name,
		CategoryID:         categoryID,
		SameSession:        true,
		DestCred:           credName,
		SourceCategoryID:   src.CategoryID,
		SourceCategoryName: src.CategoryName,
	})
	if err != nil {
		return nil, err
	}
	created, err := ScriptPostDetail(credName, opts, prepared, logger)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, nil
	}
	return &ScriptCreateOutcome{Script: *created}, nil
}

// ScriptCopy fetches from src and POSTs on dst.
func ScriptCopy(src, dst SessionRef, opts Opts, spec ScriptCopySpec, logger *slog.Logger) (*ScriptCreateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *ScriptDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = ScriptFetchFull(pin, spec.SourceScriptID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	same := src.Equal(dst)
	var outcome *ScriptCreateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareScriptForCreate(detail, ScriptPrepareSpec{
			Name:               spec.Name,
			CategoryID:         spec.CategoryID,
			SameSession:        same,
			DestCred:           pin,
			SourceCategoryID:   detail.CategoryID,
			SourceCategoryName: detail.CategoryName,
		})
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nCopy script (ID %s) as %q to destination?", spec.SourceScriptID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true
		created, err := ScriptPostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &ScriptCreateOutcome{Script: *created}
		return nil
	})
	return outcome, err
}

// ScriptMigrate fetches from src and upserts on dst by exact name.
func ScriptMigrate(src, dst SessionRef, opts Opts, spec ScriptMigrateSpec, logger *slog.Logger) (*ScriptMigrateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *ScriptDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = ScriptFetchFull(pin, spec.SourceScriptID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = detail.Name
	}
	same := src.Equal(dst)
	var outcome *ScriptMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareScriptForCreate(detail, ScriptPrepareSpec{
			Name:               name,
			CategoryID:         spec.CategoryID,
			SameSession:        same,
			DestCred:           pin,
			SourceCategoryID:   detail.CategoryID,
			SourceCategoryName: detail.CategoryName,
		})
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nMigrate script (ID %s) as %q to destination (upsert by name)?", spec.SourceScriptID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true

		destID, findErr := ScriptGetID(pin, prepared.Name, logger)
		if findErr == nil && destID != "" {
			prepared.ID = destID
			updated, err := scriptPut(pin, destID, &prepared)
			if err != nil {
				return err
			}
			fmt.Printf("Updated script (ID %s): %s\n", updated.ID, updated.Name)
			outcome = &ScriptMigrateOutcome{Script: *updated, Created: false}
			return nil
		}
		created, err := ScriptPostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &ScriptMigrateOutcome{Script: *created, Created: true}
		return nil
	})
	return outcome, err
}
