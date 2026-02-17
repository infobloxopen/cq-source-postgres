#!/usr/bin/env python3
"""Generate Go source files for the services package."""
import os

SERVICES_DIR = "resources/services"
os.makedirs(SERVICES_DIR, exist_ok=True)

# --- tables_test.go ---
tables_test = '''package services

import (
\t"context"
\t"testing"

\t"github.com/apache/arrow-go/v18/arrow"
\t"github.com/cloudquery/plugin-sdk/v4/schema"
\t"github.com/cloudquery/plugin-sdk/v4/types"
\t"github.com/stretchr/testify/assert"
\t"github.com/stretchr/testify/require"
)

func TestBuildTable_BasicColumns(t *testing.T) {
\tinfo := TableInfo{
\t\tSchema: "public",
\t\tName:   "users",
\t\tColumns: []ColumnInfo{
\t\t\t{Name: "id", UDTName: "int4", IsNullable: false, Position: 1},
\t\t\t{Name: "name", UDTName: "text", IsNullable: false, Position: 2},
\t\t\t{Name: "email", UDTName: "varchar", IsNullable: true, Position: 3},
\t\t},
\t\tPKs: []string{"id"},
\t}

\ttable := BuildTable(info, nil)

\trequire.NotNil(t, table)
\tassert.Equal(t, "users", table.Name)

\tuserCols := filterUserColumns(table.Columns)
\trequire.Len(t, userCols, 3)

\tassert.Equal(t, "id", userCols[0].Name)
\tassert.Equal(t, arrow.PrimitiveTypes.Int32, userCols[0].Type)
\tassert.True(t, userCols[0].PrimaryKey)
\tassert.True(t, userCols[0].NotNull)

\tassert.Equal(t, "name", userCols[1].Name)
\tassert.Equal(t, arrow.BinaryTypes.String, userCols[1].Type)

\tassert.Equal(t, "email", userCols[2].Name)
\tassert.False(t, userCols[2].NotNull)
}

func TestBuildTable_NonPublicSchema(t *testing.T) {
\tinfo := TableInfo{
\t\tSchema:  "analytics",
\t\tName:    "events",
\t\tColumns: []ColumnInfo{{Name: "id", UDTName: "int8", IsNullable: false, Position: 1}},
\t\tPKs:     []string{"id"},
\t}
\ttable := BuildTable(info, nil)
\tassert.Equal(t, "analytics.events", table.Name)
}

func TestBuildTable_AllColumnTypes(t *testing.T) {
\tcolumns := []ColumnInfo{
\t\t{Name: "col_int2", UDTName: "int2", Position: 1},
\t\t{Name: "col_int4", UDTName: "int4", Position: 2},
\t\t{Name: "col_int8", UDTName: "int8", Position: 3},
\t\t{Name: "col_float4", UDTName: "float4", Position: 4},
\t\t{Name: "col_float8", UDTName: "float8", Position: 5},
\t\t{Name: "col_numeric", UDTName: "numeric", Position: 6},
\t\t{Name: "col_bool", UDTName: "bool", Position: 7},
\t\t{Name: "col_text", UDTName: "text", Position: 8},
\t\t{Name: "col_bytea", UDTName: "bytea", Position: 9},
\t\t{Name: "col_timestamp", UDTName: "timestamp", Position: 10},
\t\t{Name: "col_timestamptz", UDTName: "timestamptz", Position: 11},
\t\t{Name: "col_date", UDTName: "date", Position: 12},
\t\t{Name: "col_uuid", UDTName: "uuid", Position: 13},
\t\t{Name: "col_json", UDTName: "json", Position: 14},
\t\t{Name: "col_jsonb", UDTName: "jsonb", Position: 15},
\t\t{Name: "col_inet", UDTName: "inet", Position: 16},
\t\t{Name: "col_macaddr", UDTName: "macaddr", Position: 17},
\t\t{Name: "col_int_arr", UDTName: "_int4", Position: 18},
\t\t{Name: "col_text_arr", UDTName: "_text", Position: 19},
\t}

\tinfo := TableInfo{Schema: "public", Name: "all_types", Columns: columns}
\ttable := BuildTable(info, nil)
\tuserCols := filterUserColumns(table.Columns)

\texpectedTypes := map[string]arrow.DataType{
\t\t"col_int2":        arrow.PrimitiveTypes.Int16,
\t\t"col_int4":        arrow.PrimitiveTypes.Int32,
\t\t"col_int8":        arrow.PrimitiveTypes.Int64,
\t\t"col_float4":      arrow.PrimitiveTypes.Float32,
\t\t"col_float8":      arrow.PrimitiveTypes.Float64,
\t\t"col_numeric":     arrow.BinaryTypes.String,
\t\t"col_bool":        arrow.FixedWidthTypes.Boolean,
\t\t"col_text":        arrow.BinaryTypes.String,
\t\t"col_bytea":       arrow.BinaryTypes.Binary,
\t\t"col_timestamp":   arrow.FixedWidthTypes.Timestamp_us,
\t\t"col_timestamptz": arrow.FixedWidthTypes.Timestamp_us,
\t\t"col_date":        arrow.FixedWidthTypes.Date32,
\t\t"col_uuid":        types.ExtensionTypes.UUID,
\t\t"col_json":        types.ExtensionTypes.JSON,
\t\t"col_jsonb":       types.ExtensionTypes.JSON,
\t\t"col_inet":        types.ExtensionTypes.Inet,
\t\t"col_macaddr":     types.ExtensionTypes.MAC,
\t\t"col_int_arr":     arrow.ListOf(arrow.PrimitiveTypes.Int32),
\t\t"col_text_arr":    arrow.ListOf(arrow.BinaryTypes.String),
\t}

\tfor _, col := range userCols {
\t\texpected, ok := expectedTypes[col.Name]
\t\tif ok {
\t\t\tassert.Equal(t, expected, col.Type, "type mismatch for column %s", col.Name)
\t\t}
\t}
}

func TestBuildTable_NoPrimaryKeys(t *testing.T) {
\tinfo := TableInfo{
\t\tSchema:  "public",
\t\tName:    "no_pk_table",
\t\tColumns: []ColumnInfo{{Name: "data", UDTName: "text", Position: 1}},
\t\tPKs:     nil,
\t}
\ttable := BuildTable(info, nil)
\tcqIDCol := table.Column("_cq_id")
\trequire.NotNil(t, cqIDCol)
\tassert.True(t, cqIDCol.PrimaryKey)
}

func TestBuildTable_MultiplePrimaryKeys(t *testing.T) {
\tinfo := TableInfo{
\t\tSchema: "public",
\t\tName:   "composite_pk",
\t\tColumns: []ColumnInfo{
\t\t\t{Name: "tenant_id", UDTName: "int4", IsNullable: false, Position: 1},
\t\t\t{Name: "user_id", UDTName: "int4", IsNullable: false, Position: 2},
\t\t\t{Name: "name", UDTName: "text", IsNullable: true, Position: 3},
\t\t},
\t\tPKs: []string{"tenant_id", "user_id"},
\t}
\ttable := BuildTable(info, nil)
\tuserCols := filterUserColumns(table.Columns)
\tassert.True(t, userCols[0].PrimaryKey, "tenant_id should be PK")
\tassert.True(t, userCols[1].PrimaryKey, "user_id should be PK")
\tassert.False(t, userCols[2].PrimaryKey, "name should not be PK")
}

func TestBuildTable_HasResolver(t *testing.T) {
\tresolver := func(_ context.Context, _ schema.ClientMeta, _ *schema.Resource, _ chan<- any) error {
\t\treturn nil
\t}
\tinfo := TableInfo{
\t\tSchema:  "public",
\t\tName:    "test",
\t\tColumns: []ColumnInfo{{Name: "id", UDTName: "int4", Position: 1}},
\t}
\ttable := BuildTable(info, resolver)
\tassert.NotNil(t, table.Resolver)
}

func TestIsSystemSchema(t *testing.T) {
\tassert.True(t, IsSystemSchema("pg_catalog"))
\tassert.True(t, IsSystemSchema("information_schema"))
\tassert.True(t, IsSystemSchema("pg_toast"))
\tassert.False(t, IsSystemSchema("public"))
\tassert.False(t, IsSystemSchema("myapp"))
\tassert.False(t, IsSystemSchema("analytics"))
}

func TestBuildTable_ColumnResolvers(t *testing.T) {
\tinfo := TableInfo{
\t\tSchema: "public",
\t\tName:   "resolver_test",
\t\tColumns: []ColumnInfo{
\t\t\t{Name: "id", UDTName: "int4", Position: 1},
\t\t\t{Name: "name", UDTName: "text", Position: 2},
\t\t},
\t}
\ttable := BuildTable(info, nil)
\tuserCols := filterUserColumns(table.Columns)
\tfor _, col := range userCols {
\t\tassert.NotNil(t, col.Resolver, "column %s should have a resolver", col.Name)
\t}
}

func filterUserColumns(cols schema.ColumnList) schema.ColumnList {
\tvar result schema.ColumnList
\tfor _, c := range cols {
\t\tif c.Name != "_cq_id" && c.Name != "_cq_parent_id" {
\t\t\tresult = append(result, c)
\t\t}
\t}
\treturn result
}
'''

