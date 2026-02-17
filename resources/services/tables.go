package services

import (
	"context"
	"fmt"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/infobloxopen/cq-source-postgres/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// systemSchemas contains PostgreSQL system schemas excluded from discovery.
var systemSchemas = map[string]bool{
	"pg_catalog":         true,
	"information_schema": true,
	"pg_toast":           true,
}

// IsSystemSchema returns true if the schema should be excluded from discovery.
func IsSystemSchema(name string) bool {
	return systemSchemas[name]
}

// TableInfo holds discovered table metadata from the database catalog.
type TableInfo struct {
	Schema  string
	Name    string
	Columns []ColumnInfo
	PKs     []string
}

// ColumnInfo holds discovered column metadata.
type ColumnInfo struct {
	Name       string
	UDTName    string
	IsNullable bool
	Position   int
}

// BuildTable converts a TableInfo to a schema.Table.
// The resolver parameter is the TableResolver to attach; pass nil for test-only usage.
func BuildTable(info TableInfo, resolver schema.TableResolver) *schema.Table {
	columns := make(schema.ColumnList, 0, len(info.Columns))
	pkSet := make(map[string]bool, len(info.PKs))
	for _, pk := range info.PKs {
		pkSet[pk] = true
	}

	for _, col := range info.Columns {
		arrowType := client.PgTypeToArrow(col.UDTName)
		columns = append(columns, schema.Column{
			Name:       col.Name,
			Type:       arrowType,
			PrimaryKey: pkSet[col.Name],
			NotNull:    !col.IsNullable,
			Resolver:   schema.PathResolver(col.Name),
		})
	}

	tableName := info.Name
	if info.Schema != "public" {
		tableName = info.Schema + "." + info.Name
	}

	table := &schema.Table{
		Name:     tableName,
		Columns:  columns,
		Resolver: resolver,
	}

	schema.AddCqIDs(table)
	return table
}

// DiscoverTables queries the PostgreSQL catalog and returns all user tables as schema.Tables.
func DiscoverTables(ctx context.Context, pool *pgxpool.Pool, logger zerolog.Logger) (schema.Tables, error) {
	tableRows, err := pool.Query(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
		  AND table_schema NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY table_schema, table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer tableRows.Close()

	var tableInfos []TableInfo
	for tableRows.Next() {
		var info TableInfo
		if err := tableRows.Scan(&info.Schema, &info.Name); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tableInfos = append(tableInfos, info)
	}
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	for i := range tableInfos {
		info := &tableInfos[i]
		if err := discoverColumns(ctx, pool, info); err != nil {
			return nil, err
		}
		if err := discoverPrimaryKeys(ctx, pool, info, logger); err != nil {
			return nil, err
		}
	}

	tables := make(schema.Tables, 0, len(tableInfos))
	for _, info := range tableInfos {
		resolver := MakeResolver(pool, info.Schema, info.Name)
		tables = append(tables, BuildTable(info, resolver))
	}
	return tables, nil
}

func discoverColumns(ctx context.Context, pool *pgxpool.Pool, info *TableInfo) error {
	rows, err := pool.Query(ctx, `
		SELECT column_name, udt_name, is_nullable, ordinal_position
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`, info.Schema, info.Name)
	if err != nil {
		return fmt.Errorf("failed to query columns for %s.%s: %w", info.Schema, info.Name, err)
	}
	defer rows.Close()

	for rows.Next() {
		var col ColumnInfo
		var isNullableStr string
		if err := rows.Scan(&col.Name, &col.UDTName, &isNullableStr, &col.Position); err != nil {
			return fmt.Errorf("failed to scan column row: %w", err)
		}
		col.IsNullable = isNullableStr == "YES"
		info.Columns = append(info.Columns, col)
	}
	return rows.Err()
}

func discoverPrimaryKeys(ctx context.Context, pool *pgxpool.Pool, info *TableInfo, logger zerolog.Logger) error {
	rows, err := pool.Query(ctx, `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		  AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)
	`, info.Schema, info.Name)
	if err != nil {
		logger.Warn().Err(err).Msgf("failed to query PKs for %s.%s", info.Schema, info.Name)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var pkName string
		if err := rows.Scan(&pkName); err != nil {
			return fmt.Errorf("failed to scan PK row: %w", err)
		}
		info.PKs = append(info.PKs, pkName)
	}
	return rows.Err()
}
