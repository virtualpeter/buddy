
# Go compiler
GO := go
GOFLAGS := -v -buildvcs=false
# Default builds are CGO-free (Keychain uses security(1)).
# make buddy-darwin-arm64 is Darwin/arm64-only: CGO + Security.framework + Developer ID.
CGO_ENABLED ?= 0
export CGO_ENABLED
# Signed release: identifier must stay stable across releases.
# Identity defaults to the first valid Developer ID Application from
# `security find-identity -v -p codesigning`. Set BUDDY_CODESIGN_IDENTITY to override.
ifeq ($(BUDDY_CODESIGN_IDENTITY),)
BUDDY_CODESIGN_IDENTITY := $(shell security find-identity -v -p codesigning 2>/dev/null | awk -F '"' '/Developer ID Application/ { print $$2; exit }')
endif
BUDDY_CODESIGN_IDENTIFIER ?= virtualpete.buddy.cli
BUDDY_APP_IDENTIFIER ?= virtualpete.buddy
# Team for the Agent's Xcode automatic signing; derived from the same identity.
ifeq ($(BUDDY_TEAM_ID),)
BUDDY_TEAM_ID := $(shell printf '%s' "$(BUDDY_CODESIGN_IDENTITY)" | sed -nE 's/.*\(([A-Z0-9]+)\)$$/\1/p')
endif

# All build artefacts land under dist/ (override with DIST=…).
DIST ?= dist
BUILDTARGET := $(DIST)/buddy
SIGNED_TARGET := $(DIST)/buddy-darwin-arm64
LITE_TARGET := $(DIST)/buddy-lite
LINUX_TARGET := $(DIST)/buddy-linux-amd64
WINDOWS_TARGET := $(DIST)/buddy-windows-amd64.exe
APP_TARGET := $(DIST)/Buddy.app
AGENT_TARGET := $(DIST)/BuddyAgent.app

# Find all Go files with package main
GO_MAIN_FILES := $(shell find . -name "*.go" -type f -exec grep -l "^package main" {} \; | grep -v vendor | grep -v ".git" | sort)

# Version from git (tag or short hash)
BUDDY_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Buddy CLI (builds to dist/buddy)
BUDDY_GO_FILES := $(shell find cmd/buddy internal docfs stdlib \( -name "*.go" -o -name "*.buddy" -o -name "buddy.png" \) 2>/dev/null)
DOC_SOURCES := $(shell find docs examples -type f \( -name '*.md' -o -name '*.buddy' -o -name '*.json' \) 2>/dev/null) README.md

$(DIST):
	mkdir -p $(DIST)

.PHONY: default
default: $(SIGNED_TARGET)

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

.PHONY: buddy
buddy: $(BUILDTARGET)

