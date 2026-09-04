package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// ExtensionAttributePrepareSpec controls transform before create/copy/migrate.
type ExtensionAttributePrepareSpec struct {
	Name string
}

// ExtensionAttributeCopySpec is input for cross-session EA copy.
type ExtensionAttributeCopySpec struct {
	SourceID string
	Name     string
}

// ExtensionAttributeMigrateSpec is input for cross-session EA migrate (upsert by name).
type ExtensionAttributeMigrateSpec struct {
	SourceID string
	Name     string // empty => keep source name
}

// ExtensionAttributeCreateOutcome is returned after create/copy.
type ExtensionAttributeCreateOutcome struct {
	EA ExtensionAttributeDetail
}

// ExtensionAttributeMigrateOutcome is returned after migrate upsert.
type ExtensionAttributeMigrateOutcome struct {
	EA      ExtensionAttributeDetail
	Created bool
}

// PrepareExtensionAttributeForCreate builds a create/put payload from a source EA.
// Strips id; keeps scriptContents, popup/LDAP/display/enabled. No category remap.
func PrepareExtensionAttributeForCreate(src *ExtensionAttributeDetail, spec ExtensionAttributePrepareSpec) (ExtensionAttributeDetail, error) {
	if src == nil {
		return ExtensionAttributeDetail{}, fmt.Errorf("source extension attribute is required")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return ExtensionAttributeDetail{}, fmt.Errorf("extension attribute name is required")
	}
	out := *src
	out.ID = ""
	out.Name = name
	return out, nil
}

// ExtensionAttributeClone duplicates an EA in the same session with a new name.
func ExtensionAttributeClone(credName string, opts Opts, eaID, name string, logger *slog.Logger) (*ExtensionAttributeCreateOutcome, error) {
	src, err := ExtensionAttributeFetchFull(credName, eaID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareExtensionAttributeForCreate(src, ExtensionAttributePrepareSpec{Name: name})
	if err != nil {
		return nil, err
	}
	created, err := ExtensionAttributePostDetail(credName, opts, prepared, logger)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, nil
	}
	return &ExtensionAttributeCreateOutcome{EA: *created}, nil
}

// ExtensionAttributeCopy fetches from src and POSTs on dst.
func ExtensionAttributeCopy(src, dst SessionRef, opts Opts, spec ExtensionAttributeCopySpec, logger *slog.Logger) (*ExtensionAttributeCreateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *ExtensionAttributeDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = ExtensionAttributeFetchFull(pin, spec.SourceID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	var outcome *ExtensionAttributeCreateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareExtensionAttributeForCreate(detail, ExtensionAttributePrepareSpec{Name: spec.Name})
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nCopy extension attribute (ID %s) as %q to destination?", spec.SourceID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true
		created, err := ExtensionAttributePostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &ExtensionAttributeCreateOutcome{EA: *created}
		return nil
	})
	return outcome, err
}

// ExtensionAttributeMigrate fetches from src and upserts on dst by exact name.
func ExtensionAttributeMigrate(src, dst SessionRef, opts Opts, spec ExtensionAttributeMigrateSpec, logger *slog.Logger) (*ExtensionAttributeMigrateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *ExtensionAttributeDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = ExtensionAttributeFetchFull(pin, spec.SourceID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = detail.Name
	}
	var outcome *ExtensionAttributeMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareExtensionAttributeForCreate(detail, ExtensionAttributePrepareSpec{Name: name})
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nMigrate extension attribute (ID %s) as %q to destination (upsert by name)?", spec.SourceID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true

		destID, findErr := ExtensionAttributeGetID(pin, prepared.Name, logger)
		if findErr == nil && destID != "" {
			prepared.ID = destID
			updated, err := extensionAttributePut(pin, destID, &prepared)
			if err != nil {
				return err
			}
			fmt.Printf("Updated extension attribute (ID %s): %s\n", updated.ID, updated.Name)
			outcome = &ExtensionAttributeMigrateOutcome{EA: *updated, Created: false}
			return nil
		}
		created, err := ExtensionAttributePostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &ExtensionAttributeMigrateOutcome{EA: *created, Created: true}
		return nil
	})
	return outcome, err
}
