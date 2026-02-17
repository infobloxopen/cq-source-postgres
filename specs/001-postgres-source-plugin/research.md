# CloudQuery Plugin SDK for Go — Research Notes

**Date**: 2026-02-16  
**Scope**: SDK v4, Go interfaces/method signatures, project layout, Dockerfile patterns, GitHub Actions, gRPC default port, pgx v5 usage, logical replication/CDC  

---

## 1. Latest SDK Version & Key Interfaces

### Version

| Dependency | Version |
|---|---|
| `github.com/cloudquery/plugin-sdk/v4` | **v4.94.2** (scaffold template as of 2026-02-14) |
| `github.com/apache/arrow-go/v18` | v18.5.1 |
| `github.com/cloudquery/plugin-pb-go` | v1.27.6 |
| `github.com/rs/zerolog` | v1.34.0 |

### Core Interfaces

**`plugin.Client`** — the combined interface a plugin must implement:

```go
type Client interface {
    SourceClient
    DestinationClient
    TransformerClient
}
```

**`plugin.SourceClient`** — implement this for a source plugin:

```go
type SourceClient interface {
    Close(ctx context.Context) error
    Tables(ctx context.Context, options TableOptions) (schema.Tables, error)
    Sync(ctx context.Context, options SyncOptions, res chan<- message.SyncMessage) error
}
```

**`plugin.NewClientFunc`** — factory registered with the plugin:

```go
type NewClientFunc func(context.Context, zerolog.Logger, []byte, NewClientOptions) (Client, error)
```

**`NewClientOptions`**:

```go
type NewClientOptions struct {
    NoConnection bool       // true when only table metadata is needed (e.g. doc generation)
    InvocationID string
    PluginMeta   Meta
}
```

**`plugin.SyncOptions`**:

```go
type SyncOptions struct {
    Tables              []string
    SkipTables          []string
    SkipDependentTables bool
    DeterministicCQID   bool
    BackendOptions      *BackendOptions
    Shard               *Shard
}
```

**`schema.ClientMeta`** — the minimal interface the scheduler passes to resolvers:

```go
type ClientMeta interface {
    ID() string
}
```

**`plugin.UnimplementedDestination`** — embed this to satisfy `DestinationClient` + `TransformerClient` when building a source-only plugin:

```go
type UnimplementedDestination struct {
    UnimplementedTransformer
}
```

### Plugin Constructor

```go
func NewPlugin(name string, version string, newClient NewClientFunc, options ...Option) *Plugin
```

Key options:
- `plugin.WithKind(kind string)` — `"source"` or `"destination"`
- `plugin.WithTeam(team string)` — org/team name
- `plugin.WithJSONSchema(schema string)` — spec validation schema
- `plugin.WithConnectionTester(fn ConnectionTester)` — test-connection callback
- `plugin.WithBuildTargets(targets []BuildTarget)` — cross-compilation targets

---

## 2. gRPC Server Exposure

### Default Config

| Setting | Value |
|---|---|
| **Default address** | `localhost:7777` |
| **Network** | `tcp` |
| **Max message size** | 100 MiB (`100 * 1024 * 1024`) |
| **Protocol versions** | discovery v0/v1, plugin v3, destination v0/v1 (optional) |

### serve Package

```go
import "github.com/cloudquery/plugin-sdk/v4/serve"

func main() {
    if err := serve.Plugin(plugin.Plugin()).Serve(context.Background()); err != nil {
        log.Fatalf("failed to serve plugin: %v", err)
    }
}
```

The CLI `serve` command accepts `--address` (default `localhost:7777`), `--network` (default `tcp`), `--log-level`, `--log-format`, and `--no-sentry` flags.

### Running in Developer Mode

```yaml
kind: source
spec:
  name: "postgres"
  registry: "grpc"
  path: "localhost:7777"
  tables: ["*"]
  destinations: ["postgresql"]
```

---

## 3. Recommended Project Structure

