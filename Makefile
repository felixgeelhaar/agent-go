.PHONY: all build test lint clean coverage coverage-check coverage-report \
        security security-secrets security-diff \
        release-plan release-bump release-notes release-publish \
        check example help hooks \
        workspace-sync workspace-tidy contrib-build contrib-test contrib-lint

# Default target
all: build test lint

# Build (core module packages only — use contrib-build for contrib/)
build:
	go build ./...

# Build core only
build-core:
	go build -mod=mod ./domain/... ./application/... ./interfaces/... ./infrastructure/...

# Build all contrib modules individually
contrib-build:
	@echo "Building contrib modules..."
	@for dir in contrib/*/; do \
		echo "Building $$dir"; \
		(cd "$$dir" && go build ./...) || exit 1; \
	done
	@echo "All contrib modules built successfully"

# Test (core module packages only — use contrib-test for contrib/)
test:
	go test -race -v ./...

# Test core only (explicit package list)
test-core:
	go test -race -v ./domain/... ./application/... ./interfaces/... ./infrastructure/... ./test/...

# Test all contrib modules individually
# Storage modules use -short (external service gates), matching CI.
contrib-test:
	@echo "Testing contrib modules..."
	@for dir in contrib/*/; do \
		m=$$(basename "$$dir"); \
		echo "Testing $$dir"; \
		if echo "$$m" | grep -q '^storage-'; then \
			(cd "$$dir" && go test -race -short -timeout=5m ./...) || exit 1; \
		else \
			(cd "$$dir" && go test -race -timeout=5m ./...) || exit 1; \
		fi; \
	done
	@echo "All contrib module tests passed"

# Test with coverage (core module)
test-coverage:
	go test -race -coverprofile=coverage.out ./...

# Coverage analysis (coverctl) — thresholds in .coverctl.yaml
coverage-check:
	coverctl check

coverage-report:
	coverctl report

coverage-debt:
	coverctl debt

coverage-suggest:
	coverctl suggest

# Security scanning (nox)
security:
	nox scan . --severity-threshold=high

security-secrets:
	nox scan . --history --history-depth=50 --severity-threshold=high

security-diff:
	nox diff --base=main --head=HEAD

# Release management (relicta)
release-status:
	relicta status

release-plan:
	relicta plan --analyze

release-bump:
	relicta bump --level=auto

release-notes:
	relicta notes --ai

release-approve:
	relicta approve

release-publish:
	relicta publish

release-validate:
	relicta validate-release

# Lint (core module packages only — use contrib-lint for contrib/)
lint:
	golangci-lint run ./...

# Lint core only
lint-core:
	golangci-lint run ./domain/... ./application/... ./interfaces/... ./infrastructure/...

# Lint all contrib modules
contrib-lint:
	@echo "Linting contrib modules..."
	@for dir in contrib/*/; do \
		echo "Linting $$dir"; \
		(cd "$$dir" && golangci-lint run ./...) || exit 1; \
	done
	@echo "All contrib modules linted successfully"
# Clean
clean:
	rm -f coverage.out
	go clean ./...

# Run example
example:
	go run ./example/fileops

# Workspace management
workspace-sync:
	go work sync

workspace-tidy:
	@echo "Tidying all modules..."
	go mod tidy
	@for dir in contrib/*/; do \
		echo "Tidying $$dir"; \
		(cd "$$dir" && go mod tidy) || exit 1; \
	done
	@echo "All modules tidied"

# Verify workspace
workspace-verify:
	@echo "Verifying workspace..."
	go work sync
	go build ./...
	@echo "Workspace verified successfully"

# Create new contrib module
new-contrib:
	@scripts/new-contrib.sh $(NAME)

# Install git hooks
hooks:
	@echo "Installing git hooks..."
	@cp scripts/pre-commit .git/hooks/pre-commit
	@cp scripts/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed successfully."
	@echo "  pre-commit: gofmt, go vet, golangci-lint, nox, build, core tests"
	@echo "  pre-push:   race tests, coverage check, nox security scan"

# Documentation generation
docs: docs-packs docs-api

docs-packs:
	@scripts/generate-pack-index.sh

docs-api:
	@scripts/generate-api-docs.sh

# Integration tests
test-integration:
	go test -race -v -tags=integration ./test/integration/...

# All checks (CI/CD)
check: lint test-coverage coverage-check security

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  Build & Test:"
	@echo "    build            - Build all packages (workspace)"
	@echo "    build-core       - Build core packages only"
	@echo "    contrib-build    - Build all contrib modules individually"
	@echo "    test             - Run tests with race detection (workspace)"
	@echo "    test-core        - Run core tests only"
	@echo "    contrib-test     - Test all contrib modules individually"
	@echo "    contrib-lint     - Lint all contrib modules individually"
	@echo "    test-coverage    - Run tests with coverage profile"
	@echo ""
	@echo "  Coverage (coverctl):"
	@echo "    coverage-check   - Check coverage meets threshold (80%)"
	@echo "    coverage-report  - Generate coverage report"
	@echo "    coverage-debt    - Show coverage debt by domain"
	@echo "    coverage-suggest - Suggest optimal thresholds"
	@echo ""
	@echo "  Security (nox):"
	@echo "    security         - Run nox security scan (high+ severity)"
	@echo "    security-secrets - Scan git history for leaked secrets"
	@echo "    security-diff    - Show new findings vs main branch"
	@echo ""
	@echo "  Release (relicta):"
	@echo "    release-status   - Show current release state"
	@echo "    release-plan     - Analyze commits and suggest version"
	@echo "    release-bump     - Calculate and set next version"
	@echo "    release-notes    - Generate release notes"
	@echo "    release-approve  - Approve release for publishing"
	@echo "    release-publish  - Execute release (create tags, run plugins)"
	@echo "    release-validate - Run pre-flight validation"
	@echo ""
	@echo "  Workspace Management:"
	@echo "    workspace-sync   - Sync go.work with all modules"
	@echo "    workspace-tidy   - Run go mod tidy on all modules"
	@echo "    workspace-verify - Verify workspace builds correctly"
	@echo ""
	@echo "  Other:"
	@echo "    hooks            - Install pre-commit and pre-push git hooks"
	@echo "    lint             - Run golangci-lint (workspace)"
	@echo "    lint-core        - Run golangci-lint on core only"
	@echo "    clean            - Remove generated files"
	@echo "    example          - Run the fileops example"
	@echo "    check            - Run all CI checks"
	@echo "    help             - Show this help"
