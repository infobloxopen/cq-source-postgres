package client

import (
	"fmt"
	"strings"
)

// Spec is the plugin configuration parsed from the CloudQuery YAML spec block.
type Spec struct {
	ConnectionString     string `json:"connection_string"`
	PgxLogLevel          string `json:"pgx_log_level,omitempty"`
	CdcID                string `json:"cdc_id,omitempty"`
	RowsPerRecord        int    `json:"rows_per_record,omitempty"`
	DestinationTableName string `json:"destination_table_name,omitempty"`
}

var validPgxLogLevels = []string{"error", "warn", "info", "debug", "trace"}

var dynamicPlaceholders = []string{"{{YEAR}}", "{{MONTH}}", "{{DAY}}", "{{HOUR}}", "{{MINUTE}}"}

// SetDefaults applies default values to optional fields that are not set.
func (s *Spec) SetDefaults() {
	if s.PgxLogLevel == "" {
		s.PgxLogLevel = "error"
	}
	if s.RowsPerRecord == 0 {
		s.RowsPerRecord = 500
	}
	if s.DestinationTableName == "" {
		s.DestinationTableName = "{{TABLE}}"
	}
}

// Validate returns an error if the spec contains invalid configuration.
func (s *Spec) Validate() error {
	if s.ConnectionString == "" {
		return fmt.Errorf("\"connection_string\" is required and must not be empty")
	}

	if !isValidPgxLogLevel(s.PgxLogLevel) {
		return fmt.Errorf("\"pgx_log_level\" is %q but must be one of: %s",
			s.PgxLogLevel, strings.Join(validPgxLogLevels, ", "))
	}

	if s.RowsPerRecord < 1 {
		return fmt.Errorf("\"rows_per_record\" is %d but must be >= 1", s.RowsPerRecord)
	}

	if !strings.Contains(s.DestinationTableName, "{{TABLE}}") &&
		!strings.Contains(s.DestinationTableName, "{{UUID}}") {
		return fmt.Errorf("\"destination_table_name\" must contain at least one of {{TABLE}} or {{UUID}}")
	}

	if s.CdcID != "" {
		for _, p := range dynamicPlaceholders {
			if strings.Contains(s.DestinationTableName, p) {
				return fmt.Errorf("\"destination_table_name\" must not contain dynamic time-based placeholders (%s) when cdc_id is set",
					strings.Join(dynamicPlaceholders, ", "))
			}
		}
	}

	return nil
}

func isValidPgxLogLevel(level string) bool {
	for _, valid := range validPgxLogLevels {
		if level == valid {
			return true
		}
	}
	return false
}
