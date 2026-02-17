package client

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpec_SetDefaults(t *testing.T) {
	t.Run("sets default pgx_log_level", func(t *testing.T) {
		s := Spec{ConnectionString: "postgres://localhost/db"}
		s.SetDefaults()
		assert.Equal(t, "error", s.PgxLogLevel)
	})

	t.Run("sets default rows_per_record", func(t *testing.T) {
		s := Spec{ConnectionString: "postgres://localhost/db"}
		s.SetDefaults()
		assert.Equal(t, 500, s.RowsPerRecord)
	})

	t.Run("sets default destination_table_name", func(t *testing.T) {
		s := Spec{ConnectionString: "postgres://localhost/db"}
		s.SetDefaults()
		assert.Equal(t, "{{TABLE}}", s.DestinationTableName)
	})

	t.Run("does not override user-set values", func(t *testing.T) {
		s := Spec{
			ConnectionString:     "postgres://localhost/db",
			PgxLogLevel:          "debug",
			RowsPerRecord:        100,
			DestinationTableName: "raw_{{TABLE}}",
		}
		s.SetDefaults()
		assert.Equal(t, "debug", s.PgxLogLevel)
		assert.Equal(t, 100, s.RowsPerRecord)
		assert.Equal(t, "raw_{{TABLE}}", s.DestinationTableName)
	})
}

