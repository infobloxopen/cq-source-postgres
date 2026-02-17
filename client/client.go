package client

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

// Client holds the runtime state of the plugin.
type Client struct {
	logger zerolog.Logger
	spec   Spec
	pool   *pgxpool.Pool
}

// Spec returns the client's spec configuration.
func (c *Client) Spec() Spec {
	return c.spec
}

// Pool returns the underlying pgxpool connection pool.
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// ID implements schema.ClientMeta.
func (c *Client) ID() string {
	return "cq-source-postgres"
}

// New creates a new Client from a validated Spec.
// Supports both URL format (postgres://...) and DSN format (key=value pairs).
func New(ctx context.Context, logger zerolog.Logger, spec Spec) (*Client, error) {
	poolConfig, err := pgxpool.ParseConfig(spec.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Wire pgx log adapter for database driver tracing.
	adapter := NewPgxLogAdapter(logger, spec.PgxLogLevel)
	poolConfig.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   adapter,
		LogLevel: PgxLogLevel(spec.PgxLogLevel),
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return &Client{
		logger: logger,
		spec:   spec,
		pool:   pool,
	}, nil
}

// Close closes the database connection pool.
func (c *Client) Close(_ context.Context) error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}
