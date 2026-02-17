# Implementation Plan: CloudQuery PostgreSQL Source Plugin

**Branch**: `001-postgres-source-plugin` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-postgres-source-plugin/spec.md`

## Summary

Build a CloudQuery source plugin in Go that syncs PostgreSQL tables to any CloudQuery destination. The plugin implements the CloudQuery Plugin SDK v4 `plugin.Client` interface, connects to PostgreSQL via `pgx/v5`, discovers tables dynamically from the database catalog, reads rows as Apache Arrow records, and supports CDC via logical replication using `pglogrepl`. The plugin is packaged as a Docker container published to GitHub Container Registry (ghcr.io) and starts a gRPC server on the default port (7777).

## Technical Context

**Language/Version**: Go 1.25 (latest stable per Constitution Principle I)
**Primary Dependencies**: CloudQuery Plugin SDK v4, Apache Arrow Go v18, pgx/v5, pglogrepl, zerolog
**Storage**: PostgreSQL 12–17 (source database being synced)
**Testing**: Go `testing` package; `go test ./...`; E2E tests against real PostgreSQL via Docker
**Target Platform**: Linux (container), macOS/Linux (development)
**Project Type**: Single Go module
**Container**: Docker multi-stage build → ghcr.io; gRPC server on port 7777
**CI/CD**: GitHub Actions (test + lint on push/PR, build + push container on tag)
**Performance Goals**: 100k rows across 10 tables in <60s; CDC delivery <5s latency
**Constraints**: No CGO; single binary; <200MB container image
**Scale/Scope**: Hundreds of tables, millions of rows per sync; real-time CDC streaming

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Status | Evidence |
|---|-----------|--------|----------|
| I | Latest Go Version | ✅ PASS | `go.mod` declares `go 1.25`; CI uses `go-version-file: go.mod` |
| II | Test-Driven Development | ✅ PASS | All tasks specify tests-first; Red-Green-Refactor enforced in workflow |
| III | End-to-End PostgreSQL Testing | ✅ PASS | E2E tests use Docker PostgreSQL; cover sync, CDC, error paths |
| IV | Documentation Synchronization | ✅ PASS | README, GoDoc, and spec docs updated in same PR as features |
| V | Codebase Consistency Scanning | ✅ PASS | Final task includes consistency scan; `go mod tidy` + `golangci-lint` |
| VI | E2E Tests Always Run | ✅ PASS | CI provisions PostgreSQL service container; no skip flags; no optional stages |

**Gate result**: ALL PASS — proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/001-postgres-source-plugin/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── plugin-spec.json # JSON Schema for plugin spec validation
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
.
├── main.go                          # Entry point: serve.Plugin(plugin.Plugin()).Serve()
├── go.mod
├── go.sum
├── Makefile                         # test, lint, build, gen-docs targets
├── Dockerfile                       # Multi-stage build, EXPOSE 7777
├── README.md                        # User-facing documentation
├── CHANGELOG.md                     # Release notes
│
├── client/
│   ├── client.go                    # Client struct (pgxpool, logger, spec)
│   ├── client_test.go               # Client unit tests
│   ├── spec.go                      # Spec struct with Validate() + SetDefaults()
│   ├── spec_test.go                 # Spec validation tests
│   └── pg_type_map.go               # PostgreSQL → Arrow type mapping
│
├── resources/
│   ├── plugin/
│   │   ├── plugin.go                # Plugin() constructor (name, version, team, kind)
│   │   ├── client.go                # Configure(), Tables(), Sync(), Close()
│   │   └── client_test.go           # Integration tests for plugin lifecycle
│   └── services/
│       ├── tables.go                # Dynamic table discovery from pg_catalog
│       ├── tables_test.go           # Table discovery tests
│       ├── sync.go                  # Row reading + Arrow record emission
│       ├── sync_test.go             # Sync tests
│       ├── cdc.go                   # CDC via pglogrepl (logical replication)
│       ├── cdc_test.go              # CDC tests
│       ├── template.go             # destination_table_name placeholder resolution
│       └── template_test.go        # Template tests
│
├── e2e/
│   ├── e2e_test.go                  # End-to-end tests (full sync pipeline)
│   ├── cdc_e2e_test.go              # End-to-end CDC tests
│   ├── testdata/
│   │   ├── seed.sql                 # Schema + data for E2E tests
│   │   └── config.yml               # CloudQuery config for E2E
│   └── docker-compose.yml           # PostgreSQL service for local E2E
│
├── .github/
│   └── workflows/
│       ├── test.yml                 # CI: test + lint on push/PR (with PostgreSQL service)
│       └── release.yml              # CD: build + push Docker image to ghcr.io on tag
│
└── docs/
    └── tables/                      # Auto-generated table documentation
```

**Structure Decision**: Single Go module following the CloudQuery scaffold pattern
(`client/`, `resources/plugin/`, `resources/services/`). E2E tests are in a
dedicated `e2e/` directory to clearly separate them from unit tests. CI
workflows provision PostgreSQL via GitHub Actions service containers.

## Complexity Tracking

> No Constitution Check violations — this section is intentionally empty.
