package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/rs/zerolog"
)

// CDCSlotName returns the replication slot name for a given cdc_id.
func CDCSlotName(cdcID string) string {
	return "cq_cdc_" + cdcID
}

// CDCPublicationName returns the publication name for a given cdc_id.
func CDCPublicationName(cdcID string) string {
	return "cq_pub_" + cdcID
}

// TupleColumnData represents a single column value from a WAL tuple.
type TupleColumnData struct {
	Type byte   // 'n' = NULL, 'u' = unchanged TOAST, 't' = text, 'b' = binary
	Data []byte // raw value
}

// RelationInfo caches metadata about a relation received via RelationMessage.
type RelationInfo struct {
	Schema  string
	Table   string
	Columns []string
}

// RelationCache stores relation metadata keyed by RelationID.
type RelationCache struct {
	mu    sync.RWMutex
	cache map[uint32]RelationInfo
}

// NewRelationCache creates an empty RelationCache.
func NewRelationCache() *RelationCache {
	return &RelationCache{cache: make(map[uint32]RelationInfo)}
}

// Set stores relation info for a given relation ID.
func (rc *RelationCache) Set(id uint32, info RelationInfo) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache[id] = info
}

// Get retrieves relation info for a given relation ID.
func (rc *RelationCache) Get(id uint32) (RelationInfo, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	info, ok := rc.cache[id]
	return info, ok
}

// DecodeTupleData converts tuple column data into a map of column name → value.
// NULL columns produce nil. Unchanged TOAST columns are omitted.
func DecodeTupleData(columns []string, tupleColumns []TupleColumnData) map[string]any {
	values := make(map[string]any, len(columns))
	for i, col := range tupleColumns {
		if i >= len(columns) {
			break
		}
		switch col.Type {
		case 'n': // NULL
			values[columns[i]] = nil
		case 'u': // unchanged TOAST, skip
			continue
		case 't', 'b': // text or binary
			values[columns[i]] = string(col.Data)
		}
	}
	return values
}

// validateWalLevel checks that PostgreSQL is configured with wal_level=logical.
func validateWalLevel(walLevel string) error {
	if walLevel != "logical" {
		return fmt.Errorf("PostgreSQL wal_level is %q but must be \"logical\" for CDC; "+
			"set wal_level=logical in postgresql.conf and restart PostgreSQL", walLevel)
	}
	return nil
}

// CheckWalLevelValue is the exported version of validateWalLevel for testing.
func CheckWalLevelValue(walLevel string) error {
	return validateWalLevel(walLevel)
}

// CDCConfig holds the params needed to start a CDC sync.
type CDCConfig struct {
	CdcID      string
	ConnString string
	Tables     schema.Tables
	Logger     zerolog.Logger
}

// CheckCDCPrerequisites verifies that the database supports logical replication.
func CheckCDCPrerequisites(ctx context.Context, conn *pgconn.PgConn) error {
	result := conn.Exec(ctx, "SHOW wal_level")
	rows, err := result.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to check wal_level: %w", err)
	}
	if len(rows) == 0 || len(rows[0].Rows) == 0 {
		return fmt.Errorf("failed to read wal_level from server")
	}
	walLevel := string(rows[0].Rows[0][0])
	return validateWalLevel(walLevel)
}

// EnsurePublication creates the publication if it doesn't exist.
func EnsurePublication(ctx context.Context, conn *pgconn.PgConn, cdcID string) error {
	pubName := CDCPublicationName(cdcID)
	// Use IF NOT EXISTS to make it idempotent
	sql := fmt.Sprintf("CREATE PUBLICATION %q FOR ALL TABLES", pubName)
	result := conn.Exec(ctx, sql)
	_, err := result.ReadAll()
	if err != nil {
		// Check if it already exists (error code 42710 = duplicate_object)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42710" {
			return nil
		}
		return fmt.Errorf("failed to create publication %s: %w", pubName, err)
	}
	return nil
}

// EnsureReplicationSlot creates the replication slot if it doesn't exist.
func EnsureReplicationSlot(ctx context.Context, conn *pgconn.PgConn, cdcID string) (pglogrepl.LSN, error) {
	slotName := CDCSlotName(cdcID)

	// Try to create the slot; ignore if it already exists
	result, err := pglogrepl.CreateReplicationSlot(ctx, conn, slotName, "pgoutput",
		pglogrepl.CreateReplicationSlotOptions{})
	if err != nil {
		// 42710 = duplicate_object — slot already exists
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42710" {
			// Slot exists, we'll resume from where we left off
			return 0, nil
		}
		return 0, fmt.Errorf("failed to create replication slot %s: %w", slotName, err)
	}

	lsn, err := pglogrepl.ParseLSN(result.ConsistentPoint)
	if err != nil {
		return 0, fmt.Errorf("failed to parse consistent point LSN: %w", err)
	}
	return lsn, nil
}

// CleanupCDCResources drops the replication slot and publication.
func CleanupCDCResources(ctx context.Context, conn *pgconn.PgConn, cdcID string) error {
	slotName := CDCSlotName(cdcID)
	pubName := CDCPublicationName(cdcID)

	// Drop slot
	err := pglogrepl.DropReplicationSlot(ctx, conn, slotName,
		pglogrepl.DropReplicationSlotOptions{Wait: true})
	if err != nil {
		// Ignore if slot doesn't exist
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42704" {
			// Does not exist, that's fine
		} else {
			return fmt.Errorf("failed to drop replication slot %s: %w", slotName, err)
		}
	}

	// Drop publication
	sql := fmt.Sprintf("DROP PUBLICATION IF EXISTS %q", pubName)
	result := conn.Exec(ctx, sql)
	if _, err := result.ReadAll(); err != nil {
		return fmt.Errorf("failed to drop publication %s: %w", pubName, err)
	}

	return nil
}