The official scaffold (`cloudquery scaffold source <org> <name>`) generates:

```
cq-source-<name>/
├── main.go                          # Entry point: serve.Plugin(plugin.Plugin()).Serve()
├── go.mod
├── Makefile                         # test, lint, gen-docs targets
├── .gitignore
├── test/
│   └── config.yml                   # Test configuration (registry: local)
├── client/
│   ├── client.go                    # Client struct implementing schema.ClientMeta (ID())
│   └── spec.go                      # Spec struct with SetDefaults() and Validate()
├── resources/
│   ├── plugin/
│   │   ├── plugin.go                # Plugin() constructor with name/version/team
│   │   └── client.go                # Configure(), getTables(), Sync(), Tables(), Close()
│   └── services/
│       └── table.go                 # Individual table definitions + resolvers
└── docs/
    └── tables/                      # Auto-generated table documentation
```

### Scaffold `plugin.go` Template

```go
package plugin

import "github.com/cloudquery/plugin-sdk/v4/plugin"

var (
    Name    = "postgres"
    Kind    = "source"
    Team    = "infobloxopen"
    Version = "development"
)

func Plugin() *plugin.Plugin {
    return plugin.NewPlugin(Name, Version, Configure,
        plugin.WithKind(Kind),
        plugin.WithTeam(Team),
    )
}
```

### Scaffold `client.go` Template (resources/plugin)

```go
type Client struct {
    logger     zerolog.Logger
    config     client.Spec
    tables     schema.Tables
    syncClient *client.Client
    scheduler  *scheduler.Scheduler

    plugin.UnimplementedDestination  // satisfies DestinationClient + TransformerClient
}

func Configure(ctx context.Context, logger zerolog.Logger, spec []byte, opts plugin.NewClientOptions) (plugin.Client, error) {
    if opts.NoConnection {
        return &Client{logger: logger, tables: getTables()}, nil
    }
    // parse spec, create client, build tables, create scheduler
    // ...
}
```

### Table Initialization Pattern

```go
func getTables() schema.Tables {
    tables := schema.Tables{
        services.SampleTable(),
    }
    if err := transformers.TransformTables(tables); err != nil {
        panic(err)
    }
    for _, t := range tables {
        schema.AddCqIDs(t)
        schema.AddCqClientID(t)
    }
    return tables
}
```

---

## 4. Apache Arrow Record Serialization

### Wire Format

Records are serialized via **Apache Arrow IPC format** (using `arrow/ipc` package). The `plugin-pb-go` package provides `SchemaToBytes()` / `SchemasToBytes()` helpers.

### Internal CQ Columns

Every table automatically gets these columns (via `schema.AddCqIDs()` and `schema.AddCqClientID()`):

| Column | Type | Purpose |
|---|---|---|
| `_cq_id` | `types.ExtensionTypes.UUID` | Unique row identifier (deterministic hash or random) |
| `_cq_parent_id` | `types.ExtensionTypes.UUID` | Parent row reference for relations |
| `_cq_client_id` | `arrow.BinaryTypes.String` | Multiplexed client identifier |

Destination-managed columns (added by destination plugins):

| Column | Type |
|---|---|
| `_cq_sync_time` | `arrow.FixedWidthTypes.Timestamp_us` |
| `_cq_source_name` | `arrow.BinaryTypes.String` |

### Custom Arrow Extension Types

The SDK provides custom Arrow extension types in `github.com/cloudquery/plugin-sdk/v4/types`:

- `types.ExtensionTypes.UUID`
- `types.ExtensionTypes.Inet`
- `types.ExtensionTypes.MAC`
- `types.ExtensionTypes.JSON`

### SyncMessage Types

```go
type SyncMessage interface { IsSyncMessage() bool }

// Emitted during sync:
type SyncMigrateTable struct { Table *schema.Table }
type SyncInsert struct { Record arrow.RecordBatch }
```

---

## 5. Key Dependencies

