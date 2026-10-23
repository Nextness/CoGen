BIN              ?= build/analysis
SOMETHING_PRINTER ?= build/something-printer
PDF_STORE        ?= build/pdf-store
DOC_CHECK        ?= build/doccheck
COVERAGE_CHECK   ?= build/coveragecheck
RAWSQL_CHECK     ?= build/rawsqlcheck
IMPORT_CHECK     ?= build/importcheck
PREPARE_OSF      ?= build/prepare-osf
GITLEAKS        ?= build/gitleaks
GITLEAKS_VERSION ?= v8.30.1
SQLC             ?= build/sqlc
SQLC_VERSION     ?= v1.31.1
GO               ?= go
GOWD             := .
DB               ?= corpus.metadata.db
ADDR             ?= 127.0.0.1:8080
ASSETS_DIR       ?= build/frontend/dist
CONFIG           ?= config/workspace.something
WORKSPACE        ?=
FRESH            ?=
PACKAGE          ?=
DOMAIN           ?= all
LAYER            ?= all
COMPONENT        ?=
TEST_SELECTOR    ?= build/testselect
TEST             ?=
COVERAGE_OUT     ?= build/coverage/coverage.out
COVERAGE_POLICY  ?= config/coverage_policy.something
FIXTURE_DB       ?= build/fixtures/viewer/workspace.fixture.db
BROWSER          ?= chromium
WORKERS          ?= $(if $(CI),1,8)
TEST_FILE        ?=
BROWSERS         ?= chromium firefox
REPORT_DIR       ?=
OUT              ?=
DB_METADATA      ?= corpus.metadata.db
DB_PDF           ?= corpus.pdf.db

# Quote selector values as literal shell arguments, including regexes with quotes or dollar signs.
shellquote = '$(subst ','"'"',$(1))'
selector_args = --domain $(call shellquote,$(DOMAIN)) --layer $(call shellquote,$(LAYER)) --component $(call shellquote,$(COMPONENT)) --test $(call shellquote,$(TEST)) --test-file $(call shellquote,$(TEST_FILE)) --browser $(call shellquote,$(BROWSER)) --workers $(call shellquote,$(WORKERS))
export FIXTURE_DB ASSETS_DIR

.DEFAULT_GOAL := help

.PHONY: recover help all build tools something-printer pdf-store doccheck coveragecheck rawsqlcheck importcheck prepare-osf gitleaks gitleaks-install sqlc-install sqlc-generate sqlc-check docs-catalog-update docs-state-update clean fmt format-check vet check check-frontend check-sql check-imports check-docs test test-go test-unit test-functional test-integration test-all test-race test-docs test-e2e test-e2e-live coverage fixture run migrate serve dev prepare-to-osf frontend-install frontend-browsers frontend-build frontend-classes frontend-classes-check frontend-vendor frontend-pdfjs-vendor frontend-pdfjs-vendor-check test-frontend test-frontend-all test-frontend-headed test-frontend-debug test-frontend-visual test-frontend-unit test-frontend-functional testselect frontend-report database-backup sql-report import-report