// StreamChanges starts logical replication and emits change events via the res channel.
// It performs an initial sync of all tables first, then switches to streaming mode.
func StreamChanges(ctx context.Context, cfg CDCConfig, pool interface {
	Acquire(context.Context) (*interface{}, error)
}, replConn *pgconn.PgConn, res chan<- any) error {
	slotName := CDCSlotName(cfg.CdcID)
	pubName := CDCPublicationName(cfg.CdcID)

	// Get current LSN
	sysident, err := pglogrepl.IdentifySystem(ctx, replConn)
	if err != nil {
		return fmt.Errorf("failed to identify system: %w", err)
	}

	startLSN := sysident.XLogPos

	// Start replication
	pluginArgs := []string{
		"proto_version '1'",
		fmt.Sprintf("publication_names '%s'", pubName),
	}

	err = pglogrepl.StartReplication(ctx, replConn, slotName, startLSN,
		pglogrepl.StartReplicationOptions{PluginArgs: pluginArgs})
	if err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	cfg.Logger.Info().
		Str("slot", slotName).
		Str("publication", pubName).
		Str("start_lsn", startLSN.String()).
		Msg("CDC streaming started")

	relationCache := NewRelationCache()
	clientXLogPos := startLSN
	standbyMessageTimeout := 10 * time.Second
	nextStandbyDeadline := time.Now().Add(standbyMessageTimeout)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Send periodic standby status updates
		if time.Now().After(nextStandbyDeadline) {
			err = pglogrepl.SendStandbyStatusUpdate(ctx, replConn,
				pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos})
			if err != nil {
				return fmt.Errorf("failed to send standby status update: %w", err)
			}
			nextStandbyDeadline = time.Now().Add(standbyMessageTimeout)
		}

		// Receive with deadline
		recvCtx, cancel := context.WithDeadline(ctx, nextStandbyDeadline)
		rawMsg, err := replConn.ReceiveMessage(recvCtx)
		cancel()

		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			return fmt.Errorf("failed to receive replication message: %w", err)
		}

		copyData, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		switch copyData.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(copyData.Data[1:])
			if err != nil {
				cfg.Logger.Error().Err(err).Msg("failed to parse keepalive")
				continue
			}
			if pkm.ServerWALEnd > clientXLogPos {
				clientXLogPos = pkm.ServerWALEnd
			}
			if pkm.ReplyRequested {
				nextStandbyDeadline = time.Time{}
			}

		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(copyData.Data[1:])
			if err != nil {
				cfg.Logger.Error().Err(err).Msg("failed to parse xlog data")
				continue
			}

			logicalMsg, err := pglogrepl.Parse(xld.WALData)
			if err != nil {
				cfg.Logger.Error().Err(err).Msg("failed to parse logical message")
				continue
			}

			switch msg := logicalMsg.(type) {
			case *pglogrepl.RelationMessage:
				columns := make([]string, len(msg.Columns))
				for i, col := range msg.Columns {
					columns[i] = col.Name
				}
				relationCache.Set(msg.RelationID, RelationInfo{
					Schema:  msg.Namespace,
					Table:   msg.RelationName,
					Columns: columns,
				})

			case *pglogrepl.InsertMessage:
				rel, ok := relationCache.Get(msg.RelationID)
				if !ok {
					cfg.Logger.Warn().Uint32("relation_id", msg.RelationID).Msg("unknown relation for INSERT")
					continue
				}
				tupleColumns := make([]TupleColumnData, len(msg.Tuple.Columns))
				for i, col := range msg.Tuple.Columns {
					tupleColumns[i] = TupleColumnData{Type: col.DataType, Data: col.Data}
				}
				values := DecodeTupleData(rel.Columns, tupleColumns)
				values["_cq_cdc_action"] = "INSERT"
				values["_cq_cdc_table"] = rel.Table
				values["_cq_cdc_schema"] = rel.Schema
				res <- values

			case *pglogrepl.UpdateMessage:
				rel, ok := relationCache.Get(msg.RelationID)
				if !ok {
					cfg.Logger.Warn().Uint32("relation_id", msg.RelationID).Msg("unknown relation for UPDATE")
					continue
				}
				tupleColumns := make([]TupleColumnData, len(msg.NewTuple.Columns))
				for i, col := range msg.NewTuple.Columns {
					tupleColumns[i] = TupleColumnData{Type: col.DataType, Data: col.Data}
				}
				values := DecodeTupleData(rel.Columns, tupleColumns)
				values["_cq_cdc_action"] = "UPDATE"
				values["_cq_cdc_table"] = rel.Table
				values["_cq_cdc_schema"] = rel.Schema
				res <- values

			case *pglogrepl.DeleteMessage:
				rel, ok := relationCache.Get(msg.RelationID)
				if !ok {
					cfg.Logger.Warn().Uint32("relation_id", msg.RelationID).Msg("unknown relation for DELETE")
					continue
				}
				tupleColumns := make([]TupleColumnData, len(msg.OldTuple.Columns))
				for i, col := range msg.OldTuple.Columns {
					tupleColumns[i] = TupleColumnData{Type: col.DataType, Data: col.Data}
				}
				values := DecodeTupleData(rel.Columns, tupleColumns)
				values["_cq_cdc_action"] = "DELETE"
				values["_cq_cdc_table"] = rel.Table
				values["_cq_cdc_schema"] = rel.Schema
				res <- values

			case *pglogrepl.CommitMessage:
				// Advance LSN on commit
				if msg.TransactionEndLSN > clientXLogPos {
					clientXLogPos = msg.TransactionEndLSN
				}
			}

			if xld.WALStart > clientXLogPos {
				clientXLogPos = xld.WALStart
			}
		}
	}
}
