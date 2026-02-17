<!--
Sync Impact Report
===================
Version change: N/A → 1.0.0 (initial adoption)
Modified principles: N/A (initial)
Added sections:
  - Core Principles (6 principles)
  - Technology Stack & Constraints
  - Development Workflow
  - Governance
Removed sections: N/A
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ reviewed (no changes needed)
  - .specify/templates/spec-template.md ✅ reviewed (no changes needed)
  - .specify/templates/tasks-template.md ✅ reviewed (no changes needed)
  - .specify/templates/checklist-template.md ✅ reviewed (no changes needed)
Follow-up TODOs: None
-->

# cq-source-postgres Constitution

## Core Principles

### I. Latest Go Version

All compilation and CI pipelines MUST target the latest stable release
of Go. The `go.mod` file MUST declare the latest stable Go version
(e.g., `go 1.24`) and MUST be updated within one week of a new stable
Go release. Developers MUST NOT pin to older Go versions without an
explicit, documented justification approved via the amendment process.

**Rationale**: Staying current ensures access to performance
improvements, security patches, and language features while preventing
technical debt from version drift.

### II. Test-Driven Development (NON-NEGOTIABLE)

TDD is mandatory for all feature and bug-fix work. The workflow MUST
follow the Red-Green-Refactor cycle:

1. Write a failing test that captures the expected behavior.
2. Confirm the test fails (Red).
3. Implement the minimum code to make the test pass (Green).
4. Refactor while keeping all tests green (Refactor).

No production code MUST be written without a corresponding test
written first. Pull requests that introduce untested production code
MUST be rejected during review.

**Rationale**: TDD produces higher-quality, regression-resistant code
and ensures every behavior is verified from inception.

### III. End-to-End PostgreSQL Testing

Every feature MUST include end-to-end tests that exercise the full
plugin pipeline against a real PostgreSQL instance. E2E tests MUST:

- Connect to a live PostgreSQL database (via Docker, CI service
  container, or equivalent).
- Execute actual SQL queries against real schemas and data.
- Validate that synced records match expected output.
- Cover both happy-path and error scenarios (connection failures,
  permission errors, unsupported types).

Mocks or stubs MUST NOT substitute for PostgreSQL in E2E tests;
unit tests MAY use mocks for isolated logic.

**Rationale**: A CloudQuery source plugin's primary contract is
faithful data extraction from PostgreSQL. Only real-database tests
can validate this contract.

### IV. Documentation Synchronization

Documentation MUST be updated in the same pull request as the feature
or change it describes. This applies to:

- README.md (user-facing usage, configuration, examples).
- Inline code comments and GoDoc strings.
- Spec files, plan files, and any docs/ artifacts.
- CHANGELOG or release notes entries.

A pull request MUST NOT be merged if it introduces or modifies
user-visible behavior without a corresponding documentation update.

**Rationale**: Stale documentation erodes trust and increases
onboarding friction. Co-locating doc changes with code changes
prevents drift.

### V. Codebase Consistency Scanning

Before marking a feature as complete, the developer MUST perform a
consistency scan across the entire codebase and documentation:

- Verify that all references to renamed or removed entities are
  updated (function names, CLI flags, config keys, doc references).
- Ensure error messages, log strings, and comments reflect current
  behavior.
- Confirm that examples in documentation compile and run correctly.
- Check that Go module dependencies are tidied (`go mod tidy`).

Any inconsistencies discovered MUST be fixed within the same feature
branch before the pull request is opened for review.

**Rationale**: Incremental changes accumulate inconsistencies that
compound into confusing, unreliable software. Proactive scanning
catches drift before it reaches users.

### VI. End-to-End Tests Always Run

E2E tests MUST execute on every CI run without exception. They MUST
NOT be:

- Skipped via build tags, environment flags, or conditional logic.
- Gated behind manual approval or optional CI stages.
- Disabled due to flakiness (flaky tests MUST be fixed or replaced,
  never skipped).

CI pipelines MUST provision the required PostgreSQL instance
automatically (e.g., via Docker Compose, GitHub Actions service
containers, or equivalent). A CI run where E2E tests did not execute
MUST be treated as a failed run.

**Rationale**: Tests that do not run provide no assurance. Mandatory
E2E execution on every commit ensures the plugin's core contract is
continuously verified.

## Technology Stack & Constraints

- **Language**: Go (latest stable release per Principle I).
- **Plugin Framework**: CloudQuery Plugin SDK for Go.
- **Target Database**: PostgreSQL (all supported versions per
  CloudQuery compatibility matrix).
- **Testing**: Go standard `testing` package; `go test ./...` as the
  single entry point for all test types.
- **E2E Infrastructure**: Docker (PostgreSQL container) for local and
  CI E2E tests.
- **Linting**: `golangci-lint` with project-standard configuration.
- **Build**: `go build` with no CGO unless explicitly justified.
- **CI**: GitHub Actions (or equivalent); MUST run full test suite
  including E2E on every push and pull request.

## Development Workflow

1. **Branch**: Create a feature branch from `main`.
2. **Test First**: Write failing tests per Principle II before any
   implementation.
3. **Implement**: Write minimum code to pass tests; iterate via
   Red-Green-Refactor.
4. **E2E**: Ensure E2E tests pass against a real PostgreSQL instance
   per Principle III.
5. **Document**: Update all affected documentation per Principle IV.
6. **Scan**: Run the consistency scan per Principle V.
7. **Lint & Vet**: Run `golangci-lint run` and `go vet ./...`; fix
   all findings.
8. **PR**: Open pull request; reviewers MUST verify compliance with
   all six principles.
9. **CI Gate**: CI MUST pass all tests including E2E (Principle VI)
   before merge is allowed.

## Governance

This constitution is the authoritative guide for all development on
`cq-source-postgres`. It supersedes conflicting practices, ad-hoc
decisions, and informal conventions.

### Amendment Procedure

1. Propose an amendment via pull request modifying this file.
2. The amendment MUST include a rationale and an impact assessment.
3. At least one maintainer MUST approve the amendment.
4. Version MUST be incremented per semantic versioning:
   - **MAJOR**: Principle removed or fundamentally redefined.
   - **MINOR**: New principle added or existing principle materially
     expanded.
   - **PATCH**: Clarifications, wording fixes, non-semantic
     refinements.

### Compliance Review

- Every pull request review MUST include a constitution compliance
  check.
- Violations MUST be resolved before merge.
- Quarterly audits SHOULD verify ongoing adherence across the
  codebase.

**Version**: 1.0.0 | **Ratified**: 2026-02-16 | **Last Amended**: 2026-02-16