help: ## List supported local development commands, variables, and examples.
	@printf '%s\n' 'Research analysis local development interface'
	@printf '%s\n' ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  make %-26s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '%s\n' ''
	@printf '%s\n' 'Common variables:'
	@printf '%s\n' '  DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=build/frontend/dist'
	@printf '%s\n' '  CONFIG=config/workspace.something WORKSPACE=search_id@revision FRESH=1'
	@printf '%s\n' '  DOMAIN=all LAYER=all COMPONENT=database/run (COMPONENT requires an explicit DOMAIN for make test)'
	@printf '%s\n' '  PACKAGE=./tests/backend/server/integration TEST=^TestName$$ COVERAGE_OUT=build/coverage/coverage.out COVERAGE_POLICY=config/coverage_policy.something'
	@printf '%s\n' '  BROWSER=chromium WORKERS=4 TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts'
	@printf '%s\n' '  E2E_LIVE=1 explicitly enables real Crossref, OpenAlex, and ORCID requests'
	@printf '%s\n' '  OUT=build/osf-export is required by prepare-to-osf'
	@printf '%s\n' ''
	@printf '%s\n' 'Examples:'
	@printf '%s\n' '  make run DB=corpus.metadata.db CONFIG=config/workspace.something FRESH=1'
	@printf '%s\n' '  make serve DB=corpus.metadata.db ADDR=127.0.0.1:8090 ASSETS_DIR=build/frontend/dist'
	@printf '%s\n' '  make migrate DB=corpus.metadata.db'
	@printf '%s\n' '  make prepare-to-osf DB=corpus.metadata.db CONFIG=config/workspace.something OUT=build/osf-export'
	@printf '%s\n' '  make test-go PACKAGE=./tests/backend/server/integration TEST=^TestAPIDetailsGraphModes$$'
	@printf '%s\n' '  make test-frontend BROWSER=chromium WORKERS=4 TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts'
	@printf '%s\n' '  make test-e2e'
	@printf '%s\n' '  make test-e2e-live E2E_LIVE=1'
	@printf '%s\n' '  make gitleaks'
	@printf '%s\n' ''
	@printf '%s\n' 'Family SQL generation:'
	@printf '%s\n' '  Maintain backend/database/<family>/schema/ and backend/pdfstore/schema/; committed internal/sql output is generated.'
	@printf '%s\n' '  make sqlc-install    install the pinned generator to build/sqlc; normal builds do not need it'
	@printf '%s\n' '  make sqlc-generate   build/sqlc -f config/sqlc.yaml generate'
	@printf '%s\n' '  make sqlc-check      build/sqlc -f config/sqlc.yaml diff and vet'
	@printf '%s\n' '  make build           never invokes sqlc; consumes committed internal/sql output'

all: build ## Build build/analysis; the binary contains no frontend assets.

build: ## Build build/analysis; no Go default-output binaries are created.
	@mkdir -p "$(dir $(BIN))"
	cd $(GOWD) && $(GO) build -o "$(BIN)" ./backend/cmd/analysis

tools: something-printer pdf-store doccheck coveragecheck rawsqlcheck importcheck prepare-osf testselect ## Build every maintained Go tool under build/.

something-printer: ## Build build/something-printer, a tool to inspect evaluated .something configs.
	@mkdir -p "$(dir $(SOMETHING_PRINTER))"
	cd $(GOWD) && $(GO) build -o "$(SOMETHING_PRINTER)" ./tooling/go/cmd/something-printer/

pdf-store: ## Build build/pdf-store, the validated manual PDF maintenance tool.
	@mkdir -p "$(dir $(PDF_STORE))"
	cd $(GOWD) && $(GO) build -o "$(PDF_STORE)" ./tooling/go/cmd/pdf-store/

doccheck: ## Build build/doccheck, the documentation consistency tool.
	@mkdir -p "$(dir $(DOC_CHECK))"
	cd $(GOWD) && $(GO) build -o "$(DOC_CHECK)" ./tooling/go/cmd/doccheck/

coveragecheck: ## Build build/coveragecheck, the coverage policy tool.
	@mkdir -p "$(dir $(COVERAGE_CHECK))"
	cd $(GOWD) && $(GO) build -o "$(COVERAGE_CHECK)" ./tooling/go/cmd/coveragecheck/

rawsqlcheck: ## Build build/rawsqlcheck, the raw SQL ownership tool.
	@mkdir -p "$(dir $(RAWSQL_CHECK))"
	cd $(GOWD) && $(GO) build -o "$(RAWSQL_CHECK)" ./tooling/go/cmd/rawsqlcheck/

importcheck: ## Build build/importcheck, the import and transaction graph tool.
	@mkdir -p "$(dir $(IMPORT_CHECK))"
	cd $(GOWD) && $(GO) build -o "$(IMPORT_CHECK)" ./tooling/go/cmd/importcheck/

prepare-osf: ## Build build/prepare-osf, the copy-only sanitized export tool.
	@mkdir -p "$(dir $(PREPARE_OSF))"
	cd $(GOWD) && $(GO) build -o "$(PREPARE_OSF)" ./tooling/go/cmd/prepare-osf/

