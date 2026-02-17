#!/usr/bin/env python3
"""Generate E2E test file with correct SDK API."""
import os

E2E_DIR = "e2e"
os.makedirs(E2E_DIR, exist_ok=True)

content = '''package e2e

import (
\t"context"
\t"fmt"
\t"os"
\t"strings"
\t"testing"

\t"github.com/cloudquery/plugin-sdk/v4/message"
\t"github.com/cloudquery/plugin-sdk/v4/plugin"
\t"github.com/jackc/pgx/v5/pgxpool"
\t"github.com/rs/zerolog"
\t"github.com/stretchr/testify/assert"
\t"github.com/stretchr/testify/require"

\tinternalPlugin "github.com/infobloxopen/cq-source-postgres/resources/plugin"
)

const testConnEnv = "CQ_SOURCE_PG_TEST_CONN"

func getTestConnectionString(t *testing.T) string {
\tt.Helper()
\tconn := os.Getenv(testConnEnv)
\tif conn == "" {
\t\tt.Skipf("%s not set, skipping E2E test", testConnEnv)
\t}
\treturn conn
}

func seedDatabase(t *testing.T, connString string) {
\tt.Helper()
\tctx := context.Background()
\tpool, err := pgxpool.New(ctx, connString)
\trequire.NoError(t, err)
\tdefer pool.Close()

\tseedSQL, err := os.ReadFile("testdata/seed.sql")
\trequire.NoError(t, err)

\t_, err = pool.Exec(ctx, string(seedSQL))
\trequire.NoError(t, err, "failed to execute seed SQL")
}

func newPlugin(t *testing.T, connString string) *plugin.Plugin {
\tt.Helper()
\tctx := context.Background()
\tlogger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

\tp := internalPlugin.Plugin()
\tp.SetLogger(logger)

\tspec := fmt.Sprintf(`{"connection_string": "%s"}`, connString)

\terr := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
\trequire.NoError(t, err)

\tt.Cleanup(func() {
\t\trequire.NoError(t, p.Close(ctx))
\t})
\treturn p
}

func syncAll(t *testing.T, p *plugin.Plugin, tables []string) message.SyncMessages {
\tt.Helper()
\tctx := context.Background()

\tmsgs, err := p.SyncAll(ctx, plugin.SyncOptions{
\t\tTables:     tables,
\t\tSkipTables: nil,
\t})
\trequire.NoError(t, err)
\treturn msgs
}

func countMessageTypes(msgs message.SyncMessages) (migrates int, inserts int) {
\tfor _, msg := range msgs {
\t\tswitch msg.(type) {
\t\tcase *message.SyncMigrateTable:
\t\t\tmigrates++
\t\tcase *message.SyncInsert:
\t\t\tinserts++
\t\t}
\t}
\treturn
}

func getMigratedTableNames(msgs message.SyncMessages) []string {
\tvar names []string
\tfor _, msg := range msgs {
\t\tif m, ok := msg.(*message.SyncMigrateTable); ok {
\t\t\tnames = append(names, m.Table.Name)
\t\t}
\t}
\treturn names
}

func getInsertRecordCount(msgs message.SyncMessages) int64 {
\tvar total int64
\tfor _, msg := range msgs {
\t\tif m, ok := msg.(*message.SyncInsert); ok {
\t\t\ttotal += m.Record.NumRows()
\t\t}
\t}
\treturn total
}

// T024: Full pipeline sync test
func TestE2E_FullSync(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)

\t// Get tables first
\ttables, err := p.Tables(context.Background(), plugin.TableOptions{
\t\tTables: []string{"*"},
\t})
\trequire.NoError(t, err)

\t// We should discover at least our 4 test tables
\ttableNames := make([]string, 0, len(tables))
\tfor _, tbl := range tables {
\t\ttableNames = append(tableNames, tbl.Name)
\t}
\tassert.Contains(t, tableNames, "test_types")
\tassert.Contains(t, tableNames, "test_users")
\tassert.Contains(t, tableNames, "test_orders")
\tassert.Contains(t, tableNames, "test_empty_table")

\t// Sync all tables
\tmsgs := syncAll(t, p, []string{"*"})
\trequire.NotEmpty(t, msgs)

\tmigrates, inserts := countMessageTypes(msgs)
\tassert.GreaterOrEqual(t, migrates, 4, "should have at least 4 migrate messages")
\tassert.Greater(t, inserts, 0, "should have insert messages")

\t// Verify total row count: test_types=3, test_users=3, test_orders=3, test_empty_table=0
\ttotalRows := getInsertRecordCount(msgs)
\tassert.GreaterOrEqual(t, totalRows, int64(9), "should have at least 9 rows total")
}

// T024: Type fidelity test
func TestE2E_TypeFidelity(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)

\ttables, err := p.Tables(context.Background(), plugin.TableOptions{
\t\tTables: []string{"test_types"},
\t})
\trequire.NoError(t, err)
\trequire.Len(t, tables, 1)

\ttable := tables[0]
\tassert.Equal(t, "test_types", table.Name)

\texpectedColumns := map[string]string{
\t\t"col_smallint":    "int16",
\t\t"col_integer":     "int32",
\t\t"col_bigint":      "int64",
\t\t"col_real":        "float32",
\t\t"col_double":      "float64",
\t\t"col_boolean":     "bool",
\t\t"col_text":        "utf8",
\t\t"col_uuid":        "uuid",
\t}

\tfor colName, expectedType := range expectedColumns {
\t\tcol := table.Column(colName)
\t\tif assert.NotNilf(t, col, "column %s should exist", colName) {
\t\t\tassert.Containsf(t, strings.ToLower(col.Type.String()), expectedType,
\t\t\t\t"column %s should be %s type", colName, expectedType)
\t\t}
\t}
}

// T024: Empty table emits schema but no data rows
func TestE2E_EmptyTable(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\tmsgs := syncAll(t, p, []string{"test_empty_table"})

\tmigrates, inserts := countMessageTypes(msgs)
\tassert.Equal(t, 1, migrates, "should have 1 migrate message for empty table")
\tassert.Equal(t, 0, inserts, "should have 0 inserts for empty table")

\ttableNames := getMigratedTableNames(msgs)
\tassert.Contains(t, tableNames, "test_empty_table")
}

// T024: NULL preservation test
func TestE2E_NULLPreservation(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\tmsgs := syncAll(t, p, []string{"test_types"})

\tvar hasNulls bool
\tfor _, msg := range msgs {
\t\tif m, ok := msg.(*message.SyncInsert); ok {
\t\t\trec := m.Record
\t\t\tfor i := 0; i < int(rec.NumCols()); i++ {
\t\t\t\tcol := rec.Column(i)
\t\t\t\tfor j := 0; j < col.Len(); j++ {
\t\t\t\t\tif col.IsNull(j) {
\t\t\t\t\t\thasNulls = true
\t\t\t\t\t}
\t\t\t\t}
\t\t\t}
\t\t}
\t}
\tassert.True(t, hasNulls, "should have NULL values in synced data")
}

// T025: Selective table sync
func TestE2E_SelectiveSync(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\tmsgs := syncAll(t, p, []string{"test_users", "test_orders"})

\ttableNames := getMigratedTableNames(msgs)
\tassert.Contains(t, tableNames, "test_users")
\tassert.Contains(t, tableNames, "test_orders")
\tassert.NotContains(t, tableNames, "test_types")
\tassert.NotContains(t, tableNames, "test_empty_table")

\t// Verify row counts: 3 users + 3 orders = 6
\ttotalRows := getInsertRecordCount(msgs)
\tassert.Equal(t, int64(6), totalRows)
}

// T026: Glob pattern sync
func TestE2E_GlobPatternSync(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\tmsgs := syncAll(t, p, []string{"test_*"})

\ttableNames := getMigratedTableNames(msgs)
\tassert.GreaterOrEqual(t, len(tableNames), 4, "glob should match all test_ tables")

\tfor _, name := range tableNames {
\t\tassert.Truef(t, strings.HasPrefix(name, "test_"),
\t\t\t"table %s should start with test_", name)
\t}
}

// T025: Verify tables that don't exist in the glob are excluded
func TestE2E_GlobExcludesNonMatching(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\tmsgs := syncAll(t, p, []string{"test_user*"})

\ttableNames := getMigratedTableNames(msgs)
\tassert.Contains(t, tableNames, "test_users")
\tassert.NotContains(t, tableNames, "test_orders")
\tassert.NotContains(t, tableNames, "test_types")
}

// TestE2E_PrimaryKeyDiscovery verifies that primary keys are correctly identified
func TestE2E_PrimaryKeyDiscovery(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tp := newPlugin(t, connString)
\ttables, err := p.Tables(context.Background(), plugin.TableOptions{
\t\tTables: []string{"test_users"},
\t})
\trequire.NoError(t, err)
\trequire.Len(t, tables, 1)

\tpks := tables[0].PrimaryKeys()
\tassert.Contains(t, pks, "id")
}
'''

with open(os.path.join(E2E_DIR, 'e2e_test.go'), 'w') as f:
    f.write(content)
print("OK: e2e_test.go written")
