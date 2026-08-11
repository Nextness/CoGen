BIN              ?= build/analysis
SOMETHING_JSON   ?= build/something-json
PDF_STORE        ?= build/pdf-store
DOC_CHECK        ?= build/doccheck
COVERAGE_CHECK   ?= build/coveragecheck
PREPARE_OSF      ?= build/prepare-osf
GO               ?= go
GOWD             := src
DB               ?= corpus.metadata.db
ADDR             ?= 127.0.0.1:8080
ASSETS_DIR       ?= frontend/dist
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
OUT              ?=
DB_METADATA      ?= corpus.metadata.db
DB_PDF           ?= corpus.pdf.db

.DEFAULT_GOAL := help

.PHONY: help all build tools something-json pdf-store doccheck coveragecheck prepare-osf docs-catalog-update docs-state-update clean fmt format-check vet check check-frontend check-docs test test-go test-unit test-functional test-integration test-all test-race test-docs test-e2e test-e2e-live coverage fixture run migrate serve dev prepare-to-osf frontend-install frontend-browsers frontend-build frontend-vendor frontend-pdfjs-vendor frontend-pdfjs-vendor-check test-frontend test-frontend-all test-frontend-headed test-frontend-debug test-frontend-visual test-frontend-unit frontend-report database-backup

help: ## List supported local development commands, variables, and examples.
	@printf '%s\n' 'Research analysis local development interface'
	@printf '%s\n' ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  make %-26s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '%s\n' ''
	@printf '%s\n' 'Common variables:'
	@printf '%s\n' '  DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=frontend/dist'
	@printf '%s\n' '  CONFIG=config/workspace.something WORKSPACE=search_id@revision FRESH=1'
	@printf '%s\n' '  PACKAGE=./server TEST=^TestName$$ COVERAGE_OUT=build/coverage/coverage.out COVERAGE_POLICY=config/coverage_policy.something'
	@printf '%s\n' '  BROWSER=chromium WORKERS=4 TEST_FILE=tests/viewer.spec.cjs'
	@printf '%s\n' '  E2E_LIVE=1 explicitly enables real Crossref, OpenAlex, and ORCID requests'
	@printf '%s\n' '  OUT=build/osf-export is required by prepare-to-osf'
	@printf '%s\n' ''
	@printf '%s\n' 'Examples:'
	@printf '%s\n' '  make run DB=corpus.metadata.db CONFIG=config/workspace.something FRESH=1'
	@printf '%s\n' '  make serve DB=corpus.metadata.db ADDR=127.0.0.1:8090 ASSETS_DIR=frontend/dist'
	@printf '%s\n' '  make migrate DB=corpus.metadata.db'
	@printf '%s\n' '  make prepare-to-osf DB=corpus.metadata.db CONFIG=config/workspace.something OUT=build/osf-export'
	@printf '%s\n' '  make test-go PACKAGE=./server TEST=^TestGraph$$'
	@printf '%s\n' '  make test-frontend BROWSER=chromium WORKERS=4 TEST_FILE=tests/viewer.spec.cjs'
	@printf '%s\n' '  make test-e2e'
	@printf '%s\n' '  make test-e2e-live E2E_LIVE=1'

all: build ## Build build/analysis; the binary contains no frontend assets.

build: ## Build build/analysis; no Go default-output binaries are created.
	@mkdir -p "$(dir $(BIN))"
	cd $(GOWD) && $(GO) build -o "../$(BIN)" .

tools: something-json pdf-store doccheck coveragecheck prepare-osf ## Build every maintained Go tool under build/.

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

prepare-osf: ## Build build/prepare-osf, the copy-only sanitized export tool.
	@mkdir -p "$(dir $(PREPARE_OSF))"
	cd $(GOWD) && $(GO) build -o "../$(PREPARE_OSF)" ./tools/prepare-osf/

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

test-e2e: build frontend-build something-json pdf-store ## Run offline pipeline-to-viewer E2E tests with generated databases.
	cd $(GOWD) && $(GO) test -tags=e2e . -run '^TestE2E(Deterministic|Mocked)$$' -count=1
	cd frontend && E2E_SPEC=1 ASSETS_DIR="$(abspath $(ASSETS_DIR))" FIXTURE_DB="$(abspath build/e2e/deterministic/corpus.metadata.db)" PLAYWRIGHT_MUTATION_DB="$(abspath build/e2e/deterministic/review/corpus.metadata.db)" node scripts/run-playwright.mjs --project="$(BROWSER)" $(if $(WORKERS),--workers=$(WORKERS)) tests/e2e.spec.cjs
	cd $(GOWD) && $(GO) test -tags=e2e . -run '^TestE2EReviewEvidence$$' -count=1

test-e2e-live: build something-json ## Run opt-in E2E checks against real enrichment providers; requires E2E_LIVE=1.
	@test "$(E2E_LIVE)" = "1" || (printf '%s\n' 'Set E2E_LIVE=1 to permit real provider requests.' >&2; exit 2)
	cd $(GOWD) && E2E_LIVE=1 $(GO) test -tags=e2e . -run '^TestE2ELive$$' -count=1

coverage: coveragecheck ## Write atomic whole-module coverage and enforce COVERAGE_POLICY.
	@mkdir -p "$(dir $(COVERAGE_OUT))"
	cd $(GOWD) && $(GO) test -tags='unit functional integration' ./... -count=1 -covermode=atomic -coverpkg=./... -coverprofile="../$(COVERAGE_OUT)"
	./$(COVERAGE_CHECK) --profile "$(COVERAGE_OUT)" --policy "$(COVERAGE_POLICY)"

