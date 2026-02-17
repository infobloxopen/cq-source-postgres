package client

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ID(t *testing.T) {
	c := &Client{}
	assert.Equal(t, "cq-source-postgres", c.ID())
}

const testConnEnv = "CQ_SOURCE_PG_TEST_CONN"

func getTestConnString(t *testing.T) string {
	t.Helper()
	conn := os.Getenv(testConnEnv)
	if conn == "" {
		t.Skipf("%s not set, skipping", testConnEnv)
	}
	return conn
}

// T027: URL format connection
func TestClient_New_URLFormat(t *testing.T) {
	connString := getTestConnString(t)
	logger := zerolog.New(zerolog.NewTestWriter(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	spec := Spec{ConnectionString: connString}
	spec.SetDefaults()

	c, err := New(ctx, logger, spec)
	require.NoError(t, err)
	defer func() { _ = c.Close(ctx) }()

	assert.NotNil(t, c.Pool())
	assert.Equal(t, "cq-source-postgres", c.ID())
}

// T027: DSN format connection (key=value pairs)
func TestClient_New_DSNFormat(t *testing.T) {
	_ = getTestConnString(t) // skip if no test DB

	logger := zerolog.New(zerolog.NewTestWriter(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := "host=localhost port=5432 user=cq password=cq dbname=cq_test sslmode=disable"
	spec := Spec{ConnectionString: dsn}
	spec.SetDefaults()

	c, err := New(ctx, logger, spec)
	require.NoError(t, err)
	defer func() { _ = c.Close(ctx) }()

	assert.NotNil(t, c.Pool())
}

// T028: Invalid password returns auth error
func TestClient_New_InvalidPassword(t *testing.T) {
	_ = getTestConnString(t) // skip if no test DB

	logger := zerolog.New(zerolog.NewTestWriter(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	spec := Spec{ConnectionString: "postgres://cq:wrongpassword@localhost:5432/cq_test?sslmode=disable"}
	spec.SetDefaults()

	_, err := New(ctx, logger, spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

// T028: Invalid connection string returns parse error
func TestClient_New_InvalidConnString(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	ctx := context.Background()

	spec := Spec{ConnectionString: "not-a-valid-connection-string"}
	spec.SetDefaults()

	_, err := New(ctx, logger, spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse connection string")
}

// T028: Unreachable host returns connection error
func TestClient_New_UnreachableHost(t *testing.T) {
	_ = getTestConnString(t) // skip if no test DB

	logger := zerolog.New(zerolog.NewTestWriter(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a non-routable IP address to force a timeout
	spec := Spec{ConnectionString: "postgres://cq:cq@192.0.2.1:5432/cq_test?sslmode=disable&connect_timeout=2"}
	spec.SetDefaults()

	_, err := New(ctx, logger, spec)
	require.Error(t, err)
}

// T027: Spec getter returns the configured spec
func TestClient_Spec(t *testing.T) {
	c := &Client{
		spec: Spec{ConnectionString: "test", RowsPerRecord: 100},
	}
	assert.Equal(t, "test", c.Spec().ConnectionString)
	assert.Equal(t, 100, c.Spec().RowsPerRecord)
}

// T027: Close on nil pool does not panic
func TestClient_Close_NilPool(t *testing.T) {
	c := &Client{}
	err := c.Close(context.Background())
	assert.NoError(t, err)
}

// T030: pgx log level adapter
func TestPgxLogLevelAdapter(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"error", "error"},
		{"warn", "warn"},
		{"info", "info"},
		{"debug", "debug"},
		{"trace", "trace"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			adapter := NewPgxLogAdapter(zerolog.New(zerolog.NewTestWriter(t)), tt.level)
			assert.NotNil(t, adapter)
		})
	}
}
