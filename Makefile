BIN              ?= build/analysis
SOMETHING_JSON   ?= build/something-json
PDF_STORE        ?= build/pdf-store
DOC_CHECK        ?= build/doccheck
COVERAGE_CHECK   ?= build/coveragecheck
GO               ?= go
GOWD             := src
DB               ?= corpus.metadata.db
ADDR             ?= 127.0.0.1:8080
ASSETS_DIR       ?=
CONFIG           ?= config/workspace.something
WORKSPACE        ?=
FRESH            ?=
PACKAGE          ?= ./...
TEST             ?=
COVERAGE_OUT     ?= build/coverage/coverage.out
COVERAGE_POLICY  ?= config/coverage_policy.something
FIXTURE_DB       ?= src/server/testdata/workspace.fixture.db
BROWSER          ?= chromium
WORKERS          ?=
TEST_FILE        ?=
BROWSERS         ?= chromium firefox webkit
REPORT_DIR       ?=
DB_METADATA      ?= corpus.metadata.db
DB_PDF           ?= corpus.pdf.db

.DEFAULT_GOAL := help

.PHONY: help all build tools something-json pdf-store doccheck coveragecheck docs-catalog-update docs-state-update clean fmt format-check vet check check-docs test test-go test-unit test-functional test-integration test-all test-race test-docs test-e2e test-e2e-live coverage fixture run serve dev frontend-install frontend-browsers frontend-vendor test-frontend test-frontend-all test-frontend-headed test-frontend-debug test-frontend-visual test-frontend-unit frontend-report database-backup

help: ## List supported local development commands, variables, and examples.
	@printf '%s\n' 'Research analysis local development interface'
	@printf '%s\n' ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  make %-26s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '%s\n' ''
	@printf '%s\n' 'Common variables:'
	@printf '%s\n' '  DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=src/server/frontend'
	@printf '%s\n' '  CONFIG=config/workspace.something WORKSPACE=search_id@revision FRESH=1'
	@printf '%s\n' '  PACKAGE=./server TEST=^TestName$$ COVERAGE_OUT=build/coverage/coverage.out COVERAGE_POLICY=config/coverage_policy.something'
	@printf '%s\n' '  BROWSER=chromium WORKERS=4 TEST_FILE=tests/viewer.spec.cjs'
	@printf '%s\n' '  E2E_LIVE=1 explicitly enables real Crossref, OpenAlex, and ORCID requests'
	@printf '%s\n' ''
	@printf '%s\n' 'Examples:'
	@printf '%s\n' '  make run DB=corpus.metadata.db CONFIG=config/workspace.something FRESH=1'
	@printf '%s\n' '  make serve DB=corpus.metadata.db ADDR=127.0.0.1:8090'
	@printf '%s\n' '  make test-go PACKAGE=./server TEST=^TestGraph$$'
	@printf '%s\n' '  make test-frontend BROWSER=chromium WORKERS=4 TEST_FILE=tests/viewer.spec.cjs'
	@printf '%s\n' '  make test-e2e'
	@printf '%s\n' '  make test-e2e-live E2E_LIVE=1'

all: build ## Build the normal embedded-asset executable.

build: ## Build build/analysis; no Go default-output binaries are created.
	@mkdir -p "$(dir $(BIN))"
	cd $(GOWD) && $(GO) build -o "../$(BIN)" .

tools: something-json pdf-store doccheck coveragecheck ## Build every maintained Go tool under build/.

something-json: ## Build build/something-json, a tool to inspect .something configs.
	@mkdir -p "$(dir $(SOMETHING_JSON))"
	cd $(GOWD) && $(GO) build -o "../$(SOMETHING_JSON)" ./tools/something-json/

pdf-store: ## Build build/pdf-store, the validated manual PDF maintenance tool.
	@mkdir -p "$(dir $(PDF_STORE))"
	cd $(GOWD) && $(GO) build -o "../$(PDF_STORE)" ./tools/pdf-store/

doccheck: ## Build build/doccheck, the documentation consistency tool.
	@mkdir -p "$(dir $(DOC_CHECK))"
	cd $(GOWD) && $(GO) build -o "../$(DOC_CHECK)" ./tools/doccheck/

coveragecheck: ## Build build/coveragecheck, the coverage policy tool.
	@mkdir -p "$(dir $(COVERAGE_CHECK))"
	cd $(GOWD) && $(GO) build -o "../$(COVERAGE_CHECK)" ./tools/coveragecheck/

docs-catalog-update: doccheck ## Regenerate docs/PROJECT_CATALOG.md from maintained source declarations.
	./$(DOC_CHECK) catalog update

docs-state-update: doccheck ## Acknowledge reviewed docs by updating docs/DOC-STATE.md hashes.
	./$(DOC_CHECK) state update

fmt: ## Format all Go source files in src/.
	@find "$(GOWD)" -name '*.go' -type f -print0 | xargs -0 gofmt -w

format-check: ## Fail if any Go source file needs gofmt.
	@unformatted="$$(find "$(GOWD)" -name '*.go' -type f -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n%s\n' 'Run make fmt; unformatted Go files:' "$$unformatted"; \
		exit 1; \
	fi

vet: ## Run go vet across the module.
	cd $(GOWD) && $(GO) vet ./...

check: format-check vet check-docs ## Run local Go and documentation checks.

check-docs: doccheck ## Validate documentation links, format, catalog, migrations, and acknowledged state.
	./$(DOC_CHECK) check

test: test-go ## Run all Go tests without test-result caching.

test-unit: ## Run unit tests only (requires -tags=unit).
	cd $(GOWD) && $(GO) test -tags=unit ./... -count=1