| Package | Import Path | Purpose |
|---|---|---|
| Plugin SDK v4 | `github.com/cloudquery/plugin-sdk/v4` | Core plugin framework |
| Plugin Protobuf | `github.com/cloudquery/plugin-pb-go` | gRPC protocol definitions |
| Arrow Go v18 | `github.com/apache/arrow-go/v18` | Columnar data format |
| zerolog | `github.com/rs/zerolog` | Structured logging |
| cobra | `github.com/spf13/cobra` | CLI command framework (used by serve) |
| jsonschema v6 | `github.com/santhosh-tekuri/jsonschema/v6` | Spec validation |
| errgroup | `golang.org/x/sync/errgroup` | Concurrent goroutine management |
| semaphore | `golang.org/x/sync/semaphore` | Concurrency limiting |
| testify | `github.com/stretchr/testify` | Testing assertions |

**For our PostgreSQL plugin, additionally:**

| Package | Import Path | Purpose |
|---|---|---|
| pgx v5 | `github.com/jackc/pgx/v5` | PostgreSQL driver |
| pglogrepl | `github.com/jackc/pglogrepl` | Logical replication protocol |
| pgconn | `github.com/jackc/pgx/v5/pgconn` | Low-level PostgreSQL connection |

---

## 6. Table Discovery & Schema Definition

### Table Struct

```go
type Table struct {
    Name                     string
    Title                    string
    Description              string
    Columns                  ColumnList
    Relations                Tables             // child tables
    Transform                Transform          // e.g. TransformWithStruct()
    Resolver                 TableResolver      // fetches data
    Multiplex                Multiplexer        // fan-out by client
    PreResourceResolver      RowResolver        // runs before column resolvers
    PostResourceResolver     RowResolver        // runs after column resolvers
    PreResourceChunkResolver *RowsChunkResolver // batch pre-resolver
    IsIncremental            bool               // incremental sync support
    IgnoreInTests            bool
}
```

### Column Struct

```go
type Column struct {
    Name             string
    Type             arrow.DataType  // e.g. arrow.BinaryTypes.String, arrow.PrimitiveTypes.Int64
    Description      string
    Resolver         ColumnResolver
    PrimaryKey       bool
    PrimaryKeyComponent bool
    NotNull          bool
    IncrementalKey   bool
    Unique           bool
    IgnoreInTests    bool
    TypeSchema       string
}
```

### Resolver Signatures

```go
// Table resolver — fetches all items for a table, sends them to `res` channel
type TableResolver func(ctx context.Context, meta ClientMeta, parent *Resource, res chan<- any) error

// Column resolver — sets a single column value on a resource
type ColumnResolver func(ctx context.Context, meta ClientMeta, resource *Resource, c Column) error

// Row resolver — pre/post processing of a resource
type RowResolver func(ctx context.Context, meta ClientMeta, resource *Resource) error
```

### Resolver Execution Order

1. **TableResolver** — fetches items, sends to `res` channel
2. Per item received from `res`:
   a. **PreResourceResolver** — optional pre-processing
   b. **Column resolvers** — each column's resolver populates its value
   c. **PostResourceResolver** — optional post-processing

### Built-in Resolvers

```go
// Resolves a field from Resource.Item by dotted path (e.g. "Address.City")
schema.PathResolver("field_name")

// Resolves from parent resource's column value
schema.ParentColumnResolver("parent_column_name")
```

### Example Table Definition (Manual Columns)

```go
func PostgresTable() *schema.Table {
    return &schema.Table{
        Name:     "pg_sample_table",
        Resolver: fetchSampleTable,
        Columns: []schema.Column{
            {
                Name:       "id",
                Type:       arrow.PrimitiveTypes.Int64,
                PrimaryKey: true,
                NotNull:    true,
            },
            {
                Name: "name",
                Type: arrow.BinaryTypes.String,
            },
            {
                Name: "created_at",
                Type: arrow.FixedWidthTypes.Timestamp_us,
            },
            {
                Name: "metadata",
                Type: types.ExtensionTypes.JSON,
            },
        },
    }
}
```

