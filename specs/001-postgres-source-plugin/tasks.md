# Tasks: CloudQuery PostgreSQL Source Plugin

**Input**: Design documents from `/specs/001-postgres-source-plugin/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: TDD is mandated by Constitution Principle II — all tasks include test-first workflow.

**Organization**: Tasks grouped by user story (P1 → P2 → P3) for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go module at repository root: `main.go`, `client/`, `resources/`, `e2e/`
- Tests colocated with implementation files per Go convention (`*_test.go`)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize Go module, project scaffold, and tooling

- [x] T001 Initialize Go module with `go mod init github.com/infobloxopen/cq-source-postgres` and add SDK v4, pgx/v5, pglogrepl, zerolog, arrow-go/v18 dependencies in go.mod
- [x] T002 Create project directory structure: `client/`, `resources/plugin/`, `resources/services/`, `e2e/`, `e2e/testdata/`, `docs/tables/`, `.github/workflows/`
- [x] T003 [P] Create Makefile with targets: `test`, `lint`, `build`, `gen-docs`, `docker-build` in Makefile
- [x] T004 [P] Configure golangci-lint with `.golangci.yml` at repository root
- [x] T005 [P] Create `e2e/docker-compose.yml` with PostgreSQL 16 service (user=cq, password=cq, db=cq_test, port 5432, wal_level=logical)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Write spec validation tests (empty connection string, invalid pgx_log_level, rows_per_record=0, negative rows_per_record, missing TABLE/UUID in destination_table_name) in client/spec_test.go — tests MUST FAIL initially
- [x] T007 Implement PluginSpec struct with `SetDefaults()` and `Validate()` methods in client/spec.go (FR-009, FR-016, FR-017)
- [x] T008 Write Client struct tests (New, Close, ID) in client/client_test.go — tests MUST FAIL initially
- [x] T009 Implement Client struct with `New()` constructor (pgxpool.Connect), `Close()`, and `ID()` in client/client.go
- [x] T010 Write type mapping tests (all 25+ PG→Arrow mappings from data-model.md) in client/pg_type_map_test.go — tests MUST FAIL initially
- [x] T011 [P] Implement PostgreSQL→Arrow type mapping function in client/pg_type_map.go (FR-006)
- [x] T012 Write Plugin constructor test in resources/plugin/plugin_test.go — test MUST FAIL initially
- [x] T013 Implement Plugin() constructor using `plugin.NewPlugin()` with name, version, team, kind, JSONSchema in resources/plugin/plugin.go
- [x] T014 Write PluginClient stub tests (Configure, Tables, Sync return not-implemented, Close) in resources/plugin/client_test.go — tests MUST FAIL initially
- [x] T015 Implement PluginClient struct embedding `plugin.UnimplementedDestination`, with stub `Configure()`, `Tables()`, `Sync()`, `Close()` in resources/plugin/client.go
- [x] T016 Create main.go entry point: `serve.Plugin(plugin.Plugin()).Serve(ctx)` in main.go

**Checkpoint**: Foundation ready — plugin compiles, spec validates, type mapping works, plugin registers with SDK

---

## Phase 3: User Story 1 — Basic Table Sync (Priority: P1) 🎯 MVP

**Goal**: Sync all (or selected) PostgreSQL tables to any CloudQuery destination as Apache Arrow records

**Independent Test**: Start PostgreSQL with seed data, configure `tables: ["*"]`, run sync, verify all tables and rows appear in output

### Tests for User Story 1

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [P] [US1] Write table discovery tests (discover all tables, filter by name, filter by glob, skip system schemas) in resources/services/tables_test.go
- [x] T018 [P] [US1] Write sync tests (read rows, batch into Arrow records per rows_per_record, handle empty tables, preserve NULLs) in resources/services/sync_test.go
- [x] T019 [P] [US1] Write E2E seed SQL with test schema: tables with all supported PG types (text, int, bool, timestamp, jsonb, uuid, array, numeric, bytea, inet, macaddr, interval, date) in e2e/testdata/seed.sql
- [x] T020 [P] [US1] Write E2E test config YAML for full sync (`tables: ["*"]`) in e2e/testdata/config.yml

### Implementation for User Story 1

- [x] T021 [US1] Implement table discovery: query `information_schema.tables`, `information_schema.columns`, `pg_index`/`pg_attribute` for PKs; build `schema.Tables` in resources/services/tables.go (FR-003, FR-004)
- [x] T022 [US1] Implement sync: iterate selected tables, read rows via `SELECT *`, convert to Arrow records using type map, emit via `res <- message.SyncInsert{}`, batch per `rows_per_record` in resources/services/sync.go (FR-005, FR-007, FR-018, FR-019, FR-020)
- [x] T023 [US1] Wire PluginClient.Configure() to create Client, PluginClient.Tables() to call table discovery, PluginClient.Sync() to call sync service in resources/plugin/client.go (FR-010)
- [x] T024 [US1] Write E2E test: full pipeline sync against real PostgreSQL, verify table count, row counts, type fidelity, NULL preservation, empty table schema emission in e2e/e2e_test.go (SC-001, SC-002, SC-007)
- [x] T025 [US1] Write E2E test: selective table sync (`tables: ["users", "orders"]`), verify only selected tables synced in e2e/e2e_test.go
- [x] T026 [US1] Write E2E test: glob pattern sync (`tables: ["user_*"]`), verify pattern matching in e2e/e2e_test.go

**Checkpoint**: Basic table sync works end-to-end. Plugin can discover tables, read rows, map types, and emit Arrow records.

---

## Phase 4: User Story 2 — Connection & Authentication (Priority: P1)

**Goal**: Support multiple connection string formats and handle connection errors gracefully

**Independent Test**: Attempt connections with valid URL, valid DSN, invalid password, unreachable host; verify success/failure behavior

### Tests for User Story 2

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T027 [P] [US2] Write connection format tests (URL format, DSN format, Unix socket path parsing) in client/client_test.go (FR-001, FR-002)
- [x] T028 [P] [US2] Write connection error tests (invalid password → auth error message, unreachable host → timeout error, invalid connection string → parse error) in client/client_test.go

### Implementation for User Story 2

- [x] T029 [US2] Enhance Client.New() to handle URL and DSN formats, add connection timeout, surface pgx authentication errors with actionable messages in client/client.go
- [x] T030 [US2] Implement pgx log level integration: create zerolog-to-pgx logger adapter, wire `pgx_log_level` from spec to pgxpool config in client/client.go (FR-008)
- [x] T031 [US2] Write E2E test: valid URL connection, valid DSN connection, invalid credentials error message, unreachable host timeout in e2e/e2e_test.go

**Checkpoint**: All connection string formats work. Errors are clear and actionable.

---

## Phase 5: User Story 3 — Plugin Configuration & Validation (Priority: P1)

**Goal**: Validate all configuration fields at startup with actionable error messages

**Independent Test**: Provide valid and invalid YAML configurations, verify accept/reject with appropriate messages

### Tests for User Story 3

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T032 [P] [US3] Write comprehensive spec validation tests: all edge cases from spec.md acceptance scenarios (valid config accepted, missing connection_string rejected, invalid pgx_log_level rejected with valid options listed, rows_per_record=0 rejected, default values applied) in client/spec_test.go
- [x] T033 [P] [US3] Write JSON Schema validation test: load contracts/plugin-spec.json, validate against sample configs in client/spec_test.go

### Implementation for User Story 3

- [x] T034 [US3] Enhance Spec.Validate() with detailed error messages naming the invalid field and explaining constraints in client/spec.go (SC-006)
- [x] T035 [US3] Embed JSON Schema from contracts/plugin-spec.json and pass to `plugin.WithJSONSchema()` in resources/plugin/plugin.go
- [x] T036 [US3] Write E2E test: plugin startup with valid config succeeds, startup with missing connection_string fails with clear error, startup with invalid pgx_log_level fails listing valid options in e2e/e2e_test.go

**Checkpoint**: All invalid configurations caught at startup with actionable error messages. JSON Schema validation active.

---

## Phase 6: User Story 4 — CDC via Logical Replication (Priority: P2)

**Goal**: Stream real-time changes (inserts, updates, deletes) from PostgreSQL via logical replication

**Independent Test**: Enable CDC with `cdc_id`, perform DML on source, verify changes appear in destination output in <5s

### Tests for User Story 4

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T037 [P] [US4] Write CDC unit tests: create replication slot, create publication, decode INSERT/UPDATE/DELETE WAL events, handle relation messages, confirm LSN advancement in resources/services/cdc_test.go
- [x] T038 [P] [US4] Write CDC state tests: slot creation, slot reuse on restart, LSN resume position in resources/services/cdc_test.go

### Implementation for User Story 4

- [x] T039 [US4] Implement replication slot management: create slot `cq_cdc_{cdc_id}`, create publication `cq_pub_{cdc_id}`, check slot existence, drop on cleanup in resources/services/cdc.go (FR-011)
- [x] T040 [US4] Implement WAL event streaming: connect via replication protocol, start replication from LSN, decode pgoutput messages (Begin, Relation, Insert, Update, Delete, Commit), convert to Arrow records in resources/services/cdc.go (FR-014)
- [x] T041 [US4] Implement CDC sync flow: initial full sync → switch to streaming mode, LSN tracking and confirmation, standby status heartbeat in resources/services/cdc.go (FR-012, FR-013)
- [x] T042 [US4] Wire CDC into PluginClient.Sync(): detect `cdc_id` in spec, route to CDC sync flow instead of batch sync in resources/plugin/client.go
- [x] T043 [US4] Validate CDC prerequisites: check `wal_level=logical`, return clear error if not set in resources/services/cdc.go
- [x] T044 [US4] Write E2E CDC test: enable CDC, perform initial sync, insert/update/delete rows, verify changes captured, restart and verify resume from LSN in e2e/cdc_e2e_test.go (SC-004, SC-005)
- [x] T045 [US4] Write E2E CDC test: two independent cdc_id sources on same database operate without interference in e2e/cdc_e2e_test.go
- [x] T046 [US4] Write E2E CDC test: missing wal_level=logical returns clear prerequisite error in e2e/cdc_e2e_test.go

**Checkpoint**: CDC fully functional. Real-time changes stream to destination. Restart resumes from last LSN.

---

## Phase 7: User Story 5 — Destination Table Name Templating (Priority: P3)

**Goal**: Support placeholder variables in destination table names for flexible naming conventions

**Independent Test**: Configure various `destination_table_name` patterns, run sync, verify output table names match expectations

### Tests for User Story 5

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T047 [P] [US5] Write template resolution tests: `{{TABLE}}` → actual name, `raw_{{TABLE}}` → prefixed, `{{TABLE}}_{{YEAR}}_{{MONTH}}` → with date parts, `{{UUID}}` → valid UUID, missing TABLE/UUID → error, dynamic placeholders with CDC → error in resources/services/template_test.go

### Implementation for User Story 5

- [x] T048 [US5] Implement template resolution function: parse placeholders, resolve `{{TABLE}}`, `{{UUID}}`, `{{YEAR}}`, `{{MONTH}}`, `{{DAY}}`, `{{HOUR}}`, `{{MINUTE}}` in resources/services/template.go (FR-015)
- [x] T049 [US5] Integrate template resolution into sync and CDC flows: apply to `SyncMigrateTable` message table name in resources/services/sync.go and resources/services/cdc.go
- [x] T050 [US5] Write E2E test: sync with `destination_table_name: "raw_{{TABLE}}"` produces correctly named output tables in e2e/e2e_test.go

**Checkpoint**: Destination table names resolve correctly for all placeholder combinations.

---

## Phase 8: User Story 6 — Rows Per Record Batching (Priority: P3)

**Goal**: Control how many rows are packed into each Apache Arrow record for tuning memory and throughput

**Independent Test**: Sync table with known row count using different `rows_per_record` values, verify correct number of records emitted

### Tests for User Story 6

> **TDD: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T051 [P] [US6] Write batching tests: 1200 rows with batch=500 → 3 records (500+500+200), 100 rows with batch=1000 → 1 record, batch=1 → N records in resources/services/sync_test.go

### Implementation for User Story 6

- [x] T052 [US6] Refine sync batching logic: ensure exact `rows_per_record` batching with correct final partial batch in resources/services/sync.go
- [x] T053 [US6] Write E2E test: sync with `rows_per_record: 100` and known table row counts, verify Arrow record counts match expectations in e2e/e2e_test.go

**Checkpoint**: Batching produces exact expected record counts for all configurations.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Docker, CI/CD, documentation, and consistency scanning

- [x] T054 [P] Create multi-stage Dockerfile: build stage (`golang:1.25-alpine`), runtime stage (`alpine:3.21`), EXPOSE 7777, ENTRYPOINT with `serve --address [::]:7777` in Dockerfile
- [x] T055 [P] Create GitHub Actions test workflow: Go test + lint on push/PR, PostgreSQL 16 service container (wal_level=logical), set CQ_SOURCE_PG_TEST_CONN env in .github/workflows/test.yml
- [x] T056 [P] Create GitHub Actions release workflow: trigger on tag push `v*`, build Docker image, push to ghcr.io/infobloxopen/cq-source-postgres, attach container image reference to GitHub Release in .github/workflows/release.yml
- [x] T057 [P] Write README.md with: plugin overview, installation, configuration reference (all spec fields with examples), table listing, CDC setup guide, Docker usage, development instructions in README.md (SC-008)
- [x] T058 [P] Create CHANGELOG.md with initial v0.1.0 entry in CHANGELOG.md
- [x] T059 Run full E2E test suite against real PostgreSQL to verify all user stories work together
- [x] T060 Run consistency scan: `go vet ./...`, `golangci-lint run`, `go mod tidy`, verify documentation matches implementation, check spec.md requirements coverage
- [x] T061 Run quickstart.md validation: follow quickstart steps on a clean environment to verify they work end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — **BLOCKS all user stories**
- **US1 Basic Table Sync (Phase 3)**: Depends on Foundational — **MVP milestone**
- **US2 Connection & Auth (Phase 4)**: Depends on Foundational — can run in parallel with US1
- **US3 Config Validation (Phase 5)**: Depends on Foundational — can run in parallel with US1/US2
- **US4 CDC (Phase 6)**: Depends on Foundational + US1 sync infrastructure
- **US5 Table Name Templating (Phase 7)**: Depends on US1 sync flow
- **US6 Rows Per Record (Phase 8)**: Depends on US1 sync flow
- **Polish (Phase 9)**: Depends on all desired user stories being complete

### User Story Dependencies

```
Phase 1 (Setup) ──► Phase 2 (Foundational) ──┬──► US1 (Phase 3) ──┬──► US4 (Phase 6)
                                              │                    ├──► US5 (Phase 7)
                                              │                    └──► US6 (Phase 8)
                                              ├──► US2 (Phase 4)
                                              └──► US3 (Phase 5)
                                                                        ──► Polish (Phase 9)
