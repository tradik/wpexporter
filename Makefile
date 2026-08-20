# WordPress Export JSON - Makefile

# Colors for output
ifneq (,$(findstring xterm,${TERM}))
	BLACK        := $(shell tput -Txterm setaf 0)
	RED          := $(shell tput -Txterm setaf 1)
	GREEN        := $(shell tput -Txterm setaf 2)
	YELLOW       := $(shell tput -Txterm setaf 3)
	LIGHTPURPLE  := $(shell tput -Txterm setaf 4)
	PURPLE       := $(shell tput -Txterm setaf 5)
	BLUE         := $(shell tput -Txterm setaf 6)
	WHITE        := $(shell tput -Txterm setaf 7)
	RESET := $(shell tput -Txterm sgr0)
else
	BLACK        := ""
	RED          := ""
	GREEN        := ""
	YELLOW       := ""
	LIGHTPURPLE  := ""
	PURPLE       := ""
	BLUE         := ""
	WHITE        := ""
	RESET        := ""
endif

# Application name and version
APP_NAME := wpexportjson
# The VERSION file is the single source of truth for the release version, so a
# version bump is an explicit, reviewable line in the diff. `git describe` is only
# a fallback for builds outside a release checkout: during a release, CI creates
# the tag *after* building, so describe would stamp binaries with the previous
# tag plus a commit count. Overridable: make release VERSION=v1.2.3
VERSION := $(shell test -f VERSION && echo "v$$(cat VERSION)" || git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR := build
DIST_DIR := dist

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOENV := $(GOCMD) env
BINARY_NAME := $(APP_NAME)
XMLRPC_BINARY := wpxmlrpc
MCP_BINARY := wpmcp
UMBRELLA_BINARY := wpexporter

# Build flags
# The linker stamps one shared package rather than each command's own main, so
# a new binary cannot silently miss the version.
VERSION_PKG := github.com/tradik/wpexporter/internal/version
VERSION_VARS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME) -X $(VERSION_PKG).GitCommit=$(GIT_COMMIT)
LDFLAGS := -ldflags "$(VERSION_VARS) -s -w"
PROD_LDFLAGS := -ldflags "$(VERSION_VARS) -s -w -extldflags '-static'"

# Security scanner. gosec is a pinned tool dependency of the separate tools/
# module, so `make sec` builds the same binary CI does — every transitive
# version fixed by tools/go.sum — instead of whatever an ambient
# `go install …@latest` last left on PATH. The exclusions are the CI list, kept
# here so a local run and the pipeline can never disagree about what passes:
#   G703 path traversal   the tool writes where the operator told it to
#   G704 SSRF             it fetches the WordPress URL the operator gave it
#   G117 secret patterns  auth tokens are configuration, not leaked credentials
#   G122 filepath.Walk    the TOCTOU window is acceptable while zipping an export
GOSEC_BIN := $(BUILD_DIR)/gosec
GOSEC_EXCLUDE := G703,G704,G117,G122

.PHONY: help build clean test test-coverage deps run install dev lint vet sec check format build-prod release package packages docker-build docker-push version tag snap site site-serve site-check site-clean