### Transformers (Struct-Based Auto-Discovery)

`transformers.TransformWithStruct()` generates columns automatically from a Go struct:

```go
type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    IsActive  bool      `json:"is_active"`
}

func UsersTable() *schema.Table {
    return &schema.Table{
        Name:      "users",
        Resolver:  fetchUsers,
        Transform: transformers.TransformWithStruct(&User{},
            transformers.WithPrimaryKeys("ID"),
            transformers.WithSkipFields("InternalField"),
        ),
    }
}
```

**Go type → Arrow type mapping** (via `DefaultTypeTransformer`):

| Go Type | Arrow Type |
|---|---|
| `int`, `int64` | `arrow.PrimitiveTypes.Int64` |
| `string` | `arrow.BinaryTypes.String` |
| `bool` | `arrow.FixedWidthTypes.Boolean` |
| `time.Time` | `arrow.FixedWidthTypes.Timestamp_us` |
| `net.IP` | Inet extension type |
| `[]byte` | `arrow.BinaryTypes.Binary` |
| `struct` (nested) | `types.ExtensionTypes.JSON` |

**TransformWithStruct Options:**
- `WithPrimaryKeys(fields ...string)` — mark fields as primary keys
- `WithPrimaryKeyComponents(fields ...string)` — mark as PK components
- `WithSkipFields(fields ...string)` — exclude fields
- `WithUnwrapAllEmbeddedStructs()` — flatten embedded structs
- `WithUnwrapStructFields(fields ...string)` — flatten specific struct fields
- `WithTypeTransformer(fn)` — custom Go→Arrow type mapping
- `WithNameTransformer(fn)` — custom field→column name mapping (default: json tag or CamelCase→snake_case)
- `WithResolverTransformer(fn)` — custom resolver per field
- `WithNullableFieldTransformer(fn)` — custom nullability logic

### Multiplexer Pattern

Fan-out table resolution across multiple clients (e.g., per-database, per-schema):

```go
func MultiplexBySchema(meta schema.ClientMeta) []schema.ClientMeta {
    cl := meta.(*Client)
    schemas := cl.DiscoverSchemas()
    clients := make([]schema.ClientMeta, len(schemas))
    for i, s := range schemas {
        clients[i] = cl.WithSchema(s)
    }
    return clients
}
```

---

## 7. CDC / Logical Replication Patterns

The CloudQuery Plugin SDK **does not provide built-in CDC/logical replication support**. This is custom functionality that must be implemented in the source plugin using PostgreSQL-specific libraries.

### Recommended Approach

Use `pglogrepl` (part of the pgx ecosystem) for logical replication:

```go
import "github.com/jackc/pglogrepl"
```

Key components:
1. **Replication connection** — use `pgconn.Connect()` with `replication=database` parameter
2. **Publication** — create with `CREATE PUBLICATION <name> FOR TABLE ...`
3. **Replication slot** — create with `pglogrepl.CreateReplicationSlot()`
4. **Start replication** — `pglogrepl.StartReplication()` to begin streaming WAL
5. **Decode messages** — parse `pgoutput` protocol messages (Begin, Relation, Insert, Update, Delete, Commit)
6. **Standby status** — send periodic `pglogrepl.SendStandbyStatusUpdate()` to confirm LSN

### Integration with SDK

The CDC stream should be implemented within the `Sync()` method:

1. Perform initial full sync (emit `SyncMigrateTable` + `SyncInsert` messages)
2. Record the current LSN
3. Switch to streaming mode:
   - Receive WAL messages via `pglogrepl`
   - Convert to Arrow records
   - Emit via `res chan<- message.SyncMessage`
4. Use `BackendOptions` (state client) to persist the confirmed LSN across restarts

### State Client (for LSN Persistence)

The SDK provides a state client for incremental syncs:

```go
import "github.com/cloudquery/plugin-sdk/v4/state"

type Client interface {
    SetKey(ctx context.Context, key string, value string) error
    GetKey(ctx context.Context, key string) (string, error)
    Flush(ctx context.Context) error
    Close() error
}
```

The state is stored via `SyncOptions.BackendOptions.Connection` (pointing to a destination plugin).

---

## 8. Recommended Go Version

| Setting | Value |
|---|---|
| **go directive** | `go 1.25.7` (from scaffold `go.mod.tpl`) |

---

## 9. Docker Container Packaging

### Go Plugin Pattern

Go plugins in the CloudQuery ecosystem typically **do not use Docker** for distribution. They are distributed as **native binaries** via CloudQuery Hub using `cloudquery plugin publish`.

However, for self-hosted deployment (e.g., pushing to ghcr.io), the recommended Dockerfile pattern (derived from Python/Java/Node plugin patterns that all expose port 7777):

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o cq-source-postgres .

FROM alpine:3.21
COPY --from=builder /app/cq-source-postgres /usr/local/bin/cq-source-postgres

EXPOSE 7777

ENTRYPOINT ["cq-source-postgres"]
CMD ["serve", "--address", "[::]:7777", "--log-format", "json", "--log-level", "info"]
```

**Key conventions:**
- All CloudQuery plugins expose port **7777**
- The `serve` command is the default entrypoint
- `--address [::]:7777` listens on all interfaces (required in containers)
- JSON log format for container log aggregation

### Native Binary Publishing (Official Method)

```bash
# Build the plugin package
go run main.go package -m "Release v1.0.0" v1.0.0 .

# Publish to CloudQuery Hub
cloudquery plugin publish --finalize
```

---

## 10. GitHub Actions Workflow

### Scaffold Guidance

The scaffold README states:
> 1. Run `git tag v1.0.0` to create a new tag for the release
> 2. Run `git push origin v1.0.0` to push the tag to GitHub
> 
> Once the tag is pushed, a new GitHub Actions workflow will be triggered to build the release binaries and create the new release on CloudQuery Hub.

### Recommended Workflows

The scaffold generates a test badge pointing to `.github/workflows/test.yaml`. For our project publishing to **ghcr.io**, recommended workflows:

**`.github/workflows/test.yaml`** — CI on push/PR:

```yaml
name: test
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: cq
          POSTGRES_PASSWORD: cq
          POSTGRES_DB: cq_test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test
      - run: make lint
```

**`.github/workflows/release.yaml`** — Build + push to ghcr.io on tag:

```yaml
name: release
on:
  push:
    tags: ["v*"]

permissions:
  contents: read
  packages: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

---

## Summary: Key Takeaways for Our Plugin

1. **Implement `plugin.Client`** by embedding `plugin.UnimplementedDestination` and providing `Close()`, `Tables()`, `Sync()`.
2. **Use `scheduler.NewScheduler()`** for concurrent table resolution — pass it to `Sync()` which calls `scheduler.Sync(ctx, client, tables, res)`.
3. **Register via `plugin.NewPlugin()`** with `WithKind("source")`, `WithTeam("infobloxopen")`.
4. **gRPC on port 7777** — no custom configuration needed; the SDK handles this.
5. **Apache Arrow v18** for all data serialization; use extension types (UUID, Inet, JSON, MAC) for PostgreSQL-specific types.
6. **CDC is custom** — use `pglogrepl` library, implement within `Sync()`, persist LSN via state client.
7. **Go 1.25.7** as the target Go version.
8. **Docker** — Expose 7777, multi-stage build, `serve --address [::]:7777` as CMD.
9. **Publish via ghcr.io** — Docker build+push workflow triggered on git tags.
10. **Tables are dynamic** — since we discover PostgreSQL tables at runtime, we likely won't use `TransformWithStruct()` but instead build `schema.Table`/`schema.Column` dynamically from PostgreSQL catalog queries (`information_schema` or `pg_catalog`).
