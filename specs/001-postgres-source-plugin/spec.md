# Feature Specification: CloudQuery PostgreSQL Source Plugin

**Feature Branch**: `001-postgres-source-plugin`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "implement a cloudquery source plugin for postgres in go"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Basic Table Sync (Priority: P1)

A data engineer configures the plugin with a PostgreSQL connection string and a list of tables (or wildcard `*`), then runs a CloudQuery sync. The plugin connects to PostgreSQL, discovers the requested tables, reads all rows, and emits them as Apache Arrow records to the configured destination.

**Why this priority**: This is the core value proposition of the plugin. Without basic table syncing, no other feature matters.

**Independent Test**: Start a PostgreSQL container with seed data, configure the plugin with `tables: ["*"]`, run a sync, and verify that every table and row appears in the destination output.

**Acceptance Scenarios**:

1. **Given** a PostgreSQL database with 3 tables containing seed data, **When** the user configures `tables: ["*"]` and runs a sync, **Then** all 3 tables are synced and row counts match the source.
2. **Given** a PostgreSQL database with tables `users`, `orders`, `products`, **When** the user configures `tables: ["users", "orders"]`, **Then** only `users` and `orders` are synced; `products` is skipped.
3. **Given** a PostgreSQL database with various column types (text, integer, boolean, timestamp, jsonb, uuid, array, numeric), **When** a sync runs, **Then** all column types are correctly mapped to Apache Arrow types and values are faithfully preserved.
4. **Given** a valid connection string, **When** the plugin starts, **Then** it connects successfully and logs the PostgreSQL server version.

---

### User Story 2 - Connection & Authentication (Priority: P1)

A data engineer provides credentials via a connection string in various formats (URL, DSN, Unix socket, environment variable substitution). The plugin authenticates against PostgreSQL and handles connection errors gracefully.

**Why this priority**: Authentication is a prerequisite for all other functionality. Users must be able to connect before anything else works.

**Independent Test**: Attempt connections with valid credentials, invalid credentials, unreachable host, and various connection string formats; verify success/failure behavior.

**Acceptance Scenarios**:

1. **Given** a valid `postgres://user:pass@host:5432/db?sslmode=disable` connection string, **When** the plugin initializes, **Then** it connects successfully.
2. **Given** a DSN-format connection string `user=jack password=secret host=localhost port=5432 dbname=mydb sslmode=disable`, **When** the plugin initializes, **Then** it connects successfully.
3. **Given** an invalid password in the connection string, **When** the plugin attempts to connect, **Then** it returns a clear authentication error message and exits with a non-zero status.
4. **Given** an unreachable host in the connection string, **When** the plugin attempts to connect, **Then** it returns a connection timeout error within a reasonable period and exits with a non-zero status.
5. **Given** a connection string referencing a Unix socket path, **When** the plugin initializes, **Then** it connects via the Unix socket.

---

### User Story 3 - Plugin Configuration & Validation (Priority: P1)

A data engineer writes a CloudQuery source YAML configuration specifying the plugin name, registry, path, version, tables, destinations, and nested spec options. The plugin validates the configuration at startup and rejects invalid or incomplete configurations with actionable error messages.

**Why this priority**: Configuration is the user's primary interface. Invalid configs must be caught early with clear messages to avoid wasted debugging time.

**Independent Test**: Provide various valid and invalid YAML configurations and verify that the plugin accepts or rejects them with appropriate messages.

**Acceptance Scenarios**:

1. **Given** a configuration with `connection_string` set, **When** the plugin initializes, **Then** it accepts the configuration and proceeds.
2. **Given** a configuration missing `connection_string`, **When** the plugin initializes, **Then** it returns an error stating that `connection_string` is required.
3. **Given** a configuration with `pgx_log_level: "trace"`, **When** the plugin initializes, **Then** pgx logging is set to trace level.
4. **Given** a configuration with `pgx_log_level: "invalid_level"`, **When** the plugin initializes, **Then** it returns an error listing the valid log levels.
5. **Given** a configuration with `rows_per_record: 0` or a negative number, **When** the plugin initializes, **Then** it returns a validation error.
6. **Given** a configuration with no `rows_per_record` specified, **When** the plugin initializes, **Then** it defaults to 500.

---

### User Story 4 - CDC via Logical Replication (Priority: P2)

A data engineer enables Change Data Capture (CDC) by setting a unique `cdc_id` in the plugin configuration. The plugin creates a logical replication slot, subscribes to changes on the configured tables, and streams inserts, updates, and deletes to the destination in real time after an initial full sync.

**Why this priority**: CDC is a differentiating feature that enables real-time data pipelines, but it builds on top of the basic sync infrastructure from P1 stories.

