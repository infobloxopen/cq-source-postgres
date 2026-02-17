#!/usr/bin/env python3
"""Generate sync.go with pgx type conversion."""
import os

content = '''package services

import (
\t"context"
\t"fmt"

\t"github.com/cloudquery/plugin-sdk/v4/schema"
\t"github.com/jackc/pgx/v5/pgtype"
\t"github.com/jackc/pgx/v5/pgxpool"
)

// MakeResolver creates a TableResolver that reads all rows from the specified PostgreSQL table.
func MakeResolver(pool *pgxpool.Pool, schemaName, tableName string) schema.TableResolver {
\treturn func(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
\t\tif pool == nil {
\t\t\treturn fmt.Errorf("database pool is nil")
\t\t}
\t\tqualifiedName := fmt.Sprintf("%q.%q", schemaName, tableName)
\t\tquery := fmt.Sprintf("SELECT * FROM %s", qualifiedName)

\t\trows, err := pool.Query(ctx, query)
\t\tif err != nil {
\t\t\treturn fmt.Errorf("failed to query %s: %w", qualifiedName, err)
\t\t}
\t\tdefer rows.Close()

\t\tfieldDescs := rows.FieldDescriptions()
\t\tfor rows.Next() {
\t\t\tvalues, err := rows.Values()
\t\t\tif err != nil {
\t\t\t\treturn fmt.Errorf("failed to read row from %s: %w", qualifiedName, err)
\t\t\t}
\t\t\trow := make(map[string]any, len(fieldDescs))
\t\t\tfor i, fd := range fieldDescs {
\t\t\t\trow[string(fd.Name)] = convertPgValue(values[i])
\t\t\t}
\t\t\tres <- row
\t\t}

\t\treturn rows.Err()
\t}
}

// convertPgValue converts pgx-specific Go types to types the CloudQuery SDK can handle.
// The SDK Resource.Set() panics when it receives pgx struct types (like pgtype.Numeric)
// for columns mapped to Arrow string type. This function converts those to strings.
func convertPgValue(value any) any {
\tif value == nil {
\t\treturn nil
\t}

\tswitch v := value.(type) {
\tcase pgtype.Numeric:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\tf, err := v.Float64Value()
\t\tif err == nil && f.Valid {
\t\t\treturn fmt.Sprintf("%g", f.Float64)
\t\t}
\t\t// Fallback for very large numbers
\t\tif v.Int != nil {
\t\t\treturn v.Int.String()
\t\t}
\t\treturn "0"

\tcase pgtype.Time:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\tus := v.Microseconds
\t\thours := us / 3600000000
\t\tus -= hours * 3600000000
\t\tminutes := us / 60000000
\t\tus -= minutes * 60000000
\t\tseconds := us / 1000000
\t\tus -= seconds * 1000000
\t\tif us > 0 {
\t\t\treturn fmt.Sprintf("%02d:%02d:%02d.%06d", hours, minutes, seconds, us)
\t\t}
\t\treturn fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

\tcase pgtype.Interval:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\tvar parts []string
\t\tif v.Months != 0 {
\t\t\tyears := v.Months / 12
\t\t\tmonths := v.Months % 12
\t\t\tif years != 0 {
\t\t\t\tparts = append(parts, fmt.Sprintf("%d year", years))
\t\t\t}
\t\t\tif months != 0 {
\t\t\t\tparts = append(parts, fmt.Sprintf("%d mon", months))
\t\t\t}
\t\t}
\t\tif v.Days != 0 {
\t\t\tparts = append(parts, fmt.Sprintf("%d day", v.Days))
\t\t}
\t\tif v.Microseconds != 0 {
\t\t\tus := v.Microseconds
\t\t\thours := us / 3600000000
\t\t\tus -= hours * 3600000000
\t\t\tminutes := us / 60000000
\t\t\tus -= minutes * 60000000
\t\t\tseconds := us / 1000000
\t\t\tparts = append(parts, fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds))
\t\t}
\t\tif len(parts) == 0 {
\t\t\treturn "00:00:00"
\t\t}
\t\tresult := ""
\t\tfor i, p := range parts {
\t\t\tif i > 0 {
\t\t\t\tresult += " "
\t\t\t}
\t\t\tresult += p
\t\t}
\t\treturn result

\tcase pgtype.Point:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\treturn fmt.Sprintf("(%g,%g)", v.P.X, v.P.Y)

\tcase pgtype.Line:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\treturn fmt.Sprintf("{%g,%g,%g}", v.A, v.B, v.C)

\tcase pgtype.Lseg:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\treturn fmt.Sprintf("[(%g,%g),(%g,%g)]", v.P[0].X, v.P[0].Y, v.P[1].X, v.P[1].Y)

\tcase pgtype.Box:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\treturn fmt.Sprintf("(%g,%g),(%g,%g)", v.P[0].X, v.P[0].Y, v.P[1].X, v.P[1].Y)

\tcase pgtype.Circle:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\treturn fmt.Sprintf("<(%g,%g),%g>", v.P.X, v.P.Y, v.R)

\tcase pgtype.Path:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\tresult := ""
\t\tfor i, p := range v.P {
\t\t\tif i > 0 {
\t\t\t\tresult += ","
\t\t\t}
\t\t\tresult += fmt.Sprintf("(%g,%g)", p.X, p.Y)
\t\t}
\t\tif v.Closed {
\t\t\treturn "(" + result + ")"
\t\t}
\t\treturn "[" + result + "]"

\tcase pgtype.Polygon:
\t\tif !v.Valid {
\t\t\treturn nil
\t\t}
\t\tresult := ""
\t\tfor i, p := range v.P {
\t\t\tif i > 0 {
\t\t\t\tresult += ","
\t\t\t}
\t\t\tresult += fmt.Sprintf("(%g,%g)", p.X, p.Y)
\t\t}
\t\treturn "(" + result + ")"

\tcase fmt.Stringer:
\t\treturn v.String()

\tdefault:
\t\treturn value
\t}
}
'''

with open('resources/services/sync.go', 'w') as f:
    f.write(content)
print("OK: sync.go written")
