package client

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/types"
	"github.com/stretchr/testify/assert"
)

func TestPgTypeToArrow(t *testing.T) {
	tests := []struct {
		udtName  string
		expected arrow.DataType
	}{
		// Integer types
		{"int2", arrow.PrimitiveTypes.Int16},
		{"smallint", arrow.PrimitiveTypes.Int16},
		{"int4", arrow.PrimitiveTypes.Int32},
		{"integer", arrow.PrimitiveTypes.Int32},
		{"serial", arrow.PrimitiveTypes.Int32},
		{"int8", arrow.PrimitiveTypes.Int64},
		{"bigint", arrow.PrimitiveTypes.Int64},
		{"bigserial", arrow.PrimitiveTypes.Int64},

		// Float types
		{"float4", arrow.PrimitiveTypes.Float32},
		{"real", arrow.PrimitiveTypes.Float32},
		{"float8", arrow.PrimitiveTypes.Float64},
		{"double precision", arrow.PrimitiveTypes.Float64},

		// Numeric
		{"numeric", arrow.BinaryTypes.String},
		{"decimal", arrow.BinaryTypes.String},

		// Boolean
		{"bool", arrow.FixedWidthTypes.Boolean},
		{"boolean", arrow.FixedWidthTypes.Boolean},

		// Text types
		{"text", arrow.BinaryTypes.String},
		{"varchar", arrow.BinaryTypes.String},
		{"char", arrow.BinaryTypes.String},
		{"bpchar", arrow.BinaryTypes.String},
		{"name", arrow.BinaryTypes.String},
		{"character varying", arrow.BinaryTypes.String},
		{"character", arrow.BinaryTypes.String},

		// Binary
		{"bytea", arrow.BinaryTypes.Binary},

		// Timestamps
		{"timestamp", arrow.FixedWidthTypes.Timestamp_us},
		{"timestamp without time zone", arrow.FixedWidthTypes.Timestamp_us},
		{"timestamptz", arrow.FixedWidthTypes.Timestamp_us},
		{"timestamp with time zone", arrow.FixedWidthTypes.Timestamp_us},

		// Date
		{"date", arrow.FixedWidthTypes.Date32},

		// Time (serialized as string)
		{"time", arrow.BinaryTypes.String},
		{"time without time zone", arrow.BinaryTypes.String},
		{"timetz", arrow.BinaryTypes.String},
		{"time with time zone", arrow.BinaryTypes.String},

		// Interval
		{"interval", arrow.BinaryTypes.String},

		// UUID
		{"uuid", types.ExtensionTypes.UUID},

		// JSON
		{"json", types.ExtensionTypes.JSON},
		{"jsonb", types.ExtensionTypes.JSON},

		// Network
		{"inet", types.ExtensionTypes.Inet},
		{"cidr", types.ExtensionTypes.Inet},
		{"macaddr", types.ExtensionTypes.MAC},
		{"macaddr8", types.ExtensionTypes.MAC},

		// XML
		{"xml", arrow.BinaryTypes.String},

		// Geometry
		{"point", arrow.BinaryTypes.String},
		{"line", arrow.BinaryTypes.String},
		{"lseg", arrow.BinaryTypes.String},
		{"box", arrow.BinaryTypes.String},
		{"path", arrow.BinaryTypes.String},
		{"polygon", arrow.BinaryTypes.String},
		{"circle", arrow.BinaryTypes.String},

		// Hstore
		{"hstore", types.ExtensionTypes.JSON},

		// OID
		{"oid", arrow.PrimitiveTypes.Uint32},

		// Unknown
		{"some_custom_type", arrow.BinaryTypes.String},
	}

	for _, tt := range tests {
		t.Run(tt.udtName, func(t *testing.T) {
			got := PgTypeToArrow(tt.udtName)
			assert.Equal(t, tt.expected, got, "PgTypeToArrow(%q)", tt.udtName)
		})
	}
}

func TestPgTypeToArrow_Arrays(t *testing.T) {
	t.Run("text array", func(t *testing.T) {
		got := PgTypeToArrow("_text")
		expected := arrow.ListOf(arrow.BinaryTypes.String)
		assert.Equal(t, expected, got)
	})

	t.Run("int4 array", func(t *testing.T) {
		got := PgTypeToArrow("_int4")
		expected := arrow.ListOf(arrow.PrimitiveTypes.Int32)
		assert.Equal(t, expected, got)
	})

	t.Run("uuid array", func(t *testing.T) {
		got := PgTypeToArrow("_uuid")
		expected := arrow.ListOf(types.ExtensionTypes.UUID)
		assert.Equal(t, expected, got)
	})

	t.Run("bool array", func(t *testing.T) {
		got := PgTypeToArrow("_bool")
		expected := arrow.ListOf(arrow.FixedWidthTypes.Boolean)
		assert.Equal(t, expected, got)
	})
}
