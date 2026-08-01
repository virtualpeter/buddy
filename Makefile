
# Go compiler
GO := go
GOFLAGS := -v -buildvcs=false

BUILDTARGET := buddy

# Find all Go files with package main
GO_MAIN_FILES := $(shell find . -name "*.go" -type f -exec grep -l "^package main" {} \; | grep -v vendor | grep -v ".git" | sort)

# Version from git (tag or short hash)
BUDDY_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Buddy CLI (builds to ./buddy)
BUDDY_GO_FILES := $(shell find cmd/buddy internal docfs stdlib \( -name "*.go" -o -name "*.buddy" -o -name "*.star" \) 2>/dev/null)
DOC_SOURCES := $(shell find docs examples .cursor/rules -type f \( -name '*.md' -o -name '*.buddy' -o -name '*.star' -o -name '*.json' -o -name '*.mdc' \) 2>/dev/null) README.md AGENTS.md

.PHONY: sync-docfs
sync-docfs:
	@./scripts/sync-docfs.sh

# Refresh vendored Jamf Platform API OpenAPI specs (needs network). Commit the result.
.PHONY: sync-platform-schema
sync-platform-schema:
	@./scripts/sync-platform-schema.sh

docfs/content/README.embedded: $(DOC_SOURCES) scripts/sync-docfs.sh
	@./scripts/sync-docfs.sh

$(BUILDTARGET): sync-docfs $(BUDDY_GO_FILES) docfs/content/README.embedded
	@echo "Building buddy (version $(BUDDY_VERSION))..."
	$(GO) build $(GOFLAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(BUILDTARGET) ./cmd/buddy
	@! santactl status>/dev/null|| santactl fileinfo buddy|grep Rule|egrep '(Allowed|None)' >/dev/null|| rm buddy
	@ls -l $(BUILDTARGET)

install: $(BUILDTARGET)
	rm -f ~/go/bin/$(BUILDTARGET)
	cp $(BUILDTARGET) ~/go/bin/$(BUILDTARGET)
.PHONY: install

# Run unit tests
test: sync-docfs
	$(GO) test -v ./...
.PHONY: test

# Run integration tests (requires non-production Jamf + buddy-test-source-profile; pin --tenant/--user)
integration-test: $(BUILDTARGET)
	./$(BUILDTARGET) -y scripts/integration_test.buddy
.PHONY: integration-test

# ACS hydrate shake-down against a live Classic profile (default id=7; same-tenant mutate+restore).
# When hydrate cannot resolve the ACS payload UUID from HTML, set e.g.:
#   BUDDY_ACS_PAYLOAD_UUID_7=<FIELD_EXTERNAL_CONFIGPROFILE_ID> make integration-test-acs7
# or set TEST_ACS_PAYLOAD_UUID in scripts/integration_test_acs_profile7.buddy
integration-test-acs7: $(BUILDTARGET)
	./$(BUILDTARGET) -y scripts/integration_test_acs_profile7.buddy
.PHONY: integration-test-acs7

# Read-only: classify profiles that need ACS hydrate / UUID bind
scan-acs-profiles: $(BUILDTARGET)
	./$(BUILDTARGET) -y scripts/scan_acs_profile_handling.buddy
.PHONY: scan-acs-profiles

# Local docs + notebook UI (opens browser)
docs-serve: $(BUILDTARGET)
	./$(BUILDTARGET) serve --open
.PHONY: docs-serve

# Clean all built executables
clean:
	@echo "Cleaning executables..."
	@rm -f $(BUILDTARGET)
.PHONY: clean

# Package Cursor/VS Code extension VSIX (PUBLISHER=virtualpete by default)
extension-package:
	cd editors/vscode && npm install && PUBLISHER=$(or $(PUBLISHER),virtualpete) npm run package
.PHONY: extension-package

# Help target
help:
	@echo "Generic Go Build Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  $(BUILDTARGET)   - Sync embedded docs + build $(BUILDTARGET) CLI"
	@echo "  sync-docfs - Copy docs/AGENTS/examples/rules into docfs/content for go:embed"
	@echo "  sync-platform-schema - Refresh vendored Jamf Platform API OpenAPI specs"
	@echo "  install   - Install $(BUILDTARGET) CLI to ~/go/bin/"
	@echo "  test      - Run unit tests"
	@echo "  integration-test - Run Starlark integration tests against Jamf"
	@echo "  integration-test-acs7 - ACS profile id=7 same-tenant mutate/restore suite"
	@echo "  scan-acs-profiles - Read-only audit: profiles needing ACS hydrate/bind handling"
	@echo "  docs-serve - Local docs + notebook (buddy serve --open)"
	@echo "  extension-package - Build editors/vscode VSIX (PUBLISHER=virtualpete)"
	@echo "  clean   - Remove all built executables"
	@echo "  help    - Show this help message"
.PHONY: help