with open(os.path.join(SERVICES_DIR, 'tables_test.go'), 'w') as f:
    f.write(tables_test)
print("OK: tables_test.go")

# --- sync_test.go ---
sync_test = '''package services

import (
\t"testing"

\t"github.com/stretchr/testify/assert"
)

func TestMakeResolver_ReturnsFunction(t *testing.T) {
\tresolver := MakeResolver(nil, "public", "users")
\tassert.NotNil(t, resolver)
}
'''

with open(os.path.join(SERVICES_DIR, 'sync_test.go'), 'w') as f:
    f.write(sync_test)
print("OK: sync_test.go")

# --- tables.go ---
tables_go = '''package services

import (
\t"context"
\t"fmt"

\t"github.com/cloudquery/plugin-sdk/v4/schema"
\t"github.com/infobloxopen/cq-source-postgres/client"
\t"github.com/jackc/pgx/v5/pgxpool"
\t"github.com/rs/zerolog"
)

// systemSchemas contains PostgreSQL system schemas excluded from discovery.
var systemSchemas = map[string]bool{
\t"pg_catalog":         true,
\t"information_schema": true,
\t"pg_toast":           true,
}

// IsSystemSchema returns true if the schema should be excluded from discovery.
func IsSystemSchema(name string) bool {
\treturn systemSchemas[name]
}

// TableInfo holds discovered table metadata from the database catalog.
type TableInfo struct {
\tSchema  string
\tName    string
\tColumns []ColumnInfo
\tPKs     []string
}

// ColumnInfo holds discovered column metadata.
type ColumnInfo struct {
\tName       string
\tUDTName    string
\tIsNullable bool
\tPosition   int
}

// BuildTable converts a TableInfo to a schema.Table.
// The resolver parameter is the TableResolver to attach; pass nil for test-only usage.
func BuildTable(info TableInfo, resolver schema.TableResolver) *schema.Table {
\tcolumns := make(schema.ColumnList, 0, len(info.Columns))
\tpkSet := make(map[string]bool, len(info.PKs))
\tfor _, pk := range info.PKs {
\t\tpkSet[pk] = true
\t}

\tfor _, col := range info.Columns {
\t\tarrowType := client.PgTypeToArrow(col.UDTName)
\t\tcolumns = append(columns, schema.Column{
\t\t\tName:       col.Name,
\t\t\tType:       arrowType,
\t\t\tPrimaryKey: pkSet[col.Name],
\t\t\tNotNull:    !col.IsNullable,
\t\t\tResolver:   schema.PathResolver(col.Name),
\t\t})
\t}

\ttableName := info.Name
\tif info.Schema != "public" {
\t\ttableName = info.Schema + "." + info.Name
\t}

\ttable := &schema.Table{
\t\tName:     tableName,
\t\tColumns:  columns,
\t\tResolver: resolver,
\t}

\tschema.AddCqIDs(table)
\treturn table
}

// DiscoverTables queries the PostgreSQL catalog and returns all user tables as schema.Tables.
func DiscoverTables(ctx context.Context, pool *pgxpool.Pool, logger zerolog.Logger) (schema.Tables, error) {
\ttableRows, err := pool.Query(ctx, `
\t\tSELECT table_schema, table_name
\t\tFROM information_schema.tables
\t\tWHERE table_type = 'BASE TABLE'
\t\t  AND table_schema NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
\t\tORDER BY table_schema, table_name
\t`)
\tif err != nil {
\t\treturn nil, fmt.Errorf("failed to query tables: %w", err)
\t}
\tdefer tableRows.Close()

\tvar tableInfos []TableInfo
\tfor tableRows.Next() {
\t\tvar info TableInfo
\t\tif err := tableRows.Scan(&info.Schema, &info.Name); err != nil {
\t\t\treturn nil, fmt.Errorf("failed to scan table row: %w", err)
\t\t}
\t\ttableInfos = append(tableInfos, info)
\t}
\tif err := tableRows.Err(); err != nil {
\t\treturn nil, fmt.Errorf("error iterating tables: %w", err)
\t}

\tfor i := range tableInfos {
\t\tinfo := &tableInfos[i]
\t\tif err := discoverColumns(ctx, pool, info); err != nil {
\t\t\treturn nil, err
\t\t}
\t\tif err := discoverPrimaryKeys(ctx, pool, info, logger); err != nil {
\t\t\treturn nil, err
\t\t}
\t}

\ttables := make(schema.Tables, 0, len(tableInfos))
\tfor _, info := range tableInfos {
\t\tresolver := MakeResolver(pool, info.Schema, info.Name)
\t\ttables = append(tables, BuildTable(info, resolver))
\t}
\treturn tables, nil
}

func discoverColumns(ctx context.Context, pool *pgxpool.Pool, info *TableInfo) error {
\trows, err := pool.Query(ctx, `
\t\tSELECT column_name, udt_name, is_nullable, ordinal_position
\t\tFROM information_schema.columns
\t\tWHERE table_schema = $1 AND table_name = $2
\t\tORDER BY ordinal_position
\t`, info.Schema, info.Name)
\tif err != nil {
\t\treturn fmt.Errorf("failed to query columns for %s.%s: %w", info.Schema, info.Name, err)
\t}
\tdefer rows.Close()

\tfor rows.Next() {
\t\tvar col ColumnInfo
\t\tvar isNullableStr string
\t\tif err := rows.Scan(&col.Name, &col.UDTName, &isNullableStr, &col.Position); err != nil {
\t\t\treturn fmt.Errorf("failed to scan column row: %w", err)
\t\t}
\t\tcol.IsNullable = isNullableStr == "YES"
\t\tinfo.Columns = append(info.Columns, col)
\t}
\treturn rows.Err()
}

func discoverPrimaryKeys(ctx context.Context, pool *pgxpool.Pool, info *TableInfo, logger zerolog.Logger) error {
\trows, err := pool.Query(ctx, `
\t\tSELECT a.attname
\t\tFROM pg_index i
\t\tJOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
\t\tJOIN pg_class c ON c.oid = i.indrelid
\t\tJOIN pg_namespace n ON n.oid = c.relnamespace
\t\tWHERE n.nspname = $1
\t\t  AND c.relname = $2
\t\t  AND i.indisprimary
\t\tORDER BY array_position(i.indkey, a.attnum)
\t`, info.Schema, info.Name)
\tif err != nil {
\t\tlogger.Warn().Err(err).Msgf("failed to query PKs for %s.%s", info.Schema, info.Name)
\t\treturn nil
\t}
\tdefer rows.Close()

\tfor rows.Next() {
\t\tvar pkName string
\t\tif err := rows.Scan(&pkName); err != nil {
\t\t\treturn fmt.Errorf("failed to scan PK row: %w", err)
\t\t}
\t\tinfo.PKs = append(info.PKs, pkName)
\t}
\treturn rows.Err()
}
'''