docs-catalog-update: doccheck ## Regenerate docs/PROJECT_CATALOG.md from maintained source declarations.
	./$(DOC_CHECK) catalog update

docs-state-update: doccheck ## Acknowledge reviewed docs by updating docs/DOC-STATE.md hashes.
	./$(DOC_CHECK) state update

fmt: ## Format maintained Go source and tests.
	@rg --files backend internal tooling/go tests -g '*.go' -0 | xargs -0 gofmt -w

format-check: ## Fail if any Go source file needs gofmt.
	@unformatted="$$(rg --files backend internal tooling/go tests -g '*.go' -0 | xargs -0 gofmt -l)"; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n%s\n' 'Run make fmt; unformatted Go files:' "$$unformatted"; \
		exit 1; \
	fi

vet: ## Run go vet across the module.
	cd $(GOWD) && $(GO) vet ./...

check: format-check vet check-sql check-imports check-docs gitleaks ## Run local Go, SQL ownership, import graph, documentation, and secret checks.

check-sql: rawsqlcheck ## Verify raw SQL calls only appear in approved locations.
	./$(RAWSQL_CHECK) check

sql-report: rawsqlcheck ## Print the raw SQL ownership report.
	./$(RAWSQL_CHECK) report

check-imports: importcheck ## Verify the package import and transaction graph follows the approved direction.
	./$(IMPORT_CHECK) check

import-report: importcheck ## Print the package import and transaction graph report.
	./$(IMPORT_CHECK) report

check-docs: doccheck ## Validate documentation links, format, catalog, migrations, and acknowledged state.
	./$(DOC_CHECK) check

gitleaks-install: ## Install gitleaks to build/gitleaks for local secret scanning.
	@test -x "$(GITLEAKS)" || { mkdir -p "$(dir $(GITLEAKS))"; cd $(GOWD) && GOBIN="$(abspath $(dir $(GITLEAKS)))" $(GO) install github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION); }

gitleaks: gitleaks-install ## Run the gitleaks secret scan over the repository.
	"$(GITLEAKS)" detect --source . --no-banner --redact

sqlc-install: ## Install the pinned generator to build/sqlc; normal builds do not need it.
	@test -x "$(SQLC)" || { mkdir -p "$(dir $(SQLC))"; cd $(GOWD) && GOBIN="$(abspath $(dir $(SQLC)))" $(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION); }

sqlc-generate: sqlc-install ## Regenerate committed internal/sql output with build/sqlc -f config/sqlc.yaml generate.
	"$(SQLC)" -f config/sqlc.yaml generate

sqlc-check: sqlc-install ## Diff committed output and vet queries with build/sqlc -f config/sqlc.yaml diff and vet.
	"$(SQLC)" -f config/sqlc.yaml diff
	"$(SQLC)" -f config/sqlc.yaml vet

testselect: ## Build the suite selector without building application or frontend code.
	@mkdir -p "$(dir $(TEST_SELECTOR))"
	$(GO) build -o "$(TEST_SELECTOR)" ./tooling/go/cmd/testselect

test: testselect ## Run all ordinary offline domains. Narrow with DOMAIN, LAYER, and COMPONENT.
	./$(TEST_SELECTOR) $(selector_args) --runner all

test-unit: ## Run ordinary unit suites, optionally narrowed by DOMAIN and COMPONENT.
	$(MAKE) test LAYER=unit

test-functional: ## Run ordinary functional suites, optionally narrowed by DOMAIN and COMPONENT.
	$(MAKE) test LAYER=functional

test-integration: ## Run ordinary integration suites, optionally narrowed by DOMAIN and COMPONENT.
	$(MAKE) test LAYER=integration

test-all: test ## Run every ordinary offline domain; full-system E2E remains explicit.

test-go: testselect ## Run ordinary Go suites. PACKAGE accepts a canonical tests/ package; TEST must match a test.
	./$(TEST_SELECTOR) $(selector_args) --runner go --package $(call shellquote,$(PACKAGE))

test-race: testselect ## Run ordinary Go suites with race detection; selectors narrow the scope.
	CGO_ENABLED=1 ./$(TEST_SELECTOR) $(selector_args) --runner go --race

