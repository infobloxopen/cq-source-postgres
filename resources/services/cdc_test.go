package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T037: CDC slot naming
func TestCDC_SlotName(t *testing.T) {
	assert.Equal(t, "cq_cdc_my_source", CDCSlotName("my_source"))
	assert.Equal(t, "cq_cdc_short", CDCSlotName("short"))
}

// T037: CDC publication naming
func TestCDC_PublicationName(t *testing.T) {
	assert.Equal(t, "cq_pub_my_source", CDCPublicationName("my_source"))
}

// T037: WAL event decoding — relation message caching
func TestCDC_RelationCache(t *testing.T) {
	rc := NewRelationCache()
	require.NotNil(t, rc)

	rc.Set(123, RelationInfo{
		Schema:  "public",
		Table:   "users",
		Columns: []string{"id", "name", "email"},
	})

	info, ok := rc.Get(123)
	require.True(t, ok)
	assert.Equal(t, "users", info.Table)
	assert.Equal(t, "public", info.Schema)
	assert.Equal(t, []string{"id", "name", "email"}, info.Columns)

	_, ok = rc.Get(999)
	assert.False(t, ok)
}

// T037: Decode text tuple data
func TestCDC_DecodeTupleValues(t *testing.T) {
	columns := []string{"id", "name", "active"}

	// Simulate tuple data columns
	tupleColumns := []TupleColumnData{
		{Type: 't', Data: []byte("42")},
		{Type: 't', Data: []byte("Alice")},
		{Type: 'n', Data: nil}, // NULL
	}

	values := DecodeTupleData(columns, tupleColumns)
	assert.Equal(t, "42", values["id"])
	assert.Equal(t, "Alice", values["name"])
	assert.Nil(t, values["active"])
}

// T037: Decode unchanged TOAST columns
func TestCDC_DecodeTupleToast(t *testing.T) {
	columns := []string{"id", "big_text"}
	tupleColumns := []TupleColumnData{
		{Type: 't', Data: []byte("1")},
		{Type: 'u', Data: nil}, // unchanged TOAST
	}

	values := DecodeTupleData(columns, tupleColumns)
	assert.Equal(t, "1", values["id"])
	// TOAST columns should not be present
	_, exists := values["big_text"]
	assert.False(t, exists)
}

// T038: CDC prerequisites check logic
func TestCDC_CheckWalLevel(t *testing.T) {
	// This tests the error message when wal_level != logical
	err := validateWalLevel("replica")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wal_level")
	assert.Contains(t, err.Error(), "logical")

	err = validateWalLevel("logical")
	require.NoError(t, err)
}