```

### Within Each User Story

1. Tests MUST be written and FAIL before implementation (Constitution Principle II)
2. Models/entities before services
3. Services before endpoint wiring
4. Unit tests before E2E tests
5. Story complete before moving to next priority

### Parallel Opportunities

- **Phase 1**: T003, T004, T005 can run in parallel
- **Phase 2**: T010/T011 (type map), T012/T013 (plugin constructor) can overlap after T006-T009
- **Phase 3-5**: US1, US2, US3 (all P1) can run in parallel after Foundational — but serial for single developer (US1 first as MVP)
- **Phase 6-8**: US4 depends on US1; US5 and US6 depend on US1; US5 and US6 can run in parallel
- **Phase 9**: T054, T055, T056, T057, T058 can all run in parallel

---

## Parallel Example: User Story 1

```bash
# Launch all tests for US1 together:
Task T017: "Write table discovery tests in resources/services/tables_test.go"
Task T018: "Write sync tests in resources/services/sync_test.go"
Task T019: "Write E2E seed SQL in e2e/testdata/seed.sql"
Task T020: "Write E2E test config in e2e/testdata/config.yml"

# Then implement sequentially:
Task T021: "Implement table discovery in resources/services/tables.go"
Task T022: "Implement sync in resources/services/sync.go"
Task T023: "Wire PluginClient methods in resources/plugin/client.go"