$(BUILDTARGET): sync-docfs $(BUDDY_GO_FILES) docfs/content/README.embedded | $(DIST)
	@echo "Building buddy (version $(BUDDY_VERSION)$(if $(TAGS), tags=$(TAGS)), CGO_ENABLED=$(CGO_ENABLED))..."
	$(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(BUILDTARGET) ./cmd/buddy
	@! santactl status>/dev/null|| santactl fileinfo $(BUILDTARGET)|grep Rule|egrep '(Allowed|None)' >/dev/null|| rm -f $(BUILDTARGET)
	@ls -lh $(BUILDTARGET)

# Automation-oriented binary without serve/mcp/lsp (same CLI + buddyscript).
.PHONY: buddy-lite
buddy-lite: sync-docfs | $(DIST)
	@echo "Building buddy-lite (version $(BUDDY_VERSION), tags=noserve,nomcp,nolsp, CGO_ENABLED=$(CGO_ENABLED))..."
	$(GO) build $(GOFLAGS) -tags 'noserve,nomcp,nolsp' -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(LITE_TARGET) ./cmd/buddy
	@ls -lh $(LITE_TARGET)

# Darwin/arm64 release binary: Security.framework Keychain (service virtualpete.buddy.cli, label Buddy).
# Signs with BUDDY_CODESIGN_IDENTITY (Developer ID Application; default from find-identity). Does not clobber dist/buddy.
.PHONY: buddy-darwin-arm64
buddy-darwin-arm64: $(SIGNED_TARGET)

$(SIGNED_TARGET): sync-docfs | $(DIST)
	@test "$$(uname -s)" = Darwin || { echo "make buddy-darwin-arm64 is macOS-only"; exit 1; }
	@test "$$(uname -m)" = arm64 || { echo "make buddy-darwin-arm64 is arm64-only"; exit 1; }
	@test -n "$(BUDDY_CODESIGN_IDENTITY)" || { echo "no Developer ID Application identity found; set BUDDY_CODESIGN_IDENTITY or install a Developer ID"; exit 1; }
	@echo "Building $(SIGNED_TARGET) (version $(BUDDY_VERSION)$(if $(TAGS), tags=secitem,$(TAGS), tags=secitem), CGO_ENABLED=1, identity=$(BUDDY_CODESIGN_IDENTITY))..."
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -tags 'secitem$(if $(TAGS),$(comma)$(TAGS))' -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(SIGNED_TARGET) ./cmd/buddy
	codesign --force --sign "$(BUDDY_CODESIGN_IDENTITY)" --options runtime --identifier "$(BUDDY_CODESIGN_IDENTIFIER)" $(SIGNED_TARGET)
	@info=$$(codesign -dv --verbose=2 $(SIGNED_TARGET) 2>&1); \
	printf '%s\n' "$$info"; \
	case "$$info" in *Signature=adhoc*) echo "$(SIGNED_TARGET): ad-hoc signature rejected; use a Developer ID"; exit 1;; esac; \
	case "$$info" in *TeamIdentifier=not\ set*) echo "$(SIGNED_TARGET): TeamIdentifier not set"; exit 1;; esac; \
	echo "$$info" | grep -E 'TeamIdentifier=[A-Z0-9]+' >/dev/null || { echo "$(SIGNED_TARGET): missing TeamIdentifier"; exit 1; }
	@ls -lh $(SIGNED_TARGET)

# Cross-compile Linux amd64 (pure Go; no CGO).
.PHONY: buddy-linux
buddy-linux: sync-docfs | $(DIST)
	@echo "Building $(LINUX_TARGET) (version $(BUDDY_VERSION), CGO_ENABLED=0)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(LINUX_TARGET) ./cmd/buddy
	@ls -lh $(LINUX_TARGET)

# Cross-compile Windows amd64 (pure Go; config under %APPDATA%\buddy).
.PHONY: buddy-windows
buddy-windows: sync-docfs | $(DIST)
	@echo "Building $(WINDOWS_TARGET) (version $(BUDDY_VERSION), CGO_ENABLED=0)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) $(GO_TAGS) -ldflags "-X main.Version=$(BUDDY_VERSION)" -o $(WINDOWS_TARGET) ./cmd/buddy
	@ls -lh $(WINDOWS_TARGET)

# Compare binary sizes for feature-tag combinations (writes /tmp/buddy-size-*).
.PHONY: size-report
size-report: sync-docfs
	@./scripts/buddy-size-report.sh

install: $(BUILDTARGET)
	rm -f ~/go/bin/buddy
	cp $(BUILDTARGET) ~/go/bin/buddy
.PHONY: install

# Buddy Agent: optional menu-bar app that owns its own integrations and mints
# tokens for the CLI. Developer ID signing is not cosmetic here — the embedded
# provisioning profile is what authorizes the Keychain userPresence ACL.
AGENT_SWIFT_FILES := $(shell find macos/BuddyAgent/Sources -name '*.swift' 2>/dev/null)

.PHONY: agent-app
agent-app: $(AGENT_TARGET)

$(AGENT_TARGET): $(AGENT_SWIFT_FILES) macos/BuddyAgent/BuddyAgent.xcodeproj/project.pbxproj macos/BuddyAgent/BuddyAgent.entitlements macos/BuddyAgent/Resources/Info.plist scripts/build-agent-app.sh scripts/gen-agent-version.sh | $(DIST)
	@test "$$(uname -s)" = Darwin || { echo "make agent-app is macOS-only"; exit 1; }
	./scripts/build-agent-app.sh "$(AGENT_TARGET)" "$(PKG_VERSION)" "$(BUDDY_TEAM_ID)"

# Unsigned SwiftPM loop for editing the Agent: no entitlements, so the keychain
# stays legacy and presence is simulated. Not shippable; see agent-app.
.PHONY: agent-dev
agent-dev:
	@test "$$(uname -s)" = Darwin || { echo "make agent-dev is macOS-only"; exit 1; }
	./scripts/gen-agent-version.sh
	cd macos/BuddyAgent && swift build
	@echo "Run: BUDDY_AGENT_ALLOW_UNSIGNED=1 macos/BuddyAgent/.build/debug/BuddyAgent"

