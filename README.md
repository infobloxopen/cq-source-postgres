# CloudQuery PostgreSQL Source Plugin

A [CloudQuery](https://www.cloudquery.io/) source plugin that syncs PostgreSQL tables to any CloudQuery destination. Supports full table sync and CDC (Change Data Capture) via logical replication.

## Features

- **Dynamic table discovery** — automatically discovers all user tables from the PostgreSQL catalog
- **Full type fidelity** — maps 25+ PostgreSQL types to Apache Arrow types (numeric, geometry, arrays, JSON, inet, etc.)
- **CDC support** — real-time change streaming via pglogrepl logical replication
- **Configurable batching** — control Arrow record batch size with `rows_per_record`
- **Destination table naming** — template-based table naming with `{{TABLE}}`, `{{UUID}}`, date placeholders
- **Structured logging** — zerolog with configurable pgx log levels
- **Docker-ready** — multi-stage build, published to ghcr.io

## Installation

### Docker

```bash
docker pull ghcr.io/infobloxopen/cq-source-postgres:latest
```

### Build from source

```bash
go build -o cq-source-postgres .
```

## Configuration

Create a CloudQuery source configuration file:

```yaml
kind: source
spec:
  name: postgres
  path: infobloxopen/cq-source-postgres
  registry: grpc
  destinations:
    - "your-destination"
  tables:
    - "*"             # Sync all tables (supports glob patterns)
  # skip_tables:
  #   - "pg_*"        # Skip system tables
  spec:
    connection_string: "postgres://user:password@localhost:5432/mydb?sslmode=disable"
    # pgx_log_level: "error"              # error, warn, info, debug, trace
    # rows_per_record: 500                # Rows per Apache Arrow record batch
    # destination_table_name: "{{TABLE}}" # Template for destination table names
    # cdc_id: ""                          # Set to enable CDC mode (must be unique per source)
```

### Configuration Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `connection_string` | `string` | Yes | — | PostgreSQL connection string (URL or DSN format) |
| `pgx_log_level` | `string` | No | `"error"` | pgx driver log level: `error`, `warn`, `info`, `debug`, `trace` |
| `rows_per_record` | `integer` | No | `500` | Number of rows per Apache Arrow record batch (≥ 1) |
| `destination_table_name` | `string` | No | `"{{TABLE}}"` | Template for destination table names |
| `cdc_id` | `string` | No | `""` | Unique identifier to enable CDC mode |

### Destination Table Name Placeholders

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `{{TABLE}}` | Source table name | `users` |
| `{{UUID}}` | Random UUID | `a1b2c3d4-...` |
| `{{YEAR}}` | Current year | `2026` |
| `{{MONTH}}` | Current month (zero-padded) | `02` |
| `{{DAY}}` | Current day (zero-padded) | `16` |
| `{{HOUR}}` | Current hour (zero-padded) | `14` |
| `{{MINUTE}}` | Current minute (zero-padded) | `30` |

**Note**: Dynamic date/time placeholders (`{{YEAR}}`, `{{MONTH}}`, etc.) cannot be used when `cdc_id` is set.

## CDC Setup

To use Change Data Capture, your PostgreSQL instance must have:

1. **`wal_level = logical`** in `postgresql.conf`
2. The connecting user must have `REPLICATION` privilege
3. Set a unique `cdc_id` in the plugin configuration

```yaml
spec:
  connection_string: "postgres://user:password@localhost:5432/mydb?sslmode=disable"
  cdc_id: "my-sync-source"
```

The plugin will automatically:
- Create a publication (`cq_pub_<cdc_id>`) for all tables
- Create a replication slot (`cq_slot_<cdc_id>`)
- Perform an initial full sync, then stream changes

## Supported PostgreSQL Types

| PostgreSQL Type | Arrow Type |
|-----------------|------------|
| `smallint` | `Int16` |
| `integer` | `Int32` |
| `bigint` | `Int64` |
| `oid` | `Uint32` |
| `real` | `Float32` |
| `double precision` | `Float64` |
| `numeric/decimal` | `String` |
| `boolean` | `Boolean` |
| `text/varchar/char/name` | `String` |
| `bytea` | `Binary` |
| `uuid` | `UUID` (CQ extension) |
| `json/jsonb` | `JSON` (CQ extension) |
| `inet/cidr` | `Inet` (CQ extension) |
| `macaddr/macaddr8` | `MAC` (CQ extension) |
| `timestamp/timestamptz` | `Timestamp(us)` |
| `date` | `Date32` |
| `time/timetz/interval` | `String` |
| `point/line/lseg/box/circle/path/polygon` | `String` |
| `xml/money/bit/varbit/tsvector/tsquery` | `String` |
| Array types (e.g., `_int4`) | `List(element_type)` |

## Docker Usage

### Run with gRPC server

```bash
docker run -p 7777:7777 ghcr.io/infobloxopen/cq-source-postgres:latest
```

### Custom address

```bash
docker run -p 9999:9999 ghcr.io/infobloxopen/cq-source-postgres:latest \
  serve --address [::]:9999
```

## Development

### Prerequisites

- Go 1.25+
- Docker (for E2E tests)

### Setup

```bash
# Start test PostgreSQL
docker compose -f e2e/docker-compose.yml up -d

# Run all tests
export CQ_SOURCE_PG_TEST_CONN="postgres://cq:cq@localhost:5432/cq_test?sslmode=disable"
make test

# Lint
make lint

# Build
make build
```

### Project Structure

```
├── main.go                          # Entry point
├── client/                          # Connection, configuration, type mapping
│   ├── client.go                    # PostgreSQL client (pgxpool)
│   ├── spec.go                      # Plugin spec (config validation)
│   ├── pg_type_map.go               # PostgreSQL → Arrow type mapping
│   └── pgx_logger.go               # pgx log adapter for zerolog
├── resources/
│   ├── plugin/                      # CloudQuery plugin interface
│   │   ├── plugin.go                # Plugin registration
│   │   └── client.go                # PluginClient (Sync, Tables, Close)
│   └── services/                    # Business logic
│       ├── tables.go                # Table discovery from pg_catalog
│       ├── sync.go                  # Row resolver + type conversion
│       ├── cdc.go                   # CDC via logical replication
│       └── template.go             # Destination table name resolution
├── e2e/                             # End-to-end tests
│   ├── docker-compose.yml           # PostgreSQL 16 test instance
│   ├── e2e_test.go                  # Full sync + config E2E tests
│   └── cdc_e2e_test.go              # CDC E2E tests
├── Dockerfile                       # Multi-stage build
└── .github/workflows/               # CI/CD
    ├── test.yml                     # Test + lint on push/PR
    └── release.yml                  # Build + push Docker on tag
```

## License

See [LICENSE](LICENSE) for details.
