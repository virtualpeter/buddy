
# Go compiler
GO := go
GOFLAGS := -v -buildvcs=false
# Default builds are CGO-free (Keychain uses security(1)).
# make buddy-darwin-arm64 is Darwin/arm64-only: CGO + Security.framework + Developer ID.
CGO_ENABLED ?= 0
export CGO_ENABLED
# Signed release: codesign identity is required. Identifier must stay stable across releases.
BUDDY_CODESIGN_IDENTIFIER ?= virtualpete.buddy.cli

BUILDTARGET := buddy
SIGNED_TARGET := buddy-darwin-arm64

# Find all Go files with package main
GO_MAIN_FILES := $(shell find . -name "*.go" -type f -exec grep -l "^package main" {} \; | grep -v vendor | grep -v ".git" | sort)

# Version from git (tag or short hash)
BUDDY_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Buddy CLI (builds to ./buddy)
BUDDY_GO_FILES := $(shell find cmd/buddy internal docfs stdlib \( -name "*.go" -o -name "*.buddy" -o -name "buddy.png" \) 2>/dev/null)
DOC_SOURCES := $(shell find docs examples -type f \( -name '*.md' -o -name '*.buddy' -o -name '*.json' \) 2>/dev/null) README.md

.PHONY: gendocs
gendocs:
	$(GO) run ./cmd/gendocs

.PHONY: check-gendocs
check-gendocs: gendocs
	@git diff --exit-code -- docs/generated internal/help/apps_gen.go || (echo "gendocs output stale; commit make gendocs" && exit 1)

.PHONY: sync-docfs
sync-docfs: gendocs
	@./scripts/sync-docfs.sh

# Refresh vendored Jamf Platform API OpenAPI specs (needs network). Commit the result.
.PHONY: sync-platform-schema
sync-platform-schema:
	@./scripts/sync-platform-schema.sh

docfs/content/README.embedded: gendocs $(DOC_SOURCES) scripts/sync-docfs.sh
	@./scripts/sync-docfs.sh

# Optional build tags (comma-separated). Exclusion tags shrink the binary:
#   noserve — omit buddy serve (drops goldmark/chroma)
#   nomcp   — omit buddy mcp (drops MCP SDK)
#   nolsp   — omit buddy lsp (small savings; starlark stays via script)
# Example: make buddy TAGS=noserve,nomcp
TAGS ?=
comma := ,
ifneq ($(TAGS),)
GO_TAGS := -tags '$(TAGS)'
else
GO_TAGS :=
endif