# Then E2E tests:
Task T024: "E2E full pipeline sync in e2e/e2e_test.go"
Task T025: "E2E selective table sync in e2e/e2e_test.go"
Task T026: "E2E glob pattern sync in e2e/e2e_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 — Basic Table Sync
4. **STOP and VALIDATE**: Run E2E tests, verify tables sync correctly
5. Deploy/demo if ready — this is a usable plugin

### Incremental Delivery

1. Setup + Foundational → Foundation compiles and validates specs
2. Add US1 (Basic Table Sync) → Test independently → **MVP!**
3. Add US2 (Connection & Auth) → Test connection edge cases → Hardened connectivity
4. Add US3 (Config Validation) → Test invalid configs → Production-ready validation
5. Add US4 (CDC) → Test real-time streaming → Differentiating feature
6. Add US5 (Table Name Templating) → Test naming patterns → Advanced flexibility
7. Add US6 (Rows Per Record) → Test batching → Performance tuning
8. Polish → Docker, CI/CD, docs, consistency scan → Release-ready

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (Basic Table Sync) — MVP path
   - Developer B: US2 (Connection & Auth)
   - Developer C: US3 (Config Validation)
3. After US1 completes:
   - Developer A: US4 (CDC)
   - Developer B: US5 (Table Name Templating)
   - Developer C: US6 (Rows Per Record)
4. Everyone contributes to Polish phase

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [USn] label maps task to specific user story for traceability
- Constitution Principle II: ALL tests written FIRST, verified FAILING, then implementation
- Constitution Principle III: E2E tests use real PostgreSQL, never mocked
- Constitution Principle VI: E2E tests always run in CI (no skip flags)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