fixture: ## Regenerate ignored viewer fixture databases from authoritative fixture code.
	cd $(GOWD) && FORCE_FIXTURE=1 $(GO) test -tags='unit functional integration' ./server -run TestGenerateFixture -count=1 -v

run: build ## Run the workspace pipeline. Override DB, CONFIG, WORKSPACE, and FRESH=1.
	./$(BIN) run --config "$(CONFIG)" --db "$(DB)" $(if $(WORKSPACE),--workspace "$(WORKSPACE)") $(if $(FRESH),--fresh)

migrate: build ## Apply pending metadata migrations to an existing DB without running a workspace.
	./$(BIN) migrate --db "$(DB)"

serve: build frontend-build ## Serve DB with frontend assets from ASSETS_DIR (default frontend/dist).
	./$(BIN) serve --db "$(DB)" --addr "$(ADDR)" --assets-dir "$(ASSETS_DIR)"

database-backup: $(DB_METADATA) $(DB_PDF) ## This creates a copy of current database in case we want a backup
	cp $(DB_METADATA) ./../backup_databases/
	cp $(DB_PDF) ./../backup_databases/

dev: build frontend-build ## Serve a disposable fixture pair with assembled assets for local review development.
	cd frontend && node scripts/run-dev.mjs "$(BIN)" "$(FIXTURE_DB)" "$(ASSETS_DIR)" "$(ADDR)"

prepare-to-osf: prepare-osf ## Create a sanitized corpus copy. DB and OUT are required; CONFIG is optional.
	@test -n "$(DB)" || (printf '%s\n' 'DB is required.' >&2; exit 2)
	@test -n "$(OUT)" || (printf '%s\n' 'OUT is required.' >&2; exit 2)
	./$(PREPARE_OSF) --db "$(DB)" --out "$(OUT)" $(if $(filter command line,$(origin CONFIG)),--config "$(CONFIG)")

frontend-install: ## Install locked frontend dependencies with npm ci.
	cd frontend && npm ci

frontend-build: ## Assemble frontend/dist from frontend sources with npm run build.
	cd frontend && npm run build

check-frontend: ## Type-check frontend TypeScript sources with tsc --noEmit.
	cd frontend && npm run typecheck

frontend-browsers: ## Install Playwright browsers. Override BROWSERS as needed.
	cd frontend && npm exec -- playwright install $(BROWSERS)

frontend-vendor: ## Rebuild the checked-in local D3-force browser bundle.
	cd frontend && npm run build:graph-vendor

frontend-pdfjs-vendor: ## Rebuild checked-in PDF.js 4.2.67 core, worker, CMaps, fonts, and license assets.
	cd frontend && npm run build:pdfjs-vendor

frontend-pdfjs-vendor-check: ## Verify checked-in PDF.js assets exactly match the pinned installed package.
	cmp frontend/node_modules/pdfjs-dist/build/pdf.min.mjs frontend/vendor/pdfjs/pdf.min.mjs
	cmp frontend/node_modules/pdfjs-dist/build/pdf.worker.min.mjs frontend/vendor/pdfjs/pdf.worker.min.mjs
	cmp frontend/node_modules/pdfjs-dist/LICENSE frontend/vendor/pdfjs/LICENSE
	diff -qr frontend/node_modules/pdfjs-dist/cmaps frontend/vendor/pdfjs/cmaps
	diff -qr frontend/node_modules/pdfjs-dist/standard_fonts frontend/vendor/pdfjs/standard_fonts

test-frontend: build frontend-build ## Run Chromium Playwright on an isolated fixture server. Override BROWSER, WORKERS, TEST_FILE.
	cd frontend && ASSETS_DIR="$(abspath $(ASSETS_DIR))" node scripts/run-playwright.mjs --project="$(BROWSER)" $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-all: build frontend-build ## Run all Playwright browser projects on an isolated fixture server.
	cd frontend && ASSETS_DIR="$(abspath $(ASSETS_DIR))" node scripts/run-playwright.mjs $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-headed: build frontend-build ## Run headed Chromium Playwright on an isolated fixture server.
	cd frontend && ASSETS_DIR="$(abspath $(ASSETS_DIR))" node scripts/run-playwright.mjs --project="$(BROWSER)" --headed $(if $(WORKERS),--workers=$(WORKERS)) $(TEST_FILE)

test-frontend-debug: build frontend-build ## Run Chromium Playwright debug mode on an isolated fixture server.
	cd frontend && ASSETS_DIR="$(abspath $(ASSETS_DIR))" node scripts/run-playwright.mjs --project="$(BROWSER)" --debug $(TEST_FILE)

test-frontend-visual: build frontend-build ## Run Chromium visual and accessibility browser checks on an isolated fixture server.
	cd frontend && ASSETS_DIR="$(abspath $(ASSETS_DIR))" node scripts/run-playwright.mjs --project=chromium tests/ui-quality.spec.cjs

test-frontend-unit: ## Run frontend TS unit tests with Node built-in test runner.
	cd frontend && npm run test:unit

frontend-report: ## Open a Playwright HTML report. Set REPORT_DIR to the report path printed by a test run.
	@test -n "$(REPORT_DIR)" || (printf '%s\n' 'REPORT_DIR is required; use the path printed by make test-frontend.' >&2; exit 2)
	cd frontend && npm exec -- playwright show-report "../$(REPORT_DIR)"

clean: ## Remove generated build artifacts, coverage, isolated test reports, and assembled frontend output.
	rm -rf build/ frontend/dist
