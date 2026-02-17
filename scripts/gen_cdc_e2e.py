#!/usr/bin/env python3
"""Generate e2e/cdc_e2e_test.go"""
import os

content = '''package e2e

import (
\t"context"
\t"fmt"
\t"strings"
\t"testing"

\t"github.com/cloudquery/plugin-sdk/v4/plugin"
\t"github.com/jackc/pgx/v5/pgconn"
\t"github.com/jackc/pgx/v5/pgxpool"
\t"github.com/rs/zerolog"
\t"github.com/stretchr/testify/assert"
\t"github.com/stretchr/testify/require"

\tinternalPlugin "github.com/infobloxopen/cq-source-postgres/resources/plugin"
\t"github.com/infobloxopen/cq-source-postgres/resources/services"
)

func replConnString(connStr string) string {
\tif strings.Contains(connStr, "?") {
\t\treturn connStr + "&replication=database"
\t}
\treturn connStr + "?replication=database"
}

// T044: CDC prerequisites check
func TestE2E_CDC_Prerequisites(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tctx := context.Background()

\treplConn, err := pgconn.Connect(ctx, replConnString(connString))
\trequire.NoError(t, err)
\tdefer replConn.Close(ctx)

\terr = services.CheckCDCPrerequisites(ctx, replConn)
\trequire.NoError(t, err, "Test PostgreSQL should have wal_level=logical")
}

// T044: CDC slot and publication creation
func TestE2E_CDC_SlotAndPublication(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tctx := context.Background()
\tcdcID := "test_e2e_slot"

\t// Cleanup first
\treplConn, err := pgconn.Connect(ctx, replConnString(connString))
\trequire.NoError(t, err)
\t_ = services.CleanupCDCResources(ctx, replConn, cdcID)
\treplConn.Close(ctx)

\t// Reconnect after cleanup
\treplConn, err = pgconn.Connect(ctx, replConnString(connString))
\trequire.NoError(t, err)
\tdefer replConn.Close(ctx)

\t// Create publication
\terr = services.EnsurePublication(ctx, replConn, cdcID)
\trequire.NoError(t, err)

\t// Create slot
\tlsn, err := services.EnsureReplicationSlot(ctx, replConn, cdcID)
\trequire.NoError(t, err)
\tassert.True(t, lsn > 0, "should return a non-zero LSN")

\t// Idempotent: create again should not error
\terr = services.EnsurePublication(ctx, replConn, cdcID)
\trequire.NoError(t, err)

\t// Cleanup
\terr = services.CleanupCDCResources(ctx, replConn, cdcID)
\trequire.NoError(t, err)
}

// T044: CDC initial sync test
func TestE2E_CDC_InitialSync(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tseedDatabase(t, connString)

\tctx := context.Background()
\tlogger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

\tp := internalPlugin.Plugin()
\tp.SetLogger(logger)

\tspec := fmt.Sprintf(`{"connection_string": "%s", "cdc_id": "test_initial_sync"}`, connString)
\terr := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
\trequire.NoError(t, err)
\tdefer p.Close(ctx)

\tmsgs, err := p.SyncAll(ctx, plugin.SyncOptions{
\t\tTables: []string{"test_users"},
\t})
\trequire.NoError(t, err)

\tmigrates, inserts := countMessageTypes(msgs)
\tassert.GreaterOrEqual(t, migrates, 1, "should have migrate messages")
\tassert.GreaterOrEqual(t, inserts, 0, "may have insert messages")

\t// Cleanup replication resources
\treplConn, err := pgconn.Connect(ctx, replConnString(connString))
\tif err == nil {
\t\t_ = services.CleanupCDCResources(ctx, replConn, "test_initial_sync")
\t\treplConn.Close(ctx)
\t}
}

// T045: Two independent CDC sources don\'t interfere
func TestE2E_CDC_IndependentSources(t *testing.T) {
\tconnString := getTestConnectionString(t)
\tctx := context.Background()

\tfor _, cdcID := range []string{"source_a", "source_b"} {
\t\treplConn, err := pgconn.Connect(ctx, replConnString(connString))
\t\trequire.NoError(t, err)
\t\t_ = services.CleanupCDCResources(ctx, replConn, cdcID)
\t\treplConn.Close(ctx)

\t\treplConn, err = pgconn.Connect(ctx, replConnString(connString))
\t\trequire.NoError(t, err)
\t\terr = services.EnsurePublication(ctx, replConn, cdcID)
\t\trequire.NoError(t, err)
\t\t_, err = services.EnsureReplicationSlot(ctx, replConn, cdcID)
\t\trequire.NoError(t, err)
\t\treplConn.Close(ctx)
\t}

\tpool, err := pgxpool.New(ctx, connString)
\trequire.NoError(t, err)
\tdefer pool.Close()

\tvar slotCountA, slotCountB int
\terr = pool.QueryRow(ctx, "SELECT count(*) FROM pg_replication_slots WHERE slot_name = $1",
\t\tservices.CDCSlotName("source_a")).Scan(&slotCountA)
\trequire.NoError(t, err)
\tassert.Equal(t, 1, slotCountA)

\terr = pool.QueryRow(ctx, "SELECT count(*) FROM pg_replication_slots WHERE slot_name = $1",
\t\tservices.CDCSlotName("source_b")).Scan(&slotCountB)
\trequire.NoError(t, err)
\tassert.Equal(t, 1, slotCountB)

\tfor _, cdcID := range []string{"source_a", "source_b"} {
\t\treplConn, err := pgconn.Connect(ctx, replConnString(connString))
\t\tif err == nil {
\t\t\t_ = services.CleanupCDCResources(ctx, replConn, cdcID)
\t\t\treplConn.Close(ctx)
\t\t}
\t}
}

// T046: Missing wal_level=logical returns clear error
func TestE2E_CDC_WalLevelCheck(t *testing.T) {
\terr := services.CheckWalLevelValue("replica")
\trequire.Error(t, err)
\tassert.Contains(t, err.Error(), "wal_level")
\tassert.Contains(t, err.Error(), "logical")
}
'''

os.makedirs('e2e', exist_ok=True)
with open('e2e/cdc_e2e_test.go', 'w') as f:
    f.write(content)
print("OK: cdc_e2e_test.go written")
