# Data Model: CloudQuery PostgreSQL Source Plugin

**Feature**: `001-postgres-source-plugin`
**Date**: 2026-02-16
**Source**: [spec.md](spec.md), [research.md](research.md)

## Entities

### 1. PluginSpec

The user-facing configuration parsed from the CloudQuery YAML `spec:` block.

| Field | Type | Required | Default | Validation |
|-------|------|----------|---------|------------|
| `connection_string` | `string` | Yes | — | Non-empty; must be parseable by `pgxpool.ParseConfig()` |
| `pgx_log_level` | `string` | No | `"error"` | One of: `error`, `warn`, `info`, `debug`, `trace` |
| `cdc_id` | `string` | No | `""` | If non-empty, enables CDC mode; must be unique per source |
| `rows_per_record` | `integer` | No | `500` | Must be ≥ 1 |
| `destination_table_name` | `string` | No | `"{{TABLE}}"` | Must contain `{{TABLE}}` or `{{UUID}}`; dynamic placeholders (`{{YEAR}}`, `{{MONTH}}`, `{{DAY}}`, `{{HOUR}}`, `{{MINUTE}}`) forbidden when `cdc_id` is set |

**Go struct**:

```go
type Spec struct {
    ConnectionString     string `json:"connection_string"`
    PgxLogLevel          string `json:"pgx_log_level,omitempty"`
    CdcID                string `json:"cdc_id,omitempty"`
    RowsPerRecord        int    `json:"rows_per_record,omitempty"`
    DestinationTableName string `json:"destination_table_name,omitempty"`
}
```

**Methods**:
- `SetDefaults()` — applies default values for optional fields
- `Validate() error` — returns actionable error for invalid configuration

---

### 2. Client

The runtime state of the plugin, holding the database connection and configuration.

| Field | Type | Description |
|-------|------|-------------|
| `logger` | `zerolog.Logger` | Structured logger |
| `spec` | `Spec` | Parsed and validated configuration |
| `pool` | `*pgxpool.Pool` | Connection pool to PostgreSQL |

**Implements**: `schema.ClientMeta` (provides `ID() string`)

---

### 3. PluginClient

The top-level CloudQuery plugin client that orchestrates table discovery, sync, and CDC.

| Field | Type | Description |
|-------|------|-------------|
| `logger` | `zerolog.Logger` | Structured logger |
| `config` | `client.Spec` | Plugin configuration |
| `tables` | `schema.Tables` | Discovered PostgreSQL tables as CloudQuery schema |
| `client` | `*client.Client` | Database client |
| `scheduler` | `*scheduler.Scheduler` | CloudQuery scheduler for concurrent table resolution |
| `options` | `plugin.NewClientOptions` | SDK options (NoConnection, InvocationID, etc.) |

**Implements**: `plugin.Client` (via `plugin.UnimplementedDestination` embedding)

**Methods**:
- `Tables(ctx, options) (schema.Tables, error)` — returns discovered tables filtered by options
- `Sync(ctx, options, res) error` — performs full sync or CDC sync
- `Close(ctx) error` — closes database connections

---

### 4. Table (runtime, dynamic)

Represents a PostgreSQL table discovered from the database catalog. Not a persistent entity — built at runtime via catalog introspection.

| Field | Type | Source |
|-------|------|--------|
| `schema_name` | `string` | `information_schema.tables.table_schema` |
| `table_name` | `string` | `information_schema.tables.table_name` |
| `columns` | `[]Column` | `information_schema.columns` |
| `primary_keys` | `[]string` | `pg_index` + `pg_attribute` |

**Maps to**: `schema.Table` with dynamically built `schema.Column` list

---

### 5. Column (runtime, dynamic)

Represents a column within a discovered table.

| Field | Type | Source |
|-------|------|--------|
| `column_name` | `string` | `information_schema.columns.column_name` |
| `data_type` | `string` | `information_schema.columns.data_type` |
| `udt_name` | `string` | `information_schema.columns.udt_name` |
| `is_nullable` | `bool` | `information_schema.columns.is_nullable` |
| `ordinal_position` | `int` | `information_schema.columns.ordinal_position` |

**Maps to**: `schema.Column` with Arrow type determined by `pg_type_map.go`

---

### 6. PostgreSQL → Arrow Type Mapping