with open(os.path.join(SERVICES_DIR, 'tables.go'), 'w') as f:
    f.write(tables_go)
print("OK: tables.go")

# --- sync.go ---
sync_go = '''package services

import (
\t"context"
\t"fmt"

\t"github.com/cloudquery/plugin-sdk/v4/schema"
\t"github.com/jackc/pgx/v5/pgxpool"
)

// MakeResolver creates a TableResolver that reads all rows from the specified PostgreSQL table.
func MakeResolver(pool *pgxpool.Pool, schemaName, tableName string) schema.TableResolver {
\treturn func(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
\t\tif pool == nil {
\t\t\treturn fmt.Errorf("database pool is nil")
\t\t}
\t\tqualifiedName := fmt.Sprintf("%q.%q", schemaName, tableName)
\t\tquery := fmt.Sprintf("SELECT * FROM %s", qualifiedName)

\t\trows, err := pool.Query(ctx, query)
\t\tif err != nil {
\t\t\treturn fmt.Errorf("failed to query %s: %w", qualifiedName, err)
\t\t}
\t\tdefer rows.Close()

\t\tfieldDescs := rows.FieldDescriptions()
\t\tfor rows.Next() {
\t\t\tvalues, err := rows.Values()
\t\t\tif err != nil {
\t\t\t\treturn fmt.Errorf("failed to read row from %s: %w", qualifiedName, err)
\t\t\t}
\t\t\trow := make(map[string]any, len(fieldDescs))
\t\t\tfor i, fd := range fieldDescs {
\t\t\t\trow[string(fd.Name)] = values[i]
\t\t\t}
\t\t\tres <- row
\t\t}

\t\treturn rows.Err()
\t}
}
'''

with open(os.path.join(SERVICES_DIR, 'sync.go'), 'w') as f:
    f.write(sync_go)
print("OK: sync.go")
print("ALL FILES WRITTEN")