# Buddy.app: CLI at Contents/Helpers/buddy (codesign id virtualpete.buddy.cli).
# MacOS/Buddy is a SwiftUI shell hosting buddy serve (WKWebView). PATH symlinks Helpers.
# The Agent rides along at Contents/Library/LoginItems.
BUDDY_SWIFT_FILES := $(shell find macos/Buddy/Sources -name '*.swift' 2>/dev/null)
.PHONY: app
app: $(APP_TARGET)

$(APP_TARGET): $(SIGNED_TARGET) $(AGENT_TARGET) $(BUDDY_SWIFT_FILES) macos/Buddy/Buddy.xcodeproj/project.pbxproj macos/Buddy/Buddy.entitlements macos/Buddy/Resources/Info.plist scripts/gen-buddy-version.sh packaging/Info.plist.in scripts/build-buddy-app.sh | $(DIST)
	@test "$$(uname -s)" = Darwin || { echo "make app is macOS-only"; exit 1; }
	BUDDY_CODESIGN_IDENTIFIER="$(BUDDY_CODESIGN_IDENTIFIER)" BUDDY_APP_IDENTIFIER="$(BUDDY_APP_IDENTIFIER)" \
		./scripts/build-buddy-app.sh "$(SIGNED_TARGET)" "$(APP_TARGET)" "$(PKG_VERSION)" "$(BUDDY_CODESIGN_IDENTITY)" "$(AGENT_TARGET)"

# Install Buddy.app to ~/Applications and symlink ~/go/bin/buddy at the helper.
install-signed: $(APP_TARGET)
	mkdir -p ~/Applications ~/go/bin
	rm -rf ~/Applications/Buddy.app
	ditto $(APP_TARGET) ~/Applications/Buddy.app
	rm -f ~/go/bin/buddy
	ln -s ~/Applications/Buddy.app/Contents/Helpers/buddy ~/go/bin/buddy
.PHONY: install-signed

# Branded product archive: Buddy.app → /Applications plus postinstall symlink
# /usr/local/bin/buddy → Contents/Helpers/buddy (installer UI + Finder icon).
# pkgbuild --version must be period-delimited integers (strip v prefix and -dirty / -N-gSHA).
# Optional: BUDDY_PKG_SIGN_IDENTITY='Developer ID Installer: …' to productsign the pkg.
PKG_VERSION ?= $(shell v=$$(printf '%s' "$(BUDDY_VERSION)" | sed -E 's/^v//;s/-.*//'); printf '%s\n' "$${v:-0.0.0}")
PKG_TARGET ?= $(DIST)/buddy-$(PKG_VERSION).pkg
# Shared by package and notarize so the two cannot drift.
BUILD_PKG = ./scripts/build-pkg.sh "$(APP_TARGET)" "$(PKG_TARGET)" "$(PKG_VERSION)" "$(BUDDY_APP_IDENTIFIER)" "$(BUDDY_PKG_SIGN_IDENTITY)"
.PHONY: package
package: $(APP_TARGET) | $(DIST)
	@test "$$(uname -s)" = Darwin || { echo "make package is macOS-only"; exit 1; }
	@$(BUILD_PKG)

# Notarize and staple. Two stages, in the order that leaves both artefacts
# stapled: Buddy.app first, then a pkg built from the already-stapled app.
# Needs a notarytool keychain profile and a Developer ID Installer identity:
#   xcrun notarytool store-credentials <profile> --apple-id … --team-id …
# The scripts read BUDDY_NOTARY_PROFILE to refuse anything the notary would
# reject anyway (no secure timestamp, unsigned pkg).
export BUDDY_NOTARY_PROFILE
.PHONY: notarize
notarize:
	@test "$$(uname -s)" = Darwin || { echo "make notarize is macOS-only"; exit 1; }
	@test -n "$(BUDDY_NOTARY_PROFILE)" || { echo "set BUDDY_NOTARY_PROFILE=<notarytool keychain profile>"; exit 1; }
	@test -n "$(BUDDY_PKG_SIGN_IDENTITY)" || { echo "set BUDDY_PKG_SIGN_IDENTITY='Developer ID Installer: …'"; exit 1; }
	$(MAKE) $(APP_TARGET)
	./scripts/notarize.sh "$(APP_TARGET)" "$(BUDDY_NOTARY_PROFILE)"
	@# Deliberately not a recursive 'make package': sync-docfs is phony, so the
	@# signed CLI and then Buddy.app are always considered out of date.
	@# Re-entering make here would rebuild the app and throw away the ticket
	@# just stapled to it, leaving the pkg carrying an unstapled app — the thing
	@# this two-stage order exists to prevent. Package the app on disk instead.
	@$(BUILD_PKG)
	./scripts/notarize.sh "$(PKG_TARGET)" "$(BUDDY_NOTARY_PROFILE)"