**Independent Test**: Enable CDC, perform DML operations on source tables, and verify that changes appear in the destination output in near real-time.

**Acceptance Scenarios**:

1. **Given** a configuration with `cdc_id: "my-source"`, **When** the plugin starts, **Then** it creates a logical replication slot (if not already existing) and performs an initial full sync.
2. **Given** CDC is active and a new row is inserted into a source table, **When** the change is committed, **Then** the plugin emits the new row to the destination.
3. **Given** CDC is active and an existing row is updated, **When** the change is committed, **Then** the plugin emits the updated row to the destination.
4. **Given** CDC is active and a row is deleted, **When** the change is committed, **Then** the plugin emits a delete event to the destination.
5. **Given** the plugin is stopped and restarted with the same `cdc_id`, **When** it reconnects, **Then** it resumes from where it left off without re-syncing already-captured changes.
6. **Given** two source configurations targeting the same database, **When** each uses a distinct `cdc_id`, **Then** they operate independently without interfering with each other.

---

### User Story 5 - Destination Table Name Templating (Priority: P3)

A data engineer configures `destination_table_name` with placeholder variables to control the naming convention of destination tables. The plugin resolves placeholders at sync time and writes data to the correctly named destination tables.

**Why this priority**: This is a convenience/advanced feature that adds flexibility but is not required for basic or CDC operation.

**Independent Test**: Configure various `destination_table_name` patterns, run a sync, and verify the output table names match expectations.

**Acceptance Scenarios**:

1. **Given** `destination_table_name: "{{TABLE}}"` (default), **When** syncing table `users`, **Then** the destination table is named `users`.
2. **Given** `destination_table_name: "raw_{{TABLE}}"`, **When** syncing table `users`, **Then** the destination table is named `raw_users`.
3. **Given** `destination_table_name: "{{TABLE}}_{{YEAR}}_{{MONTH}}"`, **When** syncing in February 2026, **Then** the destination table is named `users_2026_02`.
4. **Given** `destination_table_name: "{{UUID}}"`, **When** syncing table `users`, **Then** the destination table is named with a valid UUID.
5. **Given** `destination_table_name: "no_placeholder"` (missing both `{{TABLE}}` and `{{UUID}}`), **When** the plugin validates config, **Then** it returns an error stating at least one of `{{TABLE}}` or `{{UUID}}` is required.
6. **Given** CDC mode is enabled and `destination_table_name` contains `{{YEAR}}`, **When** the plugin validates config, **Then** it returns an error stating dynamic placeholders are not allowed in CDC mode.

---

### User Story 6 - Rows Per Record Batching (Priority: P3)

A data engineer configures `rows_per_record` to control how many rows are packed into a single Apache Arrow record during sync. This affects memory usage and throughput characteristics.

**Why this priority**: Batching is an optimization concern. The default of 500 works for most cases; this story provides tuning capability.

**Independent Test**: Sync a table with a known row count using different `rows_per_record` values and verify the correct number of records are emitted with the expected row counts.

**Acceptance Scenarios**:

1. **Given** a table with 1200 rows and `rows_per_record: 500` (default), **When** a sync runs, **Then** 3 Arrow records are emitted (500 + 500 + 200).
2. **Given** a table with 100 rows and `rows_per_record: 1000`, **When** a sync runs, **Then** 1 Arrow record is emitted containing 100 rows.
3. **Given** `rows_per_record: 1`, **When** a sync runs, **Then** each row is emitted as its own Arrow record.

---

### Edge Cases

