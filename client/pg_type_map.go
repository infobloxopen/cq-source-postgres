package client

import (
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/types"
)

// PgTypeToArrow maps a PostgreSQL type name to an Apache Arrow data type.
// The udtName is the underlying type name from pg_catalog (e.g., "int4", "varchar").
// Returns arrow.BinaryTypes.String as fallback for unknown types.
func PgTypeToArrow(udtName string) arrow.DataType {
	// Handle arrays: if the udt_name starts with "_", it's an array type.
	if strings.HasPrefix(udtName, "_") {
		elemType := PgTypeToArrow(strings.TrimPrefix(udtName, "_"))
		return arrow.ListOf(elemType)
	}

	switch udtName {
	// Integer types
	case "int2", "smallint":
		return arrow.PrimitiveTypes.Int16
	case "int4", "integer", "serial":
		return arrow.PrimitiveTypes.Int32
	case "int8", "bigint", "bigserial":
		return arrow.PrimitiveTypes.Int64

	// Floating-point types
	case "float4", "real":
		return arrow.PrimitiveTypes.Float32
	case "float8", "double precision":
		return arrow.PrimitiveTypes.Float64

	// Numeric / Decimal — preserve as string to avoid precision loss
	case "numeric", "decimal":
		return arrow.BinaryTypes.String

	// Boolean
	case "bool", "boolean":
		return arrow.FixedWidthTypes.Boolean

	// Text types
	case "text", "varchar", "char", "bpchar", "name", "character varying", "character":
		return arrow.BinaryTypes.String

	// Binary
	case "bytea":
		return arrow.BinaryTypes.Binary

	// Timestamp types
	case "timestamp", "timestamp without time zone":
		return arrow.FixedWidthTypes.Timestamp_us
	case "timestamptz", "timestamp with time zone":
		return arrow.FixedWidthTypes.Timestamp_us

	// Date
	case "date":
		return arrow.FixedWidthTypes.Date32

	// Time types — serialize as string
	case "time", "time without time zone":
		return arrow.BinaryTypes.String
	case "timetz", "time with time zone":
		return arrow.BinaryTypes.String

	// Interval — serialize as string
	case "interval":
		return arrow.BinaryTypes.String

	// UUID
	case "uuid":
		return types.ExtensionTypes.UUID

	// JSON types
	case "json", "jsonb":
		return types.ExtensionTypes.JSON

	// Network types
	case "inet", "cidr":
		return types.ExtensionTypes.Inet
	case "macaddr", "macaddr8":
		return types.ExtensionTypes.MAC

	// XML
	case "xml":
		return arrow.BinaryTypes.String

	// Geometry/geographic types — serialize as string
	case "point", "line", "lseg", "box", "path", "polygon", "circle":
		return arrow.BinaryTypes.String

	// Hstore — serialize as JSON
	case "hstore":
		return types.ExtensionTypes.JSON

	// OID
	case "oid":
		return arrow.PrimitiveTypes.Uint32

	default:
		// Unknown/composite types — fallback to string
		return arrow.BinaryTypes.String
	}
}
