# Quickstart: cq-source-postgres

## Prerequisites

- Go 1.25+ installed
- Docker installed (for PostgreSQL and E2E tests)
- CloudQuery CLI installed (optional, for full pipeline testing)

## 1. Clone & Build

```bash
git clone git@github.com:infobloxopen/cq-source-postgres.git
cd cq-source-postgres
go mod download
go build -o cq-source-postgres .
```

## 2. Start PostgreSQL for Testing

```bash
docker run -d --name cq-postgres-test \
  -e POSTGRES_USER=cq \
  -e POSTGRES_PASSWORD=cq \
  -e POSTGRES_DB=cq_test \
  -p 5432:5432 \
  postgres:16

# Wait for PostgreSQL to be ready
until docker exec cq-postgres-test pg_isready -U cq; do sleep 1; done
```

## 3. Run Tests

```bash
# Set connection string for tests
export CQ_SOURCE_PG_TEST_CONN="postgres://cq:cq@localhost:5432/cq_test?sslmode=disable"

# Run all tests (unit + E2E)
go test ./...

# Run only E2E tests
go test ./e2e/...
```

## 4. Run the Plugin (Developer Mode)

Start the plugin as a gRPC server:

```bash
./cq-source-postgres serve --address localhost:7777 --log-level info
```

Then configure CloudQuery CLI to connect to it:

```yaml
# config.yml
kind: source
spec:
  name: "postgresql"
  registry: "grpc"
  path: "localhost:7777"
  tables: ["*"]
  destinations: ["postgresql"]
  spec:
    connection_string: "postgres://cq:cq@localhost:5432/cq_test?sslmode=disable"
```

Run a sync:

```bash
cloudquery sync config.yml
```

## 5. Build & Run Docker Container

```bash
# Build the container
docker build -t cq-source-postgres:dev .

# Run the container (gRPC server on port 7777)
docker run -p 7777:7777 cq-source-postgres:dev
```

## 6. CDC Mode

To enable CDC, PostgreSQL must have `wal_level=logical`:

```bash
docker run -d --name cq-postgres-cdc \
  -e POSTGRES_USER=cq \
  -e POSTGRES_PASSWORD=cq \
  -e POSTGRES_DB=cq_test \
  -p 5432:5432 \
  -c wal_level=logical \
  postgres:16
```

Configure the plugin with a `cdc_id`:

```yaml
spec:
  connection_string: "postgres://cq:cq@localhost:5432/cq_test?sslmode=disable"
  cdc_id: "my-source"
```

## 7. Lint & Vet

```bash
go vet ./...
golangci-lint run
```

## Configuration Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `connection_string` | string | Yes | — | PostgreSQL connection string (URL or DSN format) |
| `pgx_log_level` | string | No | `error` | pgx log level: error, warn, info, debug, trace |
| `cdc_id` | string | No | — | Unique ID to enable CDC mode (logical replication) |
| `rows_per_record` | integer | No | `500` | Rows per Apache Arrow record batch |
| `destination_table_name` | string | No | `{{TABLE}}` | Destination table name template with placeholders |
