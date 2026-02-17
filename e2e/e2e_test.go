package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalPlugin "github.com/infobloxopen/cq-source-postgres/resources/plugin"
)

const testConnEnv = "CQ_SOURCE_PG_TEST_CONN"

func getTestConnectionString(t *testing.T) string {
	t.Helper()
	conn := os.Getenv(testConnEnv)
	if conn == "" {
		t.Skipf("%s not set, skipping E2E test", testConnEnv)
	}
	return conn
}

func seedDatabase(t *testing.T, connString string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	seedSQL, err := os.ReadFile("testdata/seed.sql")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, string(seedSQL))
	require.NoError(t, err, "failed to execute seed SQL")
}

func newPlugin(t *testing.T, connString string) *plugin.Plugin {
	t.Helper()
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{"connection_string": "%s"}`, connString)

	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, p.Close(ctx))
	})
	return p
}

func syncAll(t *testing.T, p *plugin.Plugin, tables []string) message.SyncMessages {
	t.Helper()
	ctx := context.Background()

	msgs, err := p.SyncAll(ctx, plugin.SyncOptions{
		Tables:     tables,
		SkipTables: nil,
	})
	require.NoError(t, err)
	return msgs
}

func countMessageTypes(msgs message.SyncMessages) (migrates int, inserts int) {
	for _, msg := range msgs {
		switch msg.(type) {
		case *message.SyncMigrateTable:
			migrates++
		case *message.SyncInsert:
			inserts++
		}
	}
	return
}

func getMigratedTableNames(msgs message.SyncMessages) []string {
	var names []string
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncMigrateTable); ok {
			names = append(names, m.Table.Name)
		}
	}
	return names
}

func getInsertRecordCount(msgs message.SyncMessages) int64 {
	var total int64
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncInsert); ok {
			total += m.Record.NumRows()
		}
	}
	return total
}

// T024: Full pipeline sync test
func TestE2E_FullSync(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)

	// Get tables first
	tables, err := p.Tables(context.Background(), plugin.TableOptions{
		Tables: []string{"*"},
	})
	require.NoError(t, err)

	// We should discover at least our 4 test tables
	tableNames := make([]string, 0, len(tables))
	for _, tbl := range tables {
		tableNames = append(tableNames, tbl.Name)
	}
	assert.Contains(t, tableNames, "test_types")
	assert.Contains(t, tableNames, "test_users")
	assert.Contains(t, tableNames, "test_orders")
	assert.Contains(t, tableNames, "test_empty_table")

	// Sync all tables
	msgs := syncAll(t, p, []string{"*"})
	require.NotEmpty(t, msgs)

	migrates, inserts := countMessageTypes(msgs)
	assert.GreaterOrEqual(t, migrates, 4, "should have at least 4 migrate messages")
	assert.Greater(t, inserts, 0, "should have insert messages")

	// Verify total row count: test_types=3, test_users=3, test_orders=3, test_empty_table=0
	totalRows := getInsertRecordCount(msgs)
	assert.GreaterOrEqual(t, totalRows, int64(9), "should have at least 9 rows total")
}

// T024: Type fidelity test
func TestE2E_TypeFidelity(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)

	tables, err := p.Tables(context.Background(), plugin.TableOptions{
		Tables: []string{"test_types"},
	})
	require.NoError(t, err)
	require.Len(t, tables, 1)

	table := tables[0]
	assert.Equal(t, "test_types", table.Name)

	expectedColumns := map[string]string{
		"col_smallint": "int16",
		"col_integer":  "int32",
		"col_bigint":   "int64",
		"col_real":     "float32",
		"col_double":   "float64",
		"col_boolean":  "bool",
		"col_text":     "utf8",
		"col_uuid":     "uuid",
	}

	for colName, expectedType := range expectedColumns {
		col := table.Column(colName)
		if assert.NotNilf(t, col, "column %s should exist", colName) {
			assert.Containsf(t, strings.ToLower(col.Type.String()), expectedType,
				"column %s should be %s type", colName, expectedType)
		}
	}
}

// T024: Empty table emits schema but no data rows
func TestE2E_EmptyTable(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	msgs := syncAll(t, p, []string{"test_empty_table"})

	migrates, inserts := countMessageTypes(msgs)
	assert.Equal(t, 1, migrates, "should have 1 migrate message for empty table")
	assert.Equal(t, 0, inserts, "should have 0 inserts for empty table")

	tableNames := getMigratedTableNames(msgs)
	assert.Contains(t, tableNames, "test_empty_table")
}

// T024: NULL preservation test
func TestE2E_NULLPreservation(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	msgs := syncAll(t, p, []string{"test_types"})

	var hasNulls bool
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncInsert); ok {
			rec := m.Record
			for i := 0; i < int(rec.NumCols()); i++ {
				col := rec.Column(i)
				for j := 0; j < col.Len(); j++ {
					if col.IsNull(j) {
						hasNulls = true
					}
				}
			}
		}
	}
	assert.True(t, hasNulls, "should have NULL values in synced data")
}

// T025: Selective table sync
func TestE2E_SelectiveSync(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	msgs := syncAll(t, p, []string{"test_users", "test_orders"})

	tableNames := getMigratedTableNames(msgs)
	assert.Contains(t, tableNames, "test_users")
	assert.Contains(t, tableNames, "test_orders")
	assert.NotContains(t, tableNames, "test_types")
	assert.NotContains(t, tableNames, "test_empty_table")

	// Verify row counts: 3 users + 3 orders = 6
	totalRows := getInsertRecordCount(msgs)
	assert.Equal(t, int64(6), totalRows)
}

// T026: Glob pattern sync
func TestE2E_GlobPatternSync(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	msgs := syncAll(t, p, []string{"test_*"})

	tableNames := getMigratedTableNames(msgs)
	assert.GreaterOrEqual(t, len(tableNames), 4, "glob should match all test_ tables")

	for _, name := range tableNames {
		assert.Truef(t, strings.HasPrefix(name, "test_"),
			"table %s should start with test_", name)
	}
}

// T025: Verify tables that don't exist in the glob are excluded
func TestE2E_GlobExcludesNonMatching(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	msgs := syncAll(t, p, []string{"test_user*"})

	tableNames := getMigratedTableNames(msgs)
	assert.Contains(t, tableNames, "test_users")
	assert.NotContains(t, tableNames, "test_orders")
	assert.NotContains(t, tableNames, "test_types")
}

// TestE2E_PrimaryKeyDiscovery verifies that primary keys are correctly identified
func TestE2E_PrimaryKeyDiscovery(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)
	tables, err := p.Tables(context.Background(), plugin.TableOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)
	require.Len(t, tables, 1)

	pks := tables[0].PrimaryKeys()
	assert.Contains(t, pks, "id")
}

// T031: Valid URL connection E2E
func TestE2E_ConnectionURLFormat(t *testing.T) {
	connString := getTestConnectionString(t)
	// URL format is the default format for the test connection string
	p := newPlugin(t, connString)

	tables, err := p.Tables(context.Background(), plugin.TableOptions{Tables: []string{"*"}})
	require.NoError(t, err)
	assert.Greater(t, len(tables), 0, "should discover tables via URL connection")
}

// T031: DSN format connection E2E
func TestE2E_ConnectionDSNFormat(t *testing.T) {
	_ = getTestConnectionString(t) // skip if no DB

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := `{"connection_string": "host=localhost port=5432 user=cq password=cq dbname=cq_test sslmode=disable"}`
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	tables, err := p.Tables(ctx, plugin.TableOptions{Tables: []string{"*"}})
	require.NoError(t, err)
	assert.Greater(t, len(tables), 0, "should discover tables via DSN connection")
}

// T031: Invalid credentials produce clear error
func TestE2E_ConnectionInvalidPassword(t *testing.T) {
	_ = getTestConnectionString(t) // skip if no DB

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := `{"connection_string": "postgres://cq:wrongpassword@localhost:5432/cq_test?sslmode=disable"}`
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password", "error should mention password")
}

// T031: Missing connection_string returns clear error
func TestE2E_ConnectionMissingConnString(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := `{}`
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection_string")
}

// T036: Invalid pgx_log_level returns error listing valid options
func TestE2E_ConfigInvalidPgxLogLevel(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := `{"connection_string": "postgres://cq:cq@localhost:5432/cq_test?sslmode=disable", "pgx_log_level": "verbose"}`
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pgx_log_level")
}

// T036: Valid config with all fields succeeds
func TestE2E_ConfigFullValid(t *testing.T) {
	connString := getTestConnectionString(t)

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{
		"connection_string": "%s",
		"pgx_log_level": "debug",
		"rows_per_record": 100,
		"destination_table_name": "raw_{{TABLE}}"
	}`, connString)
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	tables, err := p.Tables(ctx, plugin.TableOptions{Tables: []string{"*"}})
	require.NoError(t, err)
	assert.Greater(t, len(tables), 0)
}