$(BUILDTARGET): sync-docfs $(BUDDY_GO_FILES) docfs/content/README.embedded
	@echo "Building buddy (version $(BUDDY_VERSION)$(if $(TAGS), tags=$(TAGS)), CGO_ENABLED=$(CGO_ENABLED))..."
	$(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(BUILDTARGET) ./cmd/buddy
	@! santactl status>/dev/null|| santactl fileinfo buddy|grep Rule|egrep '(Allowed|None)' >/dev/null|| rm buddy
	@ls -lh $(BUILDTARGET)

# Automation-oriented binary without serve/mcp/lsp (same CLI + buddyscript).
.PHONY: buddy-lite
buddy-lite: sync-docfs
	@echo "Building buddy-lite (version $(BUDDY_VERSION), tags=noserve,nomcp,nolsp, CGO_ENABLED=$(CGO_ENABLED))..."
	$(GO) build $(GOFLAGS) -tags 'noserve,nomcp,nolsp' -ldflags "-X main.Version=$(BUDDY_VERSION)" -o buddy-lite ./cmd/buddy
	@ls -lh buddy-lite

# Darwin/arm64 release binary: Security.framework Keychain (service virtualpete.buddy.cli, label Buddy).
# Requires BUDDY_CODESIGN_IDENTITY (Developer ID Application). Does not clobber ./buddy.
.PHONY: $(SIGNED_TARGET)
$(SIGNED_TARGET): sync-docfs
	@test "$$(uname -s)" = Darwin || { echo "make $(SIGNED_TARGET) is macOS-only"; exit 1; }
	@test "$$(uname -m)" = arm64 || { echo "make $(SIGNED_TARGET) is arm64-only"; exit 1; }
	@test -n "$(BUDDY_CODESIGN_IDENTITY)" || { echo "BUDDY_CODESIGN_IDENTITY is required (Developer ID Application)"; exit 1; }
	@echo "Building $(SIGNED_TARGET) (version $(BUDDY_VERSION)$(if $(TAGS), tags=secitem,$(TAGS), tags=secitem), CGO_ENABLED=1)..."
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -tags 'secitem$(if $(TAGS),$(comma)$(TAGS))' -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(SIGNED_TARGET) ./cmd/buddy
	codesign --force --sign "$(BUDDY_CODESIGN_IDENTITY)" --identifier "$(BUDDY_CODESIGN_IDENTIFIER)" $(SIGNED_TARGET)
	@info=$$(codesign -dv --verbose=2 $(SIGNED_TARGET) 2>&1); \
	printf '%s\n' "$$info"; \
	case "$$info" in *Signature=adhoc*) echo "$(SIGNED_TARGET): ad-hoc signature rejected; use a Developer ID"; exit 1;; esac; \
	case "$$info" in *TeamIdentifier=not\ set*) echo "$(SIGNED_TARGET): TeamIdentifier not set"; exit 1;; esac; \
	echo "$$info" | grep -E 'TeamIdentifier=[A-Z0-9]+' >/dev/null || { echo "$(SIGNED_TARGET): missing TeamIdentifier"; exit 1; }
	@ls -lh $(SIGNED_TARGET)

# Cross-compile Linux amd64 (pure Go; no CGO).
.PHONY: buddy-linux
buddy-linux: sync-docfs
	@echo "Building buddy-linux-amd64 (version $(BUDDY_VERSION), CGO_ENABLED=0)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o buddy-linux-amd64 ./cmd/buddy
	@ls -lh buddy-linux-amd64

# Cross-compile Windows amd64 (pure Go; config under %APPDATA%\buddy).
.PHONY: buddy-windows
buddy-windows: sync-docfs
	@echo "Building buddy-windows-amd64.exe (version $(BUDDY_VERSION), CGO_ENABLED=0)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o buddy-windows-amd64.exe ./cmd/buddy
	@ls -lh buddy-windows-amd64.exe

# Compare binary sizes for feature-tag combinations (writes /tmp/buddy-size-*).
.PHONY: size-report
size-report: sync-docfs
	@./scripts/buddy-size-report.sh

install: $(BUILDTARGET)
	rm -f ~/go/bin/$(BUILDTARGET)
	cp $(BUILDTARGET) ~/go/bin/$(BUILDTARGET)
.PHONY: install

# Install the signed Darwin/arm64 binary as ~/go/bin/buddy (replaces an unsigned install).
install-signed: $(SIGNED_TARGET)
	rm -f ~/go/bin/$(BUILDTARGET)
	cp $(SIGNED_TARGET) ~/go/bin/$(BUILDTARGET)
.PHONY: install-signed

# Branded product archive: signed Darwin/arm64 binary → /usr/local/bin/buddy
# (installer welcome/background + Finder icon from internal/serve/buddy.png).
# pkgbuild --version must be period-delimited integers (strip v prefix and -dirty / -N-gSHA).
# Optional: BUDDY_PKG_SIGN_IDENTITY='Developer ID Installer: …' to productsign the pkg.
PKG_VERSION ?= $(shell v=$$(printf '%s' "$(BUDDY_VERSION)" | sed -E 's/^v//;s/-.*//'); printf '%s\n' "$${v:-0.0.0}")
PKG_TARGET ?= buddy-$(PKG_VERSION).pkg
.PHONY: package
package: $(SIGNED_TARGET)
	@test "$$(uname -s)" = Darwin || { echo "make package is macOS-only"; exit 1; }
	@./scripts/build-pkg.sh "$(SIGNED_TARGET)" "$(PKG_TARGET)" "$(PKG_VERSION)" "$(BUDDY_CODESIGN_IDENTIFIER)" "$(BUDDY_PKG_SIGN_IDENTITY)"

