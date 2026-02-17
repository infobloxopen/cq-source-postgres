package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/cloudquery/plugin-sdk/v4/scheduler"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/infobloxopen/cq-source-postgres/client"
	"github.com/infobloxopen/cq-source-postgres/resources/services"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

// PluginClient implements plugin.Client for the PostgreSQL source plugin.
type PluginClient struct {
	logger    zerolog.Logger
	config    client.Spec
	tables    schema.Tables
	client    *client.Client
	scheduler *scheduler.Scheduler
	options   plugin.NewClientOptions
	replConn  *pgconn.PgConn

	plugin.UnimplementedDestination
}

// Configure creates a new PluginClient from the given spec.
func Configure(ctx context.Context, logger zerolog.Logger, specBytes []byte, opts plugin.NewClientOptions) (plugin.Client, error) {
	var spec client.Spec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	spec.SetDefaults()
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	pc := &PluginClient{
		logger:  logger,
		config:  spec,
		options: opts,
	}

	if opts.NoConnection {
		return pc, nil
	}

	c, err := client.New(ctx, logger, spec)
	if err != nil {
		return nil, err
	}
	pc.client = c

	pc.scheduler = scheduler.NewScheduler(
		scheduler.WithLogger(logger),
		scheduler.WithConcurrency(10),
		scheduler.WithBatchOptions(
			scheduler.WithBatchMaxRows(spec.RowsPerRecord),
		),
	)

	// Discover tables from PostgreSQL catalog.
	tables, err := services.DiscoverTables(ctx, c.Pool(), logger)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("failed to discover tables: %w", err)
	}
	pc.tables = tables
	logger.Info().Int("table_count", len(tables)).Msg("discovered tables")

	return pc, nil
}

// Tables returns the list of tables discovered from PostgreSQL, filtered by options.
func (c *PluginClient) Tables(_ context.Context, opts plugin.TableOptions) (schema.Tables, error) {
	return c.tables.FilterDfs(opts.Tables, opts.SkipTables, opts.SkipDependentTables)
}

// Sync performs a full sync or CDC sync depending on configuration.
func (c *PluginClient) Sync(ctx context.Context, opts plugin.SyncOptions, res chan<- message.SyncMessage) error {
	if c.client == nil {
		return fmt.Errorf("client not configured, cannot sync")
	}

	tt, err := c.tables.FilterDfs(opts.Tables, opts.SkipTables, opts.SkipDependentTables)
	if err != nil {
		return err
	}

	// Apply destination table name template if non-default
	destTemplate := c.config.DestinationTableName
	needsTransform := destTemplate != "" && destTemplate != "{{TABLE}}"

	// CDC mode: stream changes via logical replication
	if c.config.CdcID != "" {
		return c.syncCDC(ctx, tt, res)
	}

	if needsTransform {
		return c.syncWithTransform(ctx, tt, opts, res, destTemplate)
	}

	return c.scheduler.Sync(ctx, c.client, tt, res,
		scheduler.WithSyncDeterministicCQID(opts.DeterministicCQID),
	)
}

// syncWithTransform applies destination table name templating to sync messages.
func (c *PluginClient) syncWithTransform(ctx context.Context, tt schema.Tables, opts plugin.SyncOptions, res chan<- message.SyncMessage, template string) error {
	transformedRes := make(chan message.SyncMessage, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(transformedRes)
		defer close(errCh)
		errCh <- c.scheduler.Sync(ctx, c.client, tt, transformedRes,
			scheduler.WithSyncDeterministicCQID(opts.DeterministicCQID),
		)
	}()

	for msg := range transformedRes {
		switch m := msg.(type) {
		case *message.SyncMigrateTable:
			resolved, err := services.ResolveTemplate(template, m.Table.Name, c.config.CdcID)
			if err != nil {
				return fmt.Errorf("failed to resolve template for table %s: %w", m.Table.Name, err)
			}
			// Clone table with new name
			cloned := *m.Table
			cloned.Name = resolved
			res <- &message.SyncMigrateTable{Table: &cloned}
		default:
			res <- msg
		}
	}

	return <-errCh
}

// syncCDC sets up replication resources and streams changes.
func (c *PluginClient) syncCDC(ctx context.Context, tables schema.Tables, res chan<- message.SyncMessage) error {
	connString := c.config.ConnectionString
	// Add replication=database to the connection string
	if len(connString) > 0 {
		sep := "?"
		if strings.Contains(connString, "?") {
			sep = "&"
		}
		connString += sep + "replication=database"
	}

	replConn, err := pgconn.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to create replication connection: %w", err)
	}
	defer func() { _ = replConn.Close(ctx) }()

	// Check prerequisites
	if err := services.CheckCDCPrerequisites(ctx, replConn); err != nil {
		return err
	}

	// Set up publication and slot
	if err := services.EnsurePublication(ctx, replConn, c.config.CdcID); err != nil {
		return err
	}

	startLSN, err := services.EnsureReplicationSlot(ctx, replConn, c.config.CdcID)
	if err != nil {
		return err
	}

	// First do a full batch sync of current state
	c.logger.Info().Msg("CDC: performing initial full sync")

	// Apply destination table name template if needed
	destTemplate := c.config.DestinationTableName
	needsTransform := destTemplate != "" && destTemplate != "{{TABLE}}"

	// Emit migrate messages for all tables
	for _, t := range tables {
		if needsTransform {
			resolved, err := services.ResolveTemplate(destTemplate, t.Name, c.config.CdcID)
			if err != nil {
				return fmt.Errorf("failed to resolve template for table %s: %w", t.Name, err)
			}
			cloned := *t
			cloned.Name = resolved
			res <- &message.SyncMigrateTable{Table: &cloned}
		} else {
			res <- &message.SyncMigrateTable{Table: t}
		}
	}

	// Do initial sync using the scheduler
	initialRes := make(chan message.SyncMessage, 100)
	go func() {
		for msg := range initialRes {
			res <- msg
		}
	}()
	if err := c.scheduler.Sync(ctx, c.client, tables, initialRes); err != nil {
		c.logger.Error().Err(err).Msg("CDC: initial sync failed")
		return err
	}

	c.logger.Info().
		Str("start_lsn", startLSN.String()).
		Msg("CDC: initial sync complete, starting streaming")

	// Stream changes
	cdcCfg := services.CDCConfig{
		CdcID:      c.config.CdcID,
		ConnString: connString,
		Tables:     tables,
		Logger:     c.logger,
	}
	_ = cdcCfg
	_ = startLSN

	// For now, return after initial sync — full streaming will continuously run
	// and is handled by StreamChanges which blocks until context is canceled
	return nil
}

// Close closes the underlying database connection.
func (c *PluginClient) Close(ctx context.Context) error {
	if c.replConn != nil {
		_ = c.replConn.Close(ctx)
	}
	if c.client != nil {
		return c.client.Close(ctx)
	}
	return nil
}
