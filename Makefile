
# Go compiler
GO := go
GOFLAGS := -v -buildvcs=false

BUILDTARGET := buddy

# Find all Go files with package main
GO_MAIN_FILES := $(shell find . -name "*.go" -type f -exec grep -l "^package main" {} \; | grep -v vendor | grep -v ".git" | sort)

# Version from git (tag or short hash)
BUDDY_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Buddy CLI (builds to ./buddy)
BUDDY_GO_FILES := $(shell find cmd/buddy internal docfs stdlib \( -name "*.go" -o -name "*.star" \) 2>/dev/null)
DOC_SOURCES := $(shell find docs examples .cursor/rules -type f \( -name '*.md' -o -name '*.star' -o -name '*.json' -o -name '*.mdc' \) 2>/dev/null) README.md AGENTS.md

.PHONY: sync-docfs
sync-docfs:
	@./scripts/sync-docfs.sh

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

# Run integration tests (requires Jamf test environment in config or --user override)
integration-test: $(BUILDTARGET)
	./$(BUILDTARGET) scripts/integration_test.star
.PHONY: integration-test

# Local docs + notebook UI (opens browser)
docs-serve: $(BUILDTARGET)
	./$(BUILDTARGET) serve --open
.PHONY: docs-serve

# Clean all built executables
clean:
	@echo "Cleaning executables..."
	@rm -f $(BUILDTARGET)
.PHONY: clean

# Help target
help:
	@echo "Generic Go Build Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  $(BUILDTARGET)   - Sync embedded docs + build $(BUILDTARGET) CLI"
	@echo "  sync-docfs - Copy docs/AGENTS/examples/rules into docfs/content for go:embed"
	@echo "  install   - Install $(BUILDTARGET) CLI to ~/go/bin/"
	@echo "  test      - Run unit tests"
	@echo "  integration-test - Run Starlark integration tests against Jamf"
	@echo "  docs-serve - Local docs + notebook (buddy serve --open)"
	@echo "  clean   - Remove all built executables"
	@echo "  help    - Show this help message"
.PHONY: help