# Finish a notarize run that was interrupted while waiting on Apple's queue.
# stapler fetches the ticket by cdhash, so this needs no submission id and does
# not resubmit — it only works once Apple has accepted, and only on the exact
# files that were submitted. Rebuilding first invalidates the ticket.
.PHONY: staple
staple:
	@test "$$(uname -s)" = Darwin || { echo "make staple is macOS-only"; exit 1; }
	@found=0; failed=0; \
	for artefact in "$(APP_TARGET)" "$(PKG_TARGET)"; do \
		test -e "$$artefact" || continue; \
		found=1; \
		echo "Stapling $$artefact..."; \
		if xcrun stapler staple "$$artefact"; then \
			xcrun stapler validate "$$artefact"; \
		else \
			failed=1; \
			echo "make staple: no ticket for $$artefact yet"; \
			echo "make staple: if it was rebuilt since submission its cdhash changed — notarize it again"; \
		fi; \
	done; \
	test "$$found" = 1 || { echo "make staple: nothing to staple; build and notarize first"; exit 1; }; \
	test "$$failed" = 0 || { echo "make staple: check 'xcrun notarytool history --keychain-profile \$$BUDDY_NOTARY_PROFILE'"; exit 1; }

# Abort if the working tree has uncommitted or untracked files (so artefacts
# are not stamped -dirty). Used by make release; buddy publish does the same.
.PHONY: require-clean
require-clean:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "make release: working tree is dirty; commit or stash first"; \
		git status --porcelain >&2; \
		exit 1; \
	fi

# Clean dist/, then branded pkg + linux/windows cross-builds (macOS host required for pkg).
# Notarizes when BUDDY_NOTARY_PROFILE is set, which also reorders app/pkg.
# App bundles stay out of dist/ after a successful run (they already live in the pkg).
.PHONY: release
release: require-clean
	@test "$$(uname -s)" = Darwin || { echo "make release is macOS-only (needs package)"; exit 1; }
	$(MAKE) clean
	@if [ -n "$(BUDDY_NOTARY_PROFILE)" ]; then $(MAKE) notarize; else $(MAKE) package; fi
	$(MAKE) buddy-linux buddy-windows
	@rm -rf "$(APP_TARGET)" "$(AGENT_TARGET)"
	@echo "Release artefacts in $(DIST)/ (Buddy.app / BuddyAgent.app removed; they are inside the pkg):"
	@ls -lh $(DIST)

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
	$(BUILDTARGET) -y scripts/integration_test.buddy
.PHONY: integration-test

# ACS hydrate shake-down against a live Classic profile (default id=7; same-tenant mutate+restore).
# When hydrate cannot resolve the ACS payload UUID from HTML, set e.g.:
#   BUDDY_ACS_PAYLOAD_UUID_7=<FIELD_EXTERNAL_CONFIGPROFILE_ID> make integration-test-acs7
# or set TEST_ACS_PAYLOAD_UUID in scripts/integration_test_acs_profile7.buddy
integration-test-acs7: $(BUILDTARGET)
	$(BUILDTARGET) -y scripts/integration_test_acs_profile7.buddy
.PHONY: integration-test-acs7

# Read-only: classify profiles that need ACS hydrate / UUID bind
scan-acs-profiles: $(BUILDTARGET)
	$(BUILDTARGET) -y scripts/scan_acs_profile_handling.buddy
.PHONY: scan-acs-profiles

# Local docs + config + script IDE (opens browser)
docs-serve: $(BUILDTARGET)
	$(BUILDTARGET) serve --open
.PHONY: docs-serve

# Clean all built executables
clean:
	@echo "Cleaning $(DIST)/ and legacy root artefacts..."
	@rm -rf $(DIST)
	@rm -rf Buddy.app
	@rm -f buddy buddy-signed buddy-lite buddy-linux-amd64 buddy-windows-amd64.exe buddy-darwin-arm64 buddy-Darwin-arm64 buddy-*.pkg *.pkg.unsigned