func TestSpec_Validate(t *testing.T) {
	validSpec := func() Spec {
		s := Spec{ConnectionString: "postgres://cq:cq@localhost:5432/cq_test?sslmode=disable"}
		s.SetDefaults()
		return s
	}

	t.Run("valid spec passes", func(t *testing.T) {
		s := validSpec()
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("empty connection_string returns error", func(t *testing.T) {
		s := validSpec()
		s.ConnectionString = ""
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_string")
	})

	t.Run("invalid pgx_log_level returns error with valid options", func(t *testing.T) {
		s := validSpec()
		s.PgxLogLevel = "invalid_level"
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pgx_log_level")
		assert.Contains(t, err.Error(), "error")
		assert.Contains(t, err.Error(), "warn")
		assert.Contains(t, err.Error(), "info")
		assert.Contains(t, err.Error(), "debug")
		assert.Contains(t, err.Error(), "trace")
	})

	t.Run("valid pgx_log_levels accepted", func(t *testing.T) {
		for _, level := range []string{"error", "warn", "info", "debug", "trace"} {
			s := validSpec()
			s.PgxLogLevel = level
			err := s.Validate()
			require.NoError(t, err, "log level %q should be valid", level)
		}
	})

	t.Run("rows_per_record zero returns error", func(t *testing.T) {
		s := validSpec()
		s.RowsPerRecord = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rows_per_record")
	})

	t.Run("rows_per_record negative returns error", func(t *testing.T) {
		s := validSpec()
		s.RowsPerRecord = -1
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rows_per_record")
	})

	t.Run("rows_per_record 1 is valid", func(t *testing.T) {
		s := validSpec()
		s.RowsPerRecord = 1
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("destination_table_name without TABLE or UUID returns error", func(t *testing.T) {
		s := validSpec()
		s.DestinationTableName = "no_placeholder"
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "destination_table_name")
		assert.Contains(t, err.Error(), "{{TABLE}}")
		assert.Contains(t, err.Error(), "{{UUID}}")
	})

	t.Run("destination_table_name with TABLE is valid", func(t *testing.T) {
		s := validSpec()
		s.DestinationTableName = "raw_{{TABLE}}"
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("destination_table_name with UUID is valid", func(t *testing.T) {
		s := validSpec()
		s.DestinationTableName = "{{UUID}}"
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("dynamic placeholders forbidden when cdc_id is set", func(t *testing.T) {
		s := validSpec()
		s.CdcID = "my-source"
		s.DestinationTableName = "{{TABLE}}_{{YEAR}}"
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "destination_table_name")
		assert.Contains(t, err.Error(), "cdc")
	})

	t.Run("static destination_table_name with cdc_id is valid", func(t *testing.T) {
		s := validSpec()
		s.CdcID = "my-source"
		s.DestinationTableName = "{{TABLE}}"
		err := s.Validate()
		require.NoError(t, err)
	})

	// T032: Additional edge cases
	t.Run("large rows_per_record is valid", func(t *testing.T) {
		s := validSpec()
		s.RowsPerRecord = 100000
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("destination_table_name with both TABLE and UUID is valid", func(t *testing.T) {
		s := validSpec()
		s.DestinationTableName = "{{TABLE}}_{{UUID}}"
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("destination_table_name with all dynamic placeholders and no cdc_id is valid", func(t *testing.T) {
		s := validSpec()
		s.DestinationTableName = "{{TABLE}}_{{YEAR}}_{{MONTH}}_{{DAY}}_{{HOUR}}_{{MINUTE}}"
		err := s.Validate()
		require.NoError(t, err)
	})

	t.Run("each dynamic placeholder blocked with cdc_id", func(t *testing.T) {
		for _, ph := range []string{"{{YEAR}}", "{{MONTH}}", "{{DAY}}", "{{HOUR}}", "{{MINUTE}}"} {
			s := validSpec()
			s.CdcID = "source-1"
			s.DestinationTableName = "{{TABLE}}_" + ph
			err := s.Validate()
			require.Error(t, err, "placeholder %s should be blocked with cdc_id", ph)
		}
	})

	t.Run("error messages name the invalid field", func(t *testing.T) {
		cases := []struct {
			name  string
			spec  Spec
			field string
		}{
			{"missing conn", Spec{PgxLogLevel: "error", RowsPerRecord: 1, DestinationTableName: "{{TABLE}}"}, "connection_string"},
			{"bad log level", Spec{ConnectionString: "x", PgxLogLevel: "bad", RowsPerRecord: 1, DestinationTableName: "{{TABLE}}"}, "pgx_log_level"},
			{"bad rows", Spec{ConnectionString: "x", PgxLogLevel: "error", RowsPerRecord: 0, DestinationTableName: "{{TABLE}}"}, "rows_per_record"},
			{"bad dest", Spec{ConnectionString: "x", PgxLogLevel: "error", RowsPerRecord: 1, DestinationTableName: "plain"}, "destination_table_name"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.spec.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.field)
			})
		}
	})
}

// T033: JSON Schema matches contract
func TestSpec_JSONSchemaValidation(t *testing.T) {
	schema, err := os.ReadFile("../specs/001-postgres-source-plugin/contracts/plugin-spec.json")
	if err != nil {
		// Also try at resources/plugin/schema.json embedded copy
		schema, err = os.ReadFile("../resources/plugin/schema.json")
		require.NoError(t, err, "cannot load JSON schema contract")
	}

	// Verify the schema is valid JSON and contains expected fields
	var schemaMap map[string]any
	require.NoError(t, json.Unmarshal(schema, &schemaMap))

	// Verify required fields match our spec
	required, ok := schemaMap["required"].([]any)
	require.True(t, ok, "schema should have a 'required' array")
	assert.Contains(t, required, "connection_string")

	// Verify all spec properties are in the schema
	props, ok := schemaMap["properties"].(map[string]any)
	require.True(t, ok, "schema should have 'properties'")
	for _, field := range []string{"connection_string", "pgx_log_level", "cdc_id", "rows_per_record", "destination_table_name"} {
		assert.Contains(t, props, field, "schema should contain property %s", field)
	}

	// Verify pgx_log_level enum matches our valid levels
	pgxProp := props["pgx_log_level"].(map[string]any)
	enumVals := pgxProp["enum"].([]any)
	for _, level := range validPgxLogLevels {
		assert.Contains(t, enumVals, level)
	}
}