test-docs: ## Run the documentation command functional regression suites.
	$(MAKE) test-go DOMAIN=tooling COMPONENT=go/doccheck

test-e2e: build frontend-build something-printer pdf-store ## Run ordered offline pipeline, browser continuity, and persisted review evidence checks.
	$(GO) test -tags=e2e ./tests/e2e/offline -run '^TestE2E(Deterministic|Mocked)$$' -count=1
	E2E_SPEC=1 ASSETS_DIR="$(abspath $(ASSETS_DIR))" FIXTURE_DB="$(abspath build/e2e/deterministic/corpus.metadata.db)" PLAYWRIGHT_MUTATION_DB="$(abspath build/e2e/deterministic/review/corpus.metadata.db)" node tests/support/node/run-playwright.ts --project="$(BROWSER)" $(if $(WORKERS),--workers=$(WORKERS)) tests/e2e/offline/e2e.spec.ts
	$(GO) test -tags=e2e ./tests/e2e/offline -run '^TestE2EReviewEvidence$$' -count=1

test-e2e-live: build something-printer pdf-store ## Run explicitly gated real-provider E2E checks; requires E2E_LIVE=1.
	@test "$(E2E_LIVE)" = "1" || (printf '%s\n' 'Set E2E_LIVE=1 to permit real provider requests.' >&2; exit 2)
	E2E_LIVE=1 $(GO) test -tags='e2e e2e_live' ./tests/e2e/live -run '^TestE2ELive$$' -count=1

coverage: coveragecheck ## Instrument all production packages, retain uncovered code, and enforce COVERAGE_POLICY.
	@mkdir -p "$(dir $(COVERAGE_OUT))"
	@production_packages="$$($(GO) list ./backend/... ./internal/... ./tooling/go/... | paste -sd, -)"; \
	$(GO) test ./backend/... ./internal/... ./tooling/go/... ./tests/... -count=1 -covermode=atomic -coverpkg="$$production_packages" -coverprofile="$(COVERAGE_OUT)"
	./$(COVERAGE_CHECK) --profile "$(COVERAGE_OUT)" --policy "$(COVERAGE_POLICY)"

fixture: ## Generate the disposable viewer fixture pair from the normal test-support command.
	$(GO) run ./tests/support/go/cmd/viewer-fixture --out "$(FIXTURE_DB)" --registry config/database.something

run: build ## Run the workspace pipeline. Override DB, CONFIG, WORKSPACE, and FRESH=1.
	./$(BIN) run --config "$(CONFIG)" --db "$(DB)" $(if $(WORKSPACE),--workspace "$(WORKSPACE)") $(if $(FRESH),--fresh)

recover: build ## Mark one abandoned running attempt failed. Requires DB and RUN_ID.
	./$(BIN) recover --db "$(DB)" --run-id "$(RUN_ID)"

migrate: build ## Apply pending metadata migrations to an existing DB without running a workspace.
	./$(BIN) migrate --db "$(DB)"

serve: build frontend-build ## Serve DB with frontend assets from ASSETS_DIR (default build/frontend/dist).
	./$(BIN) serve --db "$(DB)" --addr "$(ADDR)" --assets-dir "$(ASSETS_DIR)"

database-backup: $(DB_METADATA) $(DB_PDF) ## This creates a copy of current database in case we want a backup
	cp $(DB_METADATA) ./../backup_databases/
	cp $(DB_PDF) ./../backup_databases/

dev: build frontend-build fixture ## Serve a disposable fixture pair with assembled assets for local review development.
	node tooling/node/frontend/run-dev.ts "$(BIN)" "$(FIXTURE_DB)" "$(ASSETS_DIR)" "$(ADDR)"

prepare-to-osf: prepare-osf ## Create a sanitized corpus copy. DB and OUT are required; CONFIG is optional.
	@test -n "$(DB)" || (printf '%s\n' 'DB is required.' >&2; exit 2)
	@test -n "$(OUT)" || (printf '%s\n' 'OUT is required.' >&2; exit 2)
	./$(PREPARE_OSF) --db "$(DB)" --out "$(OUT)" $(if $(filter command line,$(origin CONFIG)),--config "$(CONFIG)")