.PHONY: clean

# Help target
help:
	@echo "Generic Go Build Makefile"
	@echo ""
	@echo "Targets (artefacts under $(DIST)/):"
	@echo "  buddy   - Sync embedded docs + build $(BUILDTARGET)"
	@echo "  buddy-darwin-arm64 - Darwin/arm64 helper: -tags secitem + Developer ID → $(SIGNED_TARGET)"
	@echo "  app / Buddy.app - Wrap helper in $(APP_TARGET) (Helpers/buddy + Swift MacOS/Buddy + Agent)"
	@echo "  agent-app - Developer ID Buddy Agent → $(AGENT_TARGET) (Xcode; presence ACL)"
	@echo "  agent-dev - Unsigned SwiftPM build of the Agent (legacy keychain, simulated presence)"
	@echo "  buddy-lite - Build with TAGS=noserve,nomcp,nolsp → $(LITE_TARGET)"
	@echo "  buddy-linux - Cross-compile linux/amd64 → $(LINUX_TARGET)"
	@echo "  buddy-windows - Cross-compile windows/amd64 → $(WINDOWS_TARGET)"
	@echo "  size-report - Build tag variants and print size contribution table"
	@echo "  sync-docfs - Copy docs/AGENTS/examples/rules into docfs/content for go:embed"
	@echo "  sync-platform-schema - Refresh vendored Jamf Platform API OpenAPI specs"
	@echo "  install   - Install $(BUILDTARGET) CLI to ~/go/bin/buddy"
	@echo "  install-signed - Buddy.app → ~/Applications; symlink ~/go/bin/buddy → Helpers/buddy"
	@echo "  package   - Branded pkg: /Applications/Buddy.app + /usr/local/bin/buddy symlink → $(DIST)/buddy-<version>.pkg"
	@echo "  notarize  - Notarize + staple Buddy.app then the pkg (BUDDY_NOTARY_PROFILE + BUDDY_PKG_SIGN_IDENTITY)"
	@echo "  staple    - Staple $(APP_TARGET) and the pkg after an interrupted notarize wait (no rebuild)"
	@echo "  release   - require a clean tree, then clean + package (or notarize) + linux/windows; strip Buddy.app / BuddyAgent.app from dist"
	@echo "  test      - Run unit tests (default Keychain / CGO off)"
	@echo "  test-signed - Darwin: unit tests with -tags secitem (Security.framework, CGO)"
	@echo "  integration-test - Run Starlark integration tests against Jamf"
	@echo "  integration-test-acs7 - ACS profile id=7 same-tenant mutate/restore suite"
	@echo "  scan-acs-profiles - Read-only audit: profiles needing ACS hydrate/bind handling"
	@echo "  docs-serve - Local docs + config + script IDE (buddy serve --open)"
	@echo "  clean   - Remove $(DIST)/ and leftover root build artefacts"
	@echo "  help    - Show this help message"
	@echo ""
	@echo "Build tags (TAGS=…): noserve, nomcp, nolsp — omit optional features"
	@echo "CGO_ENABLED=$(CGO_ENABLED) by default (set CGO_ENABLED=1 to override)"
	@echo "DIST=$(DIST) (override with DIST=…)"
	@echo "buddy-darwin-arm64: BUDDY_CODESIGN_IDENTITY defaults to first Developer ID Application (security find-identity); override with BUDDY_CODESIGN_IDENTITY=…; BUDDY_CODESIGN_IDENTIFIER=$(BUDDY_CODESIGN_IDENTIFIER)"
	@echo "app: BUDDY_APP_IDENTIFIER=$(BUDDY_APP_IDENTIFIER)"
	@echo "package: $(APP_TARGET) into a branded installer; optional BUDDY_PKG_SIGN_IDENTITY (Developer ID Installer)"
	@echo "agent-app: Xcode automatic signing, BUDDY_TEAM_ID=$(BUDDY_TEAM_ID) (derived from the Developer ID identity)"
	@echo "notarize: BUDDY_NOTARY_PROFILE=<notarytool keychain profile>; required for a stapled release"
	@echo "notarize: BUDDY_NOTARY_WAIT=30m bounds the wait on Apple's queue; on timeout resume with 'make staple'"
.PHONY: help
