package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T047: Template resolution tests
func TestResolveTemplate_TableOnly(t *testing.T) {
	result, err := ResolveTemplate("{{TABLE}}", "users", "")
	require.NoError(t, err)
	assert.Equal(t, "users", result)
}

func TestResolveTemplate_PrefixedTable(t *testing.T) {
	result, err := ResolveTemplate("raw_{{TABLE}}", "users", "")
	require.NoError(t, err)
	assert.Equal(t, "raw_users", result)
}

func TestResolveTemplate_UUID(t *testing.T) {
	result, err := ResolveTemplate("{{UUID}}", "users", "")
	require.NoError(t, err)
	// UUID should be a valid UUID format
	assert.Len(t, result, 36, "UUID should be 36 characters")
	assert.Equal(t, 4, strings.Count(result, "-"), "UUID should have 4 dashes")
}

func TestResolveTemplate_TableWithDateParts(t *testing.T) {
	result, err := ResolveTemplate("{{TABLE}}_{{YEAR}}_{{MONTH}}", "users", "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "users_"), "should start with table name")
	// Should have year and month components
	parts := strings.Split(result, "_")
	assert.GreaterOrEqual(t, len(parts), 3)
}

func TestResolveTemplate_AllTimePlaceholders(t *testing.T) {
	template := "{{TABLE}}_{{YEAR}}_{{MONTH}}_{{DAY}}_{{HOUR}}_{{MINUTE}}"
	result, err := ResolveTemplate(template, "events", "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "events_"))
	parts := strings.Split(result, "_")
	assert.Equal(t, 6, len(parts))
}

func TestResolveTemplate_NoPlaceholders(t *testing.T) {
	result, err := ResolveTemplate("static_name", "users", "")
	require.NoError(t, err)
	assert.Equal(t, "static_name", result)
}

func TestResolveTemplate_MixedTableAndUUID(t *testing.T) {
	result, err := ResolveTemplate("{{TABLE}}_{{UUID}}", "orders", "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "orders_"))
	// After the underscore, should be a UUID
	uuidPart := result[len("orders_"):]
	assert.Len(t, uuidPart, 36)
}

func TestResolveTemplate_EmptyTableName(t *testing.T) {
	result, err := ResolveTemplate("{{TABLE}}", "", "")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