| PostgreSQL Type | Arrow Type | Notes |
|----------------|------------|-------|
| `smallint`, `int2` | `arrow.PrimitiveTypes.Int16` | |
| `integer`, `int4` | `arrow.PrimitiveTypes.Int32` | |
| `bigint`, `int8` | `arrow.PrimitiveTypes.Int64` | |
| `real`, `float4` | `arrow.PrimitiveTypes.Float32` | |
| `double precision`, `float8` | `arrow.PrimitiveTypes.Float64` | |
| `numeric`, `decimal` | `arrow.BinaryTypes.String` | Preserved as string to avoid precision loss |
| `boolean`, `bool` | `arrow.FixedWidthTypes.Boolean` | |
| `text`, `varchar`, `char`, `name` | `arrow.BinaryTypes.String` | |
| `bytea` | `arrow.BinaryTypes.Binary` | |
| `timestamp`, `timestamp without time zone` | `arrow.FixedWidthTypes.Timestamp_us` | Microsecond precision |
| `timestamptz`, `timestamp with time zone` | `arrow.FixedWidthTypes.Timestamp_us` | Stored as UTC |
| `date` | `arrow.FixedWidthTypes.Date32` | |
| `time`, `time without time zone` | `arrow.BinaryTypes.String` | Serialized as string |
| `timetz`, `time with time zone` | `arrow.BinaryTypes.String` | Serialized as string |
| `interval` | `arrow.BinaryTypes.String` | Serialized as string |
| `uuid` | `types.ExtensionTypes.UUID` | CQ SDK UUID extension |
| `json`, `jsonb` | `types.ExtensionTypes.JSON` | CQ SDK JSON extension |
| `inet`, `cidr` | `types.ExtensionTypes.Inet` | CQ SDK Inet extension |
| `macaddr`, `macaddr8` | `types.ExtensionTypes.MAC` | CQ SDK MAC extension |
| `ARRAY` (any) | `arrow.ListOf(element_type)` | Recursive mapping for element type |
| `xml` | `arrow.BinaryTypes.String` | |
| `point`, `line`, `polygon`, `circle`, `path`, `box`, `lseg` | `arrow.BinaryTypes.String` | Geometry serialized as string |
| `hstore` | `types.ExtensionTypes.JSON` | Serialized as JSON object |
| `oid` | `arrow.PrimitiveTypes.Uint32` | |
| Unknown/composite | `arrow.BinaryTypes.String` | Fallback: serialize as string |

---

### 7. ReplicationState (CDC)

Runtime state for CDC mode, not persisted via the plugin directly — LSN is persisted via the SDK state client.

| Field | Type | Description |
|-------|------|-------------|
| `slot_name` | `string` | Derived from `cdc_id` (e.g., `cq_cdc_{cdc_id}`) |
| `publication_name` | `string` | Derived from `cdc_id` (e.g., `cq_pub_{cdc_id}`) |
| `confirmed_lsn` | `pglogrepl.LSN` | Last WAL position confirmed to PostgreSQL |
| `relations` | `map[uint32]*RelationMessage` | Relation OID → column metadata cache |
| `conn` | `*pgconn.PgConn` | Replication connection |

---

### 8. WALEvent (CDC, transient)

Change events decoded from the logical replication stream. Not persisted — immediately converted to Arrow records and emitted.

| Variant | Fields |
|---------|--------|
| `Insert` | `relation_id`, `new_tuple_data` |
| `Update` | `relation_id`, `old_tuple_data` (if REPLICA IDENTITY FULL), `new_tuple_data` |
| `Delete` | `relation_id`, `old_tuple_data` |

---

## Relationships

```
PluginSpec 1──1 Client        (spec configures client)
Client     1──1 PluginClient  (client embedded in plugin client)
PluginClient 1──* Table       (discovers multiple tables)
Table      1──* Column        (each table has columns)
Column     *──1 TypeMapping   (each column maps to an Arrow type)
PluginClient 0──1 ReplicationState (CDC mode only)
ReplicationState ──* WALEvent (streams change events)
```

## State Transitions

### Plugin Lifecycle

```
UNINITIALIZED → CONFIGURED → CONNECTED → SYNCING → CLOSED
                    │                        │
                    │                        ├── SYNCING (full sync)
                    │                        └── STREAMING (CDC mode)
                    │
                    └── ERROR (validation failure)
```

### CDC State Machine

```
INIT → CREATING_SLOT → INITIAL_SYNC → STREAMING → CLOSED
         │                                │
         └── RESUMING (slot exists) ──────┘
```
