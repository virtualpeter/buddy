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

// CategoryCopy fetches from srcCred and POSTs on dstCred.
func CategoryCopy(srcCred, dstCred string, opts Opts, spec CategoryCopySpec, logger *slog.Logger) (*GroupCreateResult, error) {
	src, err := CategoryFetch(srcCred, spec.SourceCategoryID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareCategoryForCreate(src, CategoryPrepareSpec{Name: spec.Name, Priority: spec.Priority})
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
	return CategoryPost(dstCred, optsCopy, prepared, logger)
}

// CategoryMigrate fetches from srcCred and upserts on dstCred by exact name.
func CategoryMigrate(srcCred, dstCred string, opts Opts, spec CategoryMigrateSpec, logger *slog.Logger) (*CategoryMigrateOutcome, error) {
	src, err := CategoryFetch(srcCred, spec.SourceCategoryID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = src.Name
	}
	prepared, err := PrepareCategoryForCreate(src, CategoryPrepareSpec{Name: name, Priority: spec.Priority})
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

	existing, found, err := CategoryFindByExactName(dstCred, prepared.Name)
	if err != nil {
		return nil, err
	}
	if found {
		if err := CategoryPut(dstCred, optsCopy, existing.ID, prepared, logger); err != nil {
			return nil, err
		}
		return &CategoryMigrateOutcome{ID: existing.ID, Name: prepared.Name, Priority: prepared.Priority, Created: false}, nil
	}
	created, err := CategoryPost(dstCred, optsCopy, prepared, logger)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, nil
	}
	return &CategoryMigrateOutcome{ID: created.ID, Name: prepared.Name, Priority: prepared.Priority, Created: true}, nil
}
