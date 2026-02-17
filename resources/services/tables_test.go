package services

import (
	"context"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTable_BasicColumns(t *testing.T) {
	info := TableInfo{
		Schema: "public",
		Name:   "users",
		Columns: []ColumnInfo{
			{Name: "id", UDTName: "int4", IsNullable: false, Position: 1},
			{Name: "name", UDTName: "text", IsNullable: false, Position: 2},
			{Name: "email", UDTName: "varchar", IsNullable: true, Position: 3},
		},
		PKs: []string{"id"},
	}

	table := BuildTable(info, nil)

	require.NotNil(t, table)
	assert.Equal(t, "users", table.Name)

	userCols := filterUserColumns(table.Columns)
	require.Len(t, userCols, 3)

	assert.Equal(t, "id", userCols[0].Name)
	assert.Equal(t, arrow.PrimitiveTypes.Int32, userCols[0].Type)
	assert.True(t, userCols[0].PrimaryKey)
	assert.True(t, userCols[0].NotNull)

	assert.Equal(t, "name", userCols[1].Name)
	assert.Equal(t, arrow.BinaryTypes.String, userCols[1].Type)

	assert.Equal(t, "email", userCols[2].Name)
	assert.False(t, userCols[2].NotNull)
}

func TestBuildTable_NonPublicSchema(t *testing.T) {
	info := TableInfo{
		Schema:  "analytics",
		Name:    "events",
		Columns: []ColumnInfo{{Name: "id", UDTName: "int8", IsNullable: false, Position: 1}},
		PKs:     []string{"id"},
	}
	table := BuildTable(info, nil)
	assert.Equal(t, "analytics.events", table.Name)
}

func TestBuildTable_AllColumnTypes(t *testing.T) {
	columns := []ColumnInfo{
		{Name: "col_int2", UDTName: "int2", Position: 1},
		{Name: "col_int4", UDTName: "int4", Position: 2},
		{Name: "col_int8", UDTName: "int8", Position: 3},
		{Name: "col_float4", UDTName: "float4", Position: 4},
		{Name: "col_float8", UDTName: "float8", Position: 5},
		{Name: "col_numeric", UDTName: "numeric", Position: 6},
		{Name: "col_bool", UDTName: "bool", Position: 7},
		{Name: "col_text", UDTName: "text", Position: 8},
		{Name: "col_bytea", UDTName: "bytea", Position: 9},
		{Name: "col_timestamp", UDTName: "timestamp", Position: 10},
		{Name: "col_timestamptz", UDTName: "timestamptz", Position: 11},
		{Name: "col_date", UDTName: "date", Position: 12},
		{Name: "col_uuid", UDTName: "uuid", Position: 13},
		{Name: "col_json", UDTName: "json", Position: 14},
		{Name: "col_jsonb", UDTName: "jsonb", Position: 15},
		{Name: "col_inet", UDTName: "inet", Position: 16},
		{Name: "col_macaddr", UDTName: "macaddr", Position: 17},
		{Name: "col_int_arr", UDTName: "_int4", Position: 18},
		{Name: "col_text_arr", UDTName: "_text", Position: 19},
	}

	info := TableInfo{Schema: "public", Name: "all_types", Columns: columns}
	table := BuildTable(info, nil)
	userCols := filterUserColumns(table.Columns)

	expectedTypes := map[string]arrow.DataType{
		"col_int2":        arrow.PrimitiveTypes.Int16,
		"col_int4":        arrow.PrimitiveTypes.Int32,
		"col_int8":        arrow.PrimitiveTypes.Int64,
		"col_float4":      arrow.PrimitiveTypes.Float32,
		"col_float8":      arrow.PrimitiveTypes.Float64,
		"col_numeric":     arrow.BinaryTypes.String,
		"col_bool":        arrow.FixedWidthTypes.Boolean,
		"col_text":        arrow.BinaryTypes.String,
		"col_bytea":       arrow.BinaryTypes.Binary,
		"col_timestamp":   arrow.FixedWidthTypes.Timestamp_us,
		"col_timestamptz": arrow.FixedWidthTypes.Timestamp_us,
		"col_date":        arrow.FixedWidthTypes.Date32,
		"col_uuid":        types.ExtensionTypes.UUID,
		"col_json":        types.ExtensionTypes.JSON,
		"col_jsonb":       types.ExtensionTypes.JSON,
		"col_inet":        types.ExtensionTypes.Inet,
		"col_macaddr":     types.ExtensionTypes.MAC,
		"col_int_arr":     arrow.ListOf(arrow.PrimitiveTypes.Int32),
		"col_text_arr":    arrow.ListOf(arrow.BinaryTypes.String),
	}

	for _, col := range userCols {
		expected, ok := expectedTypes[col.Name]
		if ok {
			assert.Equal(t, expected, col.Type, "type mismatch for column %s", col.Name)
		}
	}
}

func TestBuildTable_NoPrimaryKeys(t *testing.T) {
	info := TableInfo{
		Schema:  "public",
		Name:    "no_pk_table",
		Columns: []ColumnInfo{{Name: "data", UDTName: "text", Position: 1}},
		PKs:     nil,
	}
	table := BuildTable(info, nil)
	cqIDCol := table.Column("_cq_id")
	require.NotNil(t, cqIDCol)
	assert.True(t, cqIDCol.PrimaryKey)
}

func TestBuildTable_MultiplePrimaryKeys(t *testing.T) {
	info := TableInfo{
		Schema: "public",
		Name:   "composite_pk",
		Columns: []ColumnInfo{
			{Name: "tenant_id", UDTName: "int4", IsNullable: false, Position: 1},
			{Name: "user_id", UDTName: "int4", IsNullable: false, Position: 2},
			{Name: "name", UDTName: "text", IsNullable: true, Position: 3},
		},
		PKs: []string{"tenant_id", "user_id"},
	}
	table := BuildTable(info, nil)
	userCols := filterUserColumns(table.Columns)
	assert.True(t, userCols[0].PrimaryKey, "tenant_id should be PK")
	assert.True(t, userCols[1].PrimaryKey, "user_id should be PK")
	assert.False(t, userCols[2].PrimaryKey, "name should not be PK")
}

func TestBuildTable_HasResolver(t *testing.T) {
	resolver := func(_ context.Context, _ schema.ClientMeta, _ *schema.Resource, _ chan<- any) error {
		return nil
	}
	info := TableInfo{
		Schema:  "public",
		Name:    "test",
		Columns: []ColumnInfo{{Name: "id", UDTName: "int4", Position: 1}},
	}
	table := BuildTable(info, resolver)
	assert.NotNil(t, table.Resolver)
}

func TestIsSystemSchema(t *testing.T) {
	assert.True(t, IsSystemSchema("pg_catalog"))
	assert.True(t, IsSystemSchema("information_schema"))
	assert.True(t, IsSystemSchema("pg_toast"))
	assert.False(t, IsSystemSchema("public"))
	assert.False(t, IsSystemSchema("myapp"))
	assert.False(t, IsSystemSchema("analytics"))
}

func TestBuildTable_ColumnResolvers(t *testing.T) {
	info := TableInfo{
		Schema: "public",
		Name:   "resolver_test",
		Columns: []ColumnInfo{
			{Name: "id", UDTName: "int4", Position: 1},
			{Name: "name", UDTName: "text", Position: 2},
		},
	}
	table := BuildTable(info, nil)
	userCols := filterUserColumns(table.Columns)
	for _, col := range userCols {
		assert.NotNil(t, col.Resolver, "column %s should have a resolver", col.Name)
	}
}

func filterUserColumns(cols schema.ColumnList) schema.ColumnList {
	var result schema.ColumnList
	for _, c := range cols {
		if c.Name != "_cq_id" && c.Name != "_cq_parent_id" {
			result = append(result, c)
		}
	}
	return result
}