- What happens when a configured table does not exist in the database? The plugin MUST log a warning and skip the missing table without aborting the sync.
- What happens when the PostgreSQL connection drops mid-sync? The plugin MUST return a clear error indicating the connection was lost and exit with a non-zero status.
- What happens when a table has zero rows? The plugin MUST emit the table schema (columns/types) with zero data records.
- What happens when a table has columns with `NULL` values? The plugin MUST preserve `NULL` values faithfully in the Arrow output.
- What happens when the user specifies tables with a glob/wildcard pattern? The plugin MUST resolve the pattern against the database catalog and sync all matching tables.
- What happens when the database has hundreds of tables and `tables: ["*"]` is used? The plugin MUST handle large schema discovery efficiently without excessive memory usage.
- What happens when a logical replication slot already exists for a given `cdc_id`? The plugin MUST reuse the existing slot and resume from the last confirmed LSN.
- What happens when PostgreSQL lacks the `wal_level=logical` setting required for CDC? The plugin MUST return a clear error explaining the prerequisite configuration.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST connect to PostgreSQL using a connection string provided in the plugin spec configuration.
- **FR-002**: System MUST support connection strings in both URL format (`postgres://...`) and DSN format (`key=value` pairs).
- **FR-003**: System MUST discover all tables in the target database when `tables: ["*"]` is configured.
- **FR-004**: System MUST support explicit table selection via a list of table names or glob patterns.
- **FR-005**: System MUST read all rows from selected tables and emit them as Apache Arrow records.
- **FR-006**: System MUST correctly map PostgreSQL data types to Apache Arrow types (including text, integer, boolean, timestamp, jsonb, uuid, array, numeric, bytea, interval, inet, cidr, macaddr, and composite types).
- **FR-007**: System MUST batch rows into Apache Arrow records according to the `rows_per_record` configuration (default: 500).
- **FR-008**: System MUST support configurable pgx log levels (`error`, `warn`, `info`, `debug`, `trace`) via `pgx_log_level` (default: `error`).
- **FR-009**: System MUST validate all configuration fields at startup and return actionable error messages for invalid values.
- **FR-010**: System MUST implement the CloudQuery Source Plugin SDK interface (`Plugin`, `Client`, `Tables`, `Sync`).
- **FR-011**: System MUST support CDC via PostgreSQL logical replication when `cdc_id` is set to a non-empty string.
- **FR-012**: System MUST perform an initial full sync before switching to streaming mode when CDC is enabled.
- **FR-013**: System MUST resume CDC from the last confirmed LSN after restart when using the same `cdc_id`.
- **FR-014**: System MUST stream inserts, updates, and deletes to the destination during CDC mode.
- **FR-015**: System MUST support `destination_table_name` templating with placeholders: `{{TABLE}}`, `{{UUID}}`, `{{YEAR}}`, `{{MONTH}}`, `{{DAY}}`, `{{HOUR}}`, `{{MINUTE}}`.
- **FR-016**: System MUST require at least one of `{{TABLE}}` or `{{UUID}}` in `destination_table_name`.
- **FR-017**: System MUST reject dynamic time-based placeholders in `destination_table_name` when CDC mode is enabled.
- **FR-018**: System MUST skip missing tables with a warning log rather than aborting the entire sync.
- **FR-019**: System MUST emit table schema (columns and types) even for tables with zero rows.
- **FR-020**: System MUST preserve `NULL` values faithfully in the Arrow output.

### Key Entities

- **Plugin Spec**: The nested configuration object containing `connection_string`, `pgx_log_level`, `cdc_id`, `rows_per_record`, and `destination_table_name`. Parsed from YAML and validated at startup.
- **Table**: A PostgreSQL table discovered via schema introspection. Has a name, schema, columns (with types), and primary key information. Maps to a CloudQuery `Table` resource.
- **Column**: A column within a table, with a PostgreSQL type that maps to an Apache Arrow type. Includes nullability metadata.
- **Arrow Record**: A batch of rows from a single table, serialized as an Apache Arrow record. Contains up to `rows_per_record` rows.
- **Replication Slot**: A PostgreSQL logical replication slot identified by `cdc_id`. Tracks the WAL position (LSN) for CDC streaming.
- **WAL Event**: A change event (insert, update, delete) received from the logical replication stream. Contains the affected table, operation type, and row data.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can sync all tables from a PostgreSQL database to a destination with a single YAML configuration file and one command.
- **SC-002**: The plugin correctly maps all standard PostgreSQL data types to their Arrow equivalents, verified by round-trip tests against a database with all supported types.
- **SC-003**: A sync of 100,000 rows across 10 tables completes within 60 seconds on a standard development machine with a local PostgreSQL instance.
- **SC-004**: CDC mode captures and delivers a new row to the destination within 5 seconds of the source commit.
- **SC-005**: The plugin resumes CDC after restart without re-syncing previously captured data, verified by row-count comparison.
- **SC-006**: 100% of invalid configurations are rejected at startup with an error message that names the invalid field and explains the constraint.
- **SC-007**: End-to-end tests pass against a real PostgreSQL instance on every CI run without manual intervention.
- **SC-008**: All user-facing configuration options are documented with examples in the project README.

## Assumptions

- The plugin targets the CloudQuery Plugin SDK v4+ for Go.
- PostgreSQL versions 12 through 17 are in scope; older versions are not supported.
- CDC requires `wal_level=logical` to be set on the PostgreSQL server; this is a documented prerequisite, not something the plugin configures.
- The plugin runs as a gRPC server managed by the CloudQuery CLI; it does not need its own standalone binary distribution beyond what the SDK provides.
- SSL/TLS connection options are handled via the connection string parameters (`sslmode`, `sslcert`, etc.) and do not require separate plugin configuration fields.
- The `pgoutput` logical decoding plugin (built into PostgreSQL 10+) is used for CDC; third-party decoding plugins are not required.