# Run unit tests (default Keychain: security(1) on Darwin, CGO off).
test: sync-docfs
	$(GO) test -v ./...
.PHONY: test

# Compile and test the Security.framework Keychain backend (-tags secitem).
# Darwin-only; does not codesign (unit tests do not need a Developer ID).
.PHONY: test-signed
test-signed: sync-docfs
	@test "$$(uname -s)" = Darwin || { echo "make test-signed is macOS-only (Security.framework)"; exit 1; }
	@echo "Testing secitem Keychain backend (CGO_ENABLED=1, tags=secitem$(if $(TAGS),$(comma)$(TAGS)))..."
	CGO_ENABLED=1 $(GO) test -v -tags 'secitem$(if $(TAGS),$(comma)$(TAGS))' ./...

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

# Local docs + script IDE (opens browser)
docs-serve: $(BUILDTARGET)
	./$(BUILDTARGET) serve --open
.PHONY: docs-serve

# Clean all built executables
clean:
	@echo "Cleaning executables..."
	@rm -f $(BUILDTARGET) $(SIGNED_TARGET) buddy-signed buddy-lite buddy-linux-amd64 buddy-windows-amd64.exe buddy-*.pkg *.pkg.unsigned
.PHONY: clean

# Help target
help:
	@echo "Generic Go Build Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  $(BUILDTARGET)   - Sync embedded docs + build $(BUILDTARGET) CLI"
	@echo "  $(SIGNED_TARGET) - Darwin/arm64 release: -tags secitem + Developer ID → ./$(SIGNED_TARGET)"
	@echo "  buddy-lite - Build with TAGS=noserve,nomcp,nolsp → ./buddy-lite"
	@echo "  buddy-linux - Cross-compile linux/amd64 → ./buddy-linux-amd64"
	@echo "  buddy-windows - Cross-compile windows/amd64 → ./buddy-windows-amd64.exe"
	@echo "  size-report - Build tag variants and print size contribution table"
	@echo "  sync-docfs - Copy docs/AGENTS/examples/rules into docfs/content for go:embed"
	@echo "  sync-platform-schema - Refresh vendored Jamf Platform API OpenAPI specs"
	@echo "  install   - Install $(BUILDTARGET) CLI to ~/go/bin/"
	@echo "  install-signed - Build $(SIGNED_TARGET) and install as ~/go/bin/$(BUILDTARGET)"
	@echo "  package   - Build $(SIGNED_TARGET) into a branded pkg that installs /usr/local/bin/buddy"
	@echo "  test      - Run unit tests (default Keychain / CGO off)"
	@echo "  test-signed - Darwin: unit tests with -tags secitem (Security.framework, CGO)"
	@echo "  integration-test - Run Starlark integration tests against Jamf"
	@echo "  integration-test-acs7 - ACS profile id=7 same-tenant mutate/restore suite"
	@echo "  scan-acs-profiles - Read-only audit: profiles needing ACS hydrate/bind handling"
	@echo "  docs-serve - Local docs + script IDE (buddy serve --open)"
	@echo "  clean   - Remove all built executables"
	@echo "  help    - Show this help message"
	@echo ""
	@echo "Build tags (TAGS=…): noserve, nomcp, nolsp — omit optional features"
	@echo "CGO_ENABLED=$(CGO_ENABLED) by default (set CGO_ENABLED=1 to override)"
	@echo "$(SIGNED_TARGET): BUDDY_CODESIGN_IDENTITY required; BUDDY_CODESIGN_IDENTIFIER=$(BUDDY_CODESIGN_IDENTIFIER)"
	@echo "package: same as $(SIGNED_TARGET); branded installer + Finder icon; optional BUDDY_PKG_SIGN_IDENTITY (Developer ID Installer)"
.PHONY: help
