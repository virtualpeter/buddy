package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// CategoryPrepareSpec controls transform before category create/copy.
type CategoryPrepareSpec struct {
	Name     string
	Priority *int // nil = keep source priority
}

// CategoryCopySpec is input for cross-session category copy.
type CategoryCopySpec struct {
	SourceCategoryID string
	Name             string
	Priority         *int
}

// CategoryMigrateSpec is input for cross-session category migrate (upsert by name).
type CategoryMigrateSpec struct {
	SourceCategoryID string
	Name             string // empty => keep source name
	Priority         *int
}

// CategoryMigrateOutcome is returned after category_migrate upsert.
type CategoryMigrateOutcome struct {
	ID       string
	Name     string
	Priority int
	Created  bool
}

// PrepareCategoryForCreate builds a create payload from a source category.
func PrepareCategoryForCreate(src *CategoryDetail, spec CategoryPrepareSpec) (CategoryDetail, error) {
	if src == nil {
		return CategoryDetail{}, fmt.Errorf("source category is required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return CategoryDetail{}, fmt.Errorf("category name is required")
	}
	out := CategoryDetail{
		Name:     name,
		Priority: src.Priority,
	}
	if spec.Priority != nil {
		out.Priority = *spec.Priority
	}
	return out, nil
}

// CategoryClone duplicates a category in the same session with a new name.
func CategoryClone(credName string, opts Opts, categoryID, name string, priority *int, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := CategoryFetch(credName, categoryID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareCategoryForCreate(src, CategoryPrepareSpec{Name: name, Priority: priority})
	if err != nil {
		return nil, err
	}
	return CategoryPost(credName, opts, prepared, logger)
}

// CategoryCopy fetches from src and POSTs on dst.
func CategoryCopy(src, dst SessionRef, opts Opts, spec CategoryCopySpec, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var prepared CategoryDetail
	err = WithSessionRef(src, func(pin string) error {
		s, err := CategoryFetch(pin, spec.SourceCategoryID)
		if err != nil {
			return err
		}
		prepared, err = PrepareCategoryForCreate(s, CategoryPrepareSpec{Name: spec.Name, Priority: spec.Priority})
		return err
	})
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCopy category (ID %s) as %q to destination?", spec.SourceCategoryID, prepared.Name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	var created *GroupCreateResult
	err = WithSessionRef(dst, func(pin string) error {
		var postErr error
		created, postErr = CategoryPost(pin, optsCopy, prepared, logger)
		return postErr
	})
	return created, err
}

// CategoryMigrate fetches from src and upserts on dst by exact name.
func CategoryMigrate(src, dst SessionRef, opts Opts, spec CategoryMigrateSpec, logger *slog.Logger) (*CategoryMigrateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var prepared CategoryDetail
	err = WithSessionRef(src, func(pin string) error {
		s, err := CategoryFetch(pin, spec.SourceCategoryID)
		if err != nil {
			return err
		}
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = s.Name
		}
		prepared, err = PrepareCategoryForCreate(s, CategoryPrepareSpec{Name: name, Priority: spec.Priority})
		return err
	})
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nMigrate category (ID %s) as %q to destination (upsert by name)?", spec.SourceCategoryID, prepared.Name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true

	var outcome *CategoryMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		existing, found, err := CategoryFindByExactName(pin, prepared.Name)
		if err != nil {
			return err
		}
		if found {
			if err := CategoryPut(pin, optsCopy, existing.ID, prepared, logger); err != nil {
				return err
			}
			outcome = &CategoryMigrateOutcome{ID: existing.ID, Name: prepared.Name, Priority: prepared.Priority, Created: false}
			return nil
		}
		created, err := CategoryPost(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &CategoryMigrateOutcome{ID: created.ID, Name: prepared.Name, Priority: prepared.Priority, Created: true}
		return nil
	})
	return outcome, err
}
