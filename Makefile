.PHONY: all build test lint clean coverage coverage-check coverage-report \
        security security-sast security-vuln security-secrets \
        release-plan release-bump release-notes release-publish \
        check example help

# Default target
all: build test lint

# Build
build:
	go build ./...

# Test
test:
	go test -race -v ./...

# Test with coverage
test-coverage:
	go test -race -coverprofile=coverage.out ./...

# Coverage analysis (coverctl)
coverage-check:
	coverctl check --fail-under=80

coverage-report:
	coverctl report --profile=coverage.out

coverage-debt:
	coverctl debt --profile=coverage.out

coverage-suggest:
	coverctl suggest --profile=coverage.out

# Security scanning (verdict)
security:
	verdict scan --path=.

security-sast:
	verdict sast --path=.

security-vuln:
	verdict vuln --path=.

security-secrets:
	verdict secrets --path=.

security-policy:
	verdict policy-check --path=.

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

# Lint
lint:
	golangci-lint run ./...

# Clean
clean:
	rm -f coverage.out
	go clean ./...

# Run example
example:
	go run ./example/fileops

# All checks (CI/CD)
check: lint test-coverage coverage-check security

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  Build & Test:"
	@echo "    build            - Build all packages"
	@echo "    test             - Run tests with race detection"
	@echo "    test-coverage    - Run tests with coverage profile"
	@echo ""
	@echo "  Coverage (coverctl):"
	@echo "    coverage-check   - Check coverage meets threshold (80%)"
	@echo "    coverage-report  - Generate coverage report"
	@echo "    coverage-debt    - Show coverage debt by domain"
	@echo "    coverage-suggest - Suggest optimal thresholds"
	@echo ""
	@echo "  Security (verdict):"
	@echo "    security         - Run all security scans"
	@echo "    security-sast    - Static analysis security testing"
	@echo "    security-vuln    - Vulnerability scanning"
	@echo "    security-secrets - Secret detection"
	@echo "    security-policy  - Policy compliance check"
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
	@echo "  Other:"
	@echo "    lint             - Run golangci-lint"
	@echo "    clean            - Remove generated files"
	@echo "    example          - Run the fileops example"
	@echo "    check            - Run all CI checks"
	@echo "    help             - Show this help"