help: ## Show this help message
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	@echo "${BLUE}Downloading dependencies...${RESET}"
	$(GOENV) -w GOPRIVATE=github.com/tradik/*
	$(GOENV) -w GONOSUMDB=github.com/tradik/*
	$(GOENV) -w GOPROXY=https://proxy.golang.org,direct
	$(GOMOD) download
	$(GOMOD) tidy

build: deps ## Build all applications for development
	@echo "${BLUE}Building $(APP_NAME) $(VERSION)...${RESET}"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/wpexportjson
	@echo "${GREEN}Build complete: $(BUILD_DIR)/$(BINARY_NAME)${RESET}"
	@echo "${BLUE}Building $(XMLRPC_BINARY) $(VERSION)...${RESET}"
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(XMLRPC_BINARY) ./cmd/wpxmlrpc
	@echo "${GREEN}Build complete: $(BUILD_DIR)/$(XMLRPC_BINARY)${RESET}"
	@echo "${BLUE}Building $(MCP_BINARY) $(VERSION)...${RESET}"
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(MCP_BINARY) ./cmd/wpmcp
	@echo "${GREEN}Build complete: $(BUILD_DIR)/$(MCP_BINARY)${RESET}"
	@echo "${BLUE}Building $(UMBRELLA_BINARY) $(VERSION)...${RESET}"
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(UMBRELLA_BINARY) ./cmd/wpexporter
	@echo "${GREEN}Build complete: $(BUILD_DIR)/$(UMBRELLA_BINARY)${RESET}"

clean: ## Clean build artifacts
	@echo "${YELLOW}Cleaning build artifacts...${RESET}"
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@echo "${GREEN}Clean complete${RESET}"

test: ## Run tests
	@echo "${BLUE}Running tests...${RESET}"
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage and generate HTML report
	@echo "${BLUE}Running tests with coverage...${RESET}"
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	$(GOCMD) tool cover -func=coverage.out
	@echo "${GREEN}Coverage report generated: coverage.html${RESET}"

lint: ## Run linter
	@echo "${BLUE}Running linter...${RESET}"
	golangci-lint run

vet: ## Run go vet
	@echo "${BLUE}Running go vet...${RESET}"
	$(GOCMD) vet ./...

sec: ## Run gosec security scanner
	@echo "${BLUE}Running gosec security scanner...${RESET}"
	@mkdir -p $(BUILD_DIR)
	@# Removed rather than overwritten: `go build -o` refuses a target it cannot
	@# recognise as its own output, so one interrupted build would otherwise
	@# leave `make sec` failing on a stale file until someone deleted it by hand.
	@rm -f $(GOSEC_BIN)
	@$(GOCMD) -C tools build -o "$(CURDIR)/$(GOSEC_BIN)" github.com/securego/gosec/v2/cmd/gosec
	@$(GOSEC_BIN) -exclude=$(GOSEC_EXCLUDE) ./...

check: vet lint sec test ## Run all checks (vet, lint, security, tests)
	@echo "${GREEN}All checks passed${RESET}"

format: ## Format code
	@echo "${BLUE}Formatting code...${RESET}"
	$(GOCMD) fmt ./...

run: build ## Build and run the application
	@echo "${BLUE}Running $(APP_NAME)...${RESET}"
	./$(BUILD_DIR)/$(BINARY_NAME)

install: ## Install all applications globally
	@echo "${BLUE}Installing $(APP_NAME)...${RESET}"
	$(GOCMD) install ./cmd/wpexportjson
	@echo "${BLUE}Installing $(XMLRPC_BINARY)...${RESET}"
	$(GOCMD) install ./cmd/wpxmlrpc
	@echo "${BLUE}Installing $(MCP_BINARY)...${RESET}"
	$(GOCMD) install ./cmd/wpmcp
	@echo "${GREEN}All applications installed${RESET}"

install-man: ## Install man pages
	@echo "${BLUE}Installing man pages...${RESET}"
	@mkdir -p /usr/local/share/man/man1
	@cp man/wpexportjson.1 /usr/local/share/man/man1/
	@mandb -q 2>/dev/null || true
	@echo "${GREEN}Man pages installed. Use: man wpexportjson${RESET}"

dev: ## Run in development mode with air
	@echo "${BLUE}Starting development server...${RESET}"
	air

dev-install: ## Install development dependencies
	@echo "${BLUE}Installing development dependencies...${RESET}"
	$(GOGET) github.com/air-verse/air@latest

build-prod: deps ## Build production binaries with optimizations
	@echo "${BLUE}Building production $(APP_NAME) $(VERSION)...${RESET}"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(PROD_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/wpexportjson
	@echo "${GREEN}Production build complete: $(BUILD_DIR)/$(BINARY_NAME)${RESET}"
	@echo "${BLUE}Building production $(XMLRPC_BINARY) $(VERSION)...${RESET}"
	CGO_ENABLED=0 $(GOBUILD) $(PROD_LDFLAGS) -o $(BUILD_DIR)/$(XMLRPC_BINARY) ./cmd/wpxmlrpc
	@echo "${GREEN}Production build complete: $(BUILD_DIR)/$(XMLRPC_BINARY)${RESET}"
	@echo "${BLUE}Building production $(MCP_BINARY) $(VERSION)...${RESET}"
	CGO_ENABLED=0 $(GOBUILD) $(PROD_LDFLAGS) -o $(BUILD_DIR)/$(MCP_BINARY) ./cmd/wpmcp
	@echo "${GREEN}Production build complete: $(BUILD_DIR)/$(MCP_BINARY)${RESET}"

release: deps ## Build release binaries for all platforms
	@echo "${BLUE}Building release binaries $(VERSION)...${RESET}"
	@mkdir -p $(DIST_DIR)
	@echo "${YELLOW}Building Linux AMD64...${RESET}"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-linux-amd64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-linux-amd64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-linux-amd64 ./cmd/wpexporter
	@echo "${YELLOW}Building Linux ARM64...${RESET}"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-linux-arm64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-linux-arm64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(PROD_LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-linux-arm64 ./cmd/wpexporter
	@echo "${YELLOW}Building FreeBSD AMD64...${RESET}"
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-freebsd-amd64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-freebsd-amd64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-freebsd-amd64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-freebsd-amd64 ./cmd/wpexporter
	@echo "${YELLOW}Building FreeBSD ARM64...${RESET}"
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-freebsd-arm64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-freebsd-arm64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-freebsd-arm64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-freebsd-arm64 ./cmd/wpexporter
	@echo "${YELLOW}Building macOS AMD64...${RESET}"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-darwin-amd64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-darwin-amd64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-darwin-amd64 ./cmd/wpexporter
	@echo "${YELLOW}Building macOS ARM64...${RESET}"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-darwin-arm64 ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-darwin-arm64 ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-darwin-arm64 ./cmd/wpexporter
	@echo "${YELLOW}Building Windows AMD64...${RESET}"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-windows-amd64.exe ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-windows-amd64.exe ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-windows-amd64.exe ./cmd/wpexporter
	@echo "${YELLOW}Building Windows ARM64...${RESET}"
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/wpexportjson
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(XMLRPC_BINARY)-windows-arm64.exe ./cmd/wpxmlrpc
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(MCP_BINARY)-windows-arm64.exe ./cmd/wpmcp
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(UMBRELLA_BINARY)-windows-arm64.exe ./cmd/wpexporter
	@echo "${GREEN}Release binaries built in $(DIST_DIR)/${RESET}"
	@ls -la $(DIST_DIR)/

package: release ## Create basic TAR.GZ distribution packages
	@echo "${BLUE}Creating distribution packages...${RESET}"
	@mkdir -p $(DIST_DIR)/packages
	@for os in linux freebsd darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			if [ -f $(DIST_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext ]; then \
				tar -czf $(DIST_DIR)/packages/$(APP_NAME)-$(VERSION)-$$os-$$arch.tar.gz \
					-C $(DIST_DIR) $(BINARY_NAME)-$$os-$$arch$$ext $(XMLRPC_BINARY)-$$os-$$arch$$ext $(MCP_BINARY)-$$os-$$arch$$ext \
					-C .. README.md CHANGELOG.md config.example.yaml; \
			fi; \
		done; \
	done
	@echo "${GREEN}Distribution packages created in $(DIST_DIR)/packages/${RESET}"
	@ls -la $(DIST_DIR)/packages/

packages: release ## Create DEB, RPM, and TGZ packages for distribution
	@echo "${BLUE}Creating DEB, RPM, and TGZ packages...${RESET}"
	@./scripts/build-packages.sh
	@echo "${GREEN}All packages created in $(DIST_DIR)/packages/${RESET}"

version: ## Show version information
	@echo "${BLUE}Version Information:${RESET}"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

tag: ## Create and push a new version tag
	@echo "${BLUE}Current version: $(VERSION)${RESET}"
	@read -p "Enter new version (e.g., v1.2.0): " NEW_VERSION; \
	if [ -n "$$NEW_VERSION" ]; then \
		echo "${YELLOW}Creating tag $$NEW_VERSION...${RESET}"; \
		git tag -a $$NEW_VERSION -m "Release $$NEW_VERSION"; \
		echo "${YELLOW}Pushing tag to remote...${RESET}"; \
		git push origin $$NEW_VERSION; \
		echo "${GREEN}Tag $$NEW_VERSION created and pushed${RESET}"; \
	else \
		echo "${RED}No version specified${RESET}"; \
	fi

snap: ## Build snap package locally
	@echo "${BLUE}Building snap package...${RESET}"
	@if command -v snapcraft >/dev/null 2>&1; then \
		snapcraft; \
	else \
		echo "${YELLOW}snapcraft not installed. Install with: sudo snap install snapcraft --classic${RESET}"; \
	fi

docker-build: ## Build Docker image
	@echo "${BLUE}Building Docker image...${RESET}"
	docker build -t $(APP_NAME):$(VERSION) .
	docker build -t $(APP_NAME):latest .
	@echo "${GREEN}Docker image built: $(APP_NAME):$(VERSION)${RESET}"

docker-push: docker-build ## Build and push Docker image
	@echo "${BLUE}Pushing Docker image...${RESET}"
	docker push $(APP_NAME):$(VERSION)
	docker push $(APP_NAME):latest
	@echo "${GREEN}Docker image pushed${RESET}"

docker-run: ## Run application in Docker container
	@echo "${BLUE}Running $(APP_NAME) in Docker...${RESET}"
	docker run --rm -it -v $(PWD)/export:/app/export $(APP_NAME):latest

security-scan: ## Run security scan on binaries
	@echo "${BLUE}Running security scan...${RESET}"
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "${YELLOW}govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest${RESET}"; \
	fi

benchmark: ## Run benchmarks (decoding, conversion, crawl — the hot paths)
	@echo "${BLUE}Running benchmarks...${RESET}"
	$(GOTEST) -run '^$$' -bench=. -benchmem -benchtime 300x -count 3 ./...

coverage: ## Generate test coverage report
	@echo "${BLUE}Generating coverage report...${RESET}"
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Coverage report generated: coverage.html${RESET}"

install-tools: ## Install development tools
	@echo "${BLUE}Installing development tools...${RESET}"
	$(GOGET) github.com/air-verse/air@latest
	$(GOGET) golang.org/x/vuln/cmd/govulncheck@latest
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "${GREEN}Development tools installed${RESET}"

# ── Documentation site ────────────────────────────────────────────────────────
# https://wpexporter.tradik.com/ — this repository's docs/ folder rendered with
# the bundled ssgtheme (templates/ssgtheme), configured by ./docs-site.yaml. No
# content is copied: content_sources reads docs/ in place, so editing a guide and
# rebuilding is the whole workflow.
#
# Needs SSG on PATH — `sudo snap install ssg`, or see https://ssg.tradik.com/install/.
SSG := ssg
SITE_CONFIG := docs-site.yaml
SITE_DIR := .site

site: ## 📚 Build the documentation site into .site/
	@command -v $(SSG) >/dev/null 2>&1 || { echo "${RED}ssg not found on PATH — see https://ssg.tradik.com/install/${RESET}"; exit 1; }
	@echo "${BLUE}📚 Building documentation site...${RESET}"
	@$(SSG) --config $(SITE_CONFIG)
	@echo "${GREEN}✅ Documentation site generated in $(SITE_DIR)/${RESET}"

site-serve: ## 🌐 Build the docs site and serve it on http://127.0.0.1:8888, rebuilding on change
	@command -v $(SSG) >/dev/null 2>&1 || { echo "${RED}ssg not found on PATH — see https://ssg.tradik.com/install/${RESET}"; exit 1; }
	@echo "${BLUE}🌐 Serving documentation site on http://127.0.0.1:8888 ...${RESET}"
	@$(SSG) --config $(SITE_CONFIG) --watch --http

site-check: ## 🔗 Build the docs site the way CI does — a dead internal link fails
	@command -v $(SSG) >/dev/null 2>&1 || { echo "${RED}ssg not found on PATH — see https://ssg.tradik.com/install/${RESET}"; exit 1; }
	@echo "${BLUE}🔗 Building documentation site with strict link checking...${RESET}"
	@$(SSG) --config $(SITE_CONFIG) --check-links=strict
	@echo "${GREEN}✅ No broken internal links${RESET}"

site-clean: ## 🧹 Remove the generated documentation site
	@echo "${YELLOW}Cleaning documentation site...${RESET}"
	@rm -rf $(SITE_DIR)
	@echo "${GREEN}Clean complete${RESET}"

ci: deps test lint security-scan ## Run CI pipeline locally
	@echo "${GREEN}CI pipeline completed successfully${RESET}"

all: clean deps test lint build-prod package ## Build everything for production release
	@echo "${GREEN}Production build completed${RESET}"