test-functional: ## Run functional tests only (requires -tags=functional).
	cd $(GOWD) && $(GO) test -tags=functional ./... -count=1

test-integration: ## Run integration tests only (requires -tags=integration).
	cd $(GOWD) && $(GO) test -tags=integration ./... -count=1

test-all: ## Run all tests (unit + functional + integration). Space in tags means OR.
	cd $(GOWD) && $(GO) test -tags='unit functional integration' ./... -count=1

test-go: ## Run Go tests. Override PACKAGE and TEST for focused tests.
	cd $(GOWD) && $(GO) test -tags='unit functional integration' $(PACKAGE) -count=1 $(if $(TEST),-run '$(TEST)')

test-race: ## Run all Go tests with the race detector.
	cd $(GOWD) && CGO_ENABLED=1 GOARCH=amd64 $(GO) test -tags='unit functional integration' -race ./... -count=1

test-docs: ## Run documentation tool unit tests.
	cd $(GOWD) && $(GO) test -tags=unit ./tools/doccheck -count=1

test-e2e: build something-json ## Run offline pipeline-to-viewer E2E tests with generated databases.
	cd $(GOWD) && $(GO) test -tags=e2e . -run '^TestE2E(Deterministic|Mocked)$$' -count=1
	cd frontend && E2E_SPEC=1 FIXTURE_DB="$(abspath build/e2e/deterministic/corpus.metadata.db)" node scripts/run-playwright.mjs --project="$(BROWSER)" $(if $(WORKERS),--workers=$(WORKERS)) tests/e2e.spec.cjs

test-e2e-live: build something-json ## Run opt-in E2E checks against real enrichment providers; requires E2E_LIVE=1.
	@test "$(E2E_LIVE)" = "1" || (printf '%s\n' 'Set E2E_LIVE=1 to permit real provider requests.' >&2; exit 2)
	cd $(GOWD) && E2E_LIVE=1 $(GO) test -tags=e2e . -run '^TestE2ELive$$' -count=1

coverage: coveragecheck ## Write atomic whole-module coverage and enforce COVERAGE_POLICY.
	@mkdir -p "$(dir $(COVERAGE_OUT))"
	cd $(GOWD) && $(GO) test -tags='unit functional integration' ./... -count=1 -covermode=atomic -coverpkg=./... -coverprofile="../$(COVERAGE_OUT)"
	./$(COVERAGE_CHECK) --profile "$(COVERAGE_OUT)" --policy "$(COVERAGE_POLICY)"

fixture: ## Regenerate the committed read-only viewer fixture database.
	cd $(GOWD) && FORCE_FIXTURE=1 $(GO) test -tags='unit functional integration' ./server -run TestGenerateFixture -count=1 -v

run: build ## Run the workspace pipeline. Override DB, CONFIG, WORKSPACE, and FRESH=1.
	./$(BIN) run --config "$(CONFIG)" --db "$(DB)" $(if $(WORKSPACE),--workspace "$(WORKSPACE)") $(if $(FRESH),--fresh)

serve: build ## Serve DB with embedded assets, or ASSETS_DIR for filesystem assets.
	./$(BIN) serve --db "$(DB)" --addr "$(ADDR)" $(if $(ASSETS_DIR),--assets-dir "$(ASSETS_DIR)")

database-backup: $(DB_METADATA) $(DB_PDF) ## This creates a copy of current database in case we want a backup
	cp $(DB_METADATA) ./../backup_databases/
	cp $(DB_PDF) ./../backup_databases/

dev: DB := $(FIXTURE_DB)
dev: ASSETS_DIR := src/server/frontend
dev: serve ## Serve the fixture with filesystem assets and no tracked-source mutation.

frontend-install: ## Install locked frontend dependencies with npm ci.
	cd frontend && npm ci

frontend-browsers: ## Install Playwright browsers. Override BROWSERS as needed.
	cd frontend && npm exec -- playwright install $(BROWSERS)

frontend-vendor: ## Rebuild the checked-in local D3-force browser bundle.
	cd frontend && npm run build:graph-vendor

test-frontend: build ## Run Chromium Playwright on an isolated fixture server. Override BROWSER, WORKERS, TEST_FILE.
	cd frontend && node scripts/run-playwright.mjs --project="$(BROWSER)" $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-all: build ## Run all Playwright browser projects on an isolated fixture server.
	cd frontend && node scripts/run-playwright.mjs $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-headed: build ## Run headed Chromium Playwright on an isolated fixture server.
	cd frontend && node scripts/run-playwright.mjs --project="$(BROWSER)" --headed $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-debug: build ## Run Chromium Playwright debug mode on an isolated fixture server.
	cd frontend && node scripts/run-playwright.mjs --project="$(BROWSER)" --debug $(TEST_FILE)

test-frontend-visual: build ## Run Chromium visual and accessibility browser checks on an isolated fixture server.
	cd frontend && node scripts/run-playwright.mjs --project=chromium tests/ui-quality.spec.cjs

test-frontend-unit: ## Run frontend JS unit tests with Node built-in test runner.
	cd frontend && npm run test:unit

frontend-report: ## Open a Playwright HTML report. Set REPORT_DIR to the report path printed by a test run.
	@test -n "$(REPORT_DIR)" || (printf '%s\n' 'REPORT_DIR is required; use the path printed by make test-frontend.' >&2; exit 2)
	cd frontend && npm exec -- playwright show-report "../$(REPORT_DIR)"

clean: ## Remove generated build artifacts, coverage, and isolated test reports.
	rm -rf build/
