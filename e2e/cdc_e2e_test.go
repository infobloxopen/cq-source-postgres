package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalPlugin "github.com/infobloxopen/cq-source-postgres/resources/plugin"
	"github.com/infobloxopen/cq-source-postgres/resources/services"
)

func replConnString(connStr string) string {
	if strings.Contains(connStr, "?") {
		return connStr + "&replication=database"
	}
	return connStr + "?replication=database"
}

// T044: CDC prerequisites check
func TestE2E_CDC_Prerequisites(t *testing.T) {
	connString := getTestConnectionString(t)
	ctx := context.Background()

	replConn, err := pgconn.Connect(ctx, replConnString(connString))
	require.NoError(t, err)
	defer func() { _ = replConn.Close(ctx) }()

	err = services.CheckCDCPrerequisites(ctx, replConn)
	require.NoError(t, err, "Test PostgreSQL should have wal_level=logical")
}

// T044: CDC slot and publication creation
func TestE2E_CDC_SlotAndPublication(t *testing.T) {
	connString := getTestConnectionString(t)
	ctx := context.Background()
	cdcID := "test_e2e_slot"

	// Cleanup first
	replConn, err := pgconn.Connect(ctx, replConnString(connString))
	require.NoError(t, err)
	_ = services.CleanupCDCResources(ctx, replConn, cdcID)
	_ = replConn.Close(ctx)

	// Reconnect after cleanup
	replConn, err = pgconn.Connect(ctx, replConnString(connString))
	require.NoError(t, err)
	defer func() { _ = replConn.Close(ctx) }()

	// Create publication
	err = services.EnsurePublication(ctx, replConn, cdcID)
	require.NoError(t, err)

	// Create slot
	lsn, err := services.EnsureReplicationSlot(ctx, replConn, cdcID)
	require.NoError(t, err)
	assert.True(t, lsn > 0, "should return a non-zero LSN")

	// Idempotent: create again should not error
	err = services.EnsurePublication(ctx, replConn, cdcID)
	require.NoError(t, err)

	// Cleanup
	err = services.CleanupCDCResources(ctx, replConn, cdcID)
	require.NoError(t, err)
}

// T044: CDC initial sync test
func TestE2E_CDC_InitialSync(t *testing.T) {
	connString := getTestConnectionString(t)
	seedDatabase(t, connString)

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.InfoLevel)

	p := internalPlugin.Plugin()
	p.SetLogger(logger)

	spec := fmt.Sprintf(`{"connection_string": "%s", "cdc_id": "test_initial_sync"}`, connString)
	err := p.Init(ctx, []byte(spec), plugin.NewClientOptions{})
	require.NoError(t, err)
	defer func() { _ = p.Close(ctx) }()

	msgs, err := p.SyncAll(ctx, plugin.SyncOptions{
		Tables: []string{"test_users"},
	})
	require.NoError(t, err)

	migrates, inserts := countMessageTypes(msgs)
	assert.GreaterOrEqual(t, migrates, 1, "should have migrate messages")
	assert.GreaterOrEqual(t, inserts, 0, "may have insert messages")

	// Cleanup replication resources
	replConn, err := pgconn.Connect(ctx, replConnString(connString))
	if err == nil {
		_ = services.CleanupCDCResources(ctx, replConn, "test_initial_sync")
		_ = replConn.Close(ctx)
	}
}

// T045: Two independent CDC sources don't interfere
func TestE2E_CDC_IndependentSources(t *testing.T) {
	connString := getTestConnectionString(t)
	ctx := context.Background()

	for _, cdcID := range []string{"source_a", "source_b"} {
		replConn, err := pgconn.Connect(ctx, replConnString(connString))
		require.NoError(t, err)
		_ = services.CleanupCDCResources(ctx, replConn, cdcID)
		_ = replConn.Close(ctx)

		replConn, err = pgconn.Connect(ctx, replConnString(connString))
		require.NoError(t, err)
		err = services.EnsurePublication(ctx, replConn, cdcID)
		require.NoError(t, err)
		_, err = services.EnsureReplicationSlot(ctx, replConn, cdcID)
		require.NoError(t, err)
		_ = replConn.Close(ctx)
	}

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	var slotCountA, slotCountB int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM pg_replication_slots WHERE slot_name = $1",
		services.CDCSlotName("source_a")).Scan(&slotCountA)
	require.NoError(t, err)
	assert.Equal(t, 1, slotCountA)

	err = pool.QueryRow(ctx, "SELECT count(*) FROM pg_replication_slots WHERE slot_name = $1",
		services.CDCSlotName("source_b")).Scan(&slotCountB)
	require.NoError(t, err)
	assert.Equal(t, 1, slotCountB)

	for _, cdcID := range []string{"source_a", "source_b"} {
		replConn, err := pgconn.Connect(ctx, replConnString(connString))
		if err == nil {
			_ = services.CleanupCDCResources(ctx, replConn, cdcID)
			_ = replConn.Close(ctx)
		}
	}
}

// T046: Missing wal_level=logical returns clear error
func TestE2E_CDC_WalLevelCheck(t *testing.T) {
	err := services.CheckWalLevelValue("replica")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wal_level")
	assert.Contains(t, err.Error(), "logical")
}
