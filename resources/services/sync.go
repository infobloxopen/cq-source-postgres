package services

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MakeResolver creates a TableResolver that reads all rows from the specified PostgreSQL table.
func MakeResolver(pool *pgxpool.Pool, schemaName, tableName string) schema.TableResolver {
	return func(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
		if pool == nil {
			return fmt.Errorf("database pool is nil")
		}
		qualifiedName := fmt.Sprintf("%q.%q", schemaName, tableName)
		query := fmt.Sprintf("SELECT * FROM %s", qualifiedName)

		rows, err := pool.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to query %s: %w", qualifiedName, err)
		}
		defer rows.Close()

		fieldDescs := rows.FieldDescriptions()
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return fmt.Errorf("failed to read row from %s: %w", qualifiedName, err)
			}
			row := make(map[string]any, len(fieldDescs))
			for i, fd := range fieldDescs {
				row[string(fd.Name)] = convertPgValue(values[i])
			}
			res <- row
		}

		return rows.Err()
	}
}

// convertPgValue converts pgx-specific Go types to types the CloudQuery SDK can handle.
// The SDK Resource.Set() panics when it receives pgx struct types (like pgtype.Numeric)
// for columns mapped to Arrow string type. This function converts those to strings.
func convertPgValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case pgtype.Numeric:
		if !v.Valid {
			return nil
		}
		f, err := v.Float64Value()
		if err == nil && f.Valid {
			return fmt.Sprintf("%g", f.Float64)
		}
		// Fallback for very large numbers
		if v.Int != nil {
			return v.Int.String()
		}
		return "0"

	case pgtype.Time:
		if !v.Valid {
			return nil
		}
		us := v.Microseconds
		hours := us / 3600000000
		us -= hours * 3600000000
		minutes := us / 60000000
		us -= minutes * 60000000
		seconds := us / 1000000
		us -= seconds * 1000000
		if us > 0 {
			return fmt.Sprintf("%02d:%02d:%02d.%06d", hours, minutes, seconds, us)
		}
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	case pgtype.Interval:
		if !v.Valid {
			return nil
		}
		var parts []string
		if v.Months != 0 {
			years := v.Months / 12
			months := v.Months % 12
			if years != 0 {
				parts = append(parts, fmt.Sprintf("%d year", years))
			}
			if months != 0 {
				parts = append(parts, fmt.Sprintf("%d mon", months))
			}
		}
		if v.Days != 0 {
			parts = append(parts, fmt.Sprintf("%d day", v.Days))
		}
		if v.Microseconds != 0 {
			us := v.Microseconds
			hours := us / 3600000000
			us -= hours * 3600000000
			minutes := us / 60000000
			us -= minutes * 60000000
			seconds := us / 1000000
			parts = append(parts, fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds))
		}
		if len(parts) == 0 {
			return "00:00:00"
		}
		result := ""
		for i, p := range parts {
			if i > 0 {
				result += " "
			}
			result += p
		}
		return result

	case pgtype.Point:
		if !v.Valid {
			return nil
		}
		return fmt.Sprintf("(%g,%g)", v.P.X, v.P.Y)

	case pgtype.Line:
		if !v.Valid {
			return nil
		}
		return fmt.Sprintf("{%g,%g,%g}", v.A, v.B, v.C)

	case pgtype.Lseg:
		if !v.Valid {
			return nil
		}
		return fmt.Sprintf("[(%g,%g),(%g,%g)]", v.P[0].X, v.P[0].Y, v.P[1].X, v.P[1].Y)

	case pgtype.Box:
		if !v.Valid {
			return nil
		}
		return fmt.Sprintf("(%g,%g),(%g,%g)", v.P[0].X, v.P[0].Y, v.P[1].X, v.P[1].Y)

	case pgtype.Circle:
		if !v.Valid {
			return nil
		}
		return fmt.Sprintf("<(%g,%g),%g>", v.P.X, v.P.Y, v.R)

	case pgtype.Path:
		if !v.Valid {
			return nil
		}
		result := ""
		for i, p := range v.P {
			if i > 0 {
				result += ","
			}
			result += fmt.Sprintf("(%g,%g)", p.X, p.Y)
		}
		if v.Closed {
			return "(" + result + ")"
		}
		return "[" + result + "]"

	case pgtype.Polygon:
		if !v.Valid {
			return nil
		}
		result := ""
		for i, p := range v.P {
			if i > 0 {
				result += ","
			}
			result += fmt.Sprintf("(%g,%g)", p.X, p.Y)
		}
		return "(" + result + ")"

	case time.Time:
		// SDK's date32 type expects "YYYY-MM-DD" format, not the full timestamp
		// that pgx returns for date columns.
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
			return v.Format("2006-01-02")
		}
		return v

	case pgtype.Date:
		if !v.Valid {
			return nil
		}
		return v.Time.Format("2006-01-02")

	case fmt.Stringer:
		return v.String()

	default:
		return value
	}
}