// T036: Extra unknown field rejected by JSON Schema
func TestE2E_ConfigExtraFieldRejected(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := `{"connection_string": "postgres://x", "unknown_field": true}`
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	// JSON Schema has additionalProperties=false, so this should fail
	require.Error(t, err)
}

// T050: Template resolution applies destination_table_name to sync messages
func TestE2E_TemplateResolution(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{
		"connection_string": "%s",
		"destination_table_name": "raw_{{TABLE}}"
	}`, connString)

	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	msgs, err := p.SyncAll(ctx, plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	migratedNames := getMigratedTableNames(msgs)
	require.NotEmpty(t, migratedNames, "should have migrate messages")

	// All migrated table names should have the raw_ prefix
	for _, name := range migratedNames {
		assert.True(t, strings.HasPrefix(name, "raw_"), "expected table name to start with raw_, got: %s", name)
	}
}

// T050: Without destination_table_name, table names are unchanged
func TestE2E_TemplateResolutionDefault(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)

	msgs, err := p.SyncAll(context.Background(), plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	migratedNames := getMigratedTableNames(msgs)
	require.NotEmpty(t, migratedNames)

	for _, name := range migratedNames {
		assert.False(t, strings.HasPrefix(name, "raw_"), "default should not prefix, got: %s", name)
	}
}

// T053: Batching — rows_per_record=1 produces one record per row
func TestE2E_BatchingRowsPerRecord1(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{
		"connection_string": "%s",
		"rows_per_record": 1
	}`, connString)

	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	msgs, err := p.SyncAll(ctx, plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	// test_users has 3 rows, with batch=1 each row should be its own record
	var insertCount int
	var totalRows int64
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncInsert); ok {
			insertCount++
			totalRows += m.Record.NumRows()
			assert.Equal(t, int64(1), m.Record.NumRows(), "each insert should have exactly 1 row")
		}
	}
	assert.Equal(t, 3, insertCount, "should have 3 insert messages for 3 rows with batch=1")
	assert.Equal(t, int64(3), totalRows, "total rows should be 3")
}

// T053: Batching — large rows_per_record batches all rows into one record
func TestE2E_BatchingLargeBatchSize(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{
		"connection_string": "%s",
		"rows_per_record": 10000
	}`, connString)

	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	msgs, err := p.SyncAll(ctx, plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	// test_users has 3 rows, with batch=10000 all should fit in 1 record
	var insertCount int
	var totalRows int64
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncInsert); ok {
			insertCount++
			totalRows += m.Record.NumRows()
		}
	}
	assert.Equal(t, 1, insertCount, "should have 1 insert message for 3 rows with large batch size")
	assert.Equal(t, int64(3), totalRows, "total rows should be 3")
}

// T053: Default batch size (500) - all 3 rows in one record
func TestE2E_BatchingDefault(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	p := newPlugin(t, connString)

	msgs, err := p.SyncAll(context.Background(), plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	var totalRows int64
	for _, msg := range msgs {
		if m, ok := msg.(*message.SyncInsert); ok {
			totalRows += m.Record.NumRows()
		}
	}
	assert.Equal(t, int64(3), totalRows, "total rows should be 3")
}