frontend-install: ## Install locked frontend dependencies with npm ci.
	npm ci

frontend-build: ## Assemble build/frontend/dist from frontend sources with npm run build.
	npm run build --workspace @cogen/tooling

check-frontend: frontend-classes-check ## Verify CSS classes and type-check frontend TypeScript sources with tsc --noEmit.
	npm run typecheck --workspaces --if-present

frontend-classes: ## Regenerate the committed TypeScript registry from authoritative stylesheets.
	npm run generate:classes --workspace @cogen/tooling

frontend-classes-check: ## Verify class registry freshness and statically visible class-token uses.
	npm run check:classes --workspace @cogen/tooling

frontend-browsers: ## Install Playwright browsers. Override BROWSERS as needed.
	npm exec --workspace tests -- playwright install $(BROWSERS)

frontend-vendor: ## Rebuild the checked-in local D3-force browser bundle.
	npm run build:graph-vendor --workspace @cogen/tooling

frontend-pdfjs-vendor: ## Rebuild checked-in PDF.js 4.2.67 core, worker, CMaps, fonts, and license assets.
	npm run build:pdfjs-vendor --workspace @cogen/tooling

frontend-pdfjs-vendor-check: ## Verify checked-in PDF.js assets exactly match the pinned installed package.
	cmp node_modules/pdfjs-dist/build/pdf.min.mjs frontend/vendor/pdfjs/pdf.min.mjs
	cmp node_modules/pdfjs-dist/build/pdf.worker.min.mjs frontend/vendor/pdfjs/pdf.worker.min.mjs
	cmp node_modules/pdfjs-dist/LICENSE frontend/vendor/pdfjs/LICENSE
	diff -qr node_modules/pdfjs-dist/cmaps frontend/vendor/pdfjs/cmaps
	diff -qr node_modules/pdfjs-dist/standard_fonts frontend/vendor/pdfjs/standard_fonts

test-frontend: testselect ## Run viewer browser integration suites in one project; TEST_FILE must belong to the selected component.
	./$(TEST_SELECTOR) --domain frontend --layer integration --component $(call shellquote,$(if $(COMPONENT),$(COMPONENT),viewer)) --runner browser --test-file $(call shellquote,$(TEST_FILE)) --browser $(call shellquote,$(BROWSER)) --workers $(call shellquote,$(WORKERS))

test-frontend-all: ## Run viewer browser integration in every configured browser project.
	$(MAKE) test-frontend BROWSER=all

test-frontend-headed: testselect ## Run focused viewer integration in a headed browser.
	./$(TEST_SELECTOR) --domain frontend --layer integration --component viewer --runner browser --test-file $(call shellquote,$(TEST_FILE)) --browser $(call shellquote,$(BROWSER)) --workers $(call shellquote,$(WORKERS)) --headed

test-frontend-debug: testselect ## Run focused viewer integration with the Playwright debugger.
	./$(TEST_SELECTOR) --domain frontend --layer integration --component viewer --runner browser --test-file $(call shellquote,$(TEST_FILE)) --browser $(call shellquote,$(BROWSER)) --debug

test-frontend-visual: ## Run Chromium visual and accessibility checks against an isolated fixture pair.
	$(MAKE) test DOMAIN=frontend COMPONENT=quality LAYER=integration BROWSER=chromium

test-frontend-unit: ## Run frontend product Node unit suites without starting a viewer or browser.
	$(MAKE) test DOMAIN=frontend LAYER=unit

test-frontend-functional: ## Run frontend product Node functional suites without starting a viewer or browser.
	$(MAKE) test DOMAIN=frontend LAYER=functional

frontend-report: ## Open a Playwright HTML report. Set REPORT_DIR to the report path printed by a test run.
	@test -n "$(REPORT_DIR)" || (printf '%s\n' 'REPORT_DIR is required; use the path printed by make test-frontend.' >&2; exit 2)
	npm exec --workspace tests -- playwright show-report "$(abspath $(REPORT_DIR))"

clean: ## Remove generated build artifacts, coverage, isolated test reports, and assembled frontend output.
	rm -rf build/
