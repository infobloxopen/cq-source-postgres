package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeResolver_ReturnsFunction(t *testing.T) {
	resolver := MakeResolver(nil, "public", "users")
	assert.NotNil(t, resolver)
}

func TestMakeResolver_NilPoolError(t *testing.T) {
	resolver := MakeResolver(nil, "public", "users")
	require.NotNil(t, resolver)

	ctx := context.Background()
	res := make(chan any, 10)
	err := resolver(ctx, nil, nil, res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// T051: Batching tests — verify convertPgValue handles all types correctly
// ensuring data is consumable by the SDK's Arrow record builder.
// The actual rows_per_record batching is handled by SDK scheduler (WithBatchMaxRows).

func TestConvertPgValue_Nil(t *testing.T) {
	assert.Nil(t, convertPgValue(nil))
}

func TestConvertPgValue_PassthroughTypes(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"string", "hello"},
		{"int", 42},
		{"int64", int64(100)},
		{"float64", 3.14},
		{"bool", true},
		{"bytes", []byte("data")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPgValue(tt.input)
			assert.Equal(t, tt.input, result)
		})
	}
}

func TestConvertPgValue_Numeric(t *testing.T) {
	t.Run("valid numeric", func(t *testing.T) {
		n := pgtype.Numeric{Valid: true}
		_ = n.ScanInt64(pgtype.Int8{Int64: 42, Valid: true})
		result := convertPgValue(n)
		assert.NotNil(t, result)
		assert.IsType(t, "", result)
	})

	t.Run("invalid numeric", func(t *testing.T) {
		n := pgtype.Numeric{Valid: false}
		assert.Nil(t, convertPgValue(n))
	})
}

func TestConvertPgValue_Time(t *testing.T) {
	t.Run("valid time", func(t *testing.T) {
		pt := pgtype.Time{Valid: true, Microseconds: 45296000000} // 12:34:56
		result := convertPgValue(pt)
		assert.Equal(t, "12:34:56", result)
	})

	t.Run("time with microseconds", func(t *testing.T) {
		pt := pgtype.Time{Valid: true, Microseconds: 45296123456} // 12:34:56.123456
		result := convertPgValue(pt)
		assert.Equal(t, "12:34:56.123456", result)
	})

	t.Run("invalid time", func(t *testing.T) {
		pt := pgtype.Time{Valid: false}
		assert.Nil(t, convertPgValue(pt))
	})
}

func TestConvertPgValue_Interval(t *testing.T) {
	t.Run("zero interval", func(t *testing.T) {
		iv := pgtype.Interval{Valid: true}
		result := convertPgValue(iv)
		assert.Equal(t, "00:00:00", result)
	})

	t.Run("days and hours", func(t *testing.T) {
		iv := pgtype.Interval{Valid: true, Days: 3, Microseconds: 7200000000} // 3 day 02:00:00
		result := convertPgValue(iv)
		assert.Contains(t, fmt.Sprintf("%v", result), "3 day")
	})

	t.Run("invalid interval", func(t *testing.T) {
		iv := pgtype.Interval{Valid: false}
		assert.Nil(t, convertPgValue(iv))
	})
}

func TestConvertPgValue_GeometryTypes(t *testing.T) {
	t.Run("point", func(t *testing.T) {
		p := pgtype.Point{P: pgtype.Vec2{X: 1, Y: 2}, Valid: true}
		result := convertPgValue(p)
		assert.Equal(t, "(1,2)", result)
	})

	t.Run("invalid point", func(t *testing.T) {
		p := pgtype.Point{Valid: false}
		assert.Nil(t, convertPgValue(p))
	})

	t.Run("circle", func(t *testing.T) {
		c := pgtype.Circle{P: pgtype.Vec2{X: 0, Y: 0}, R: 5, Valid: true}
		result := convertPgValue(c)
		assert.Equal(t, "<(0,0),5>", result)
	})

	t.Run("box", func(t *testing.T) {
		b := pgtype.Box{P: [2]pgtype.Vec2{{X: 1, Y: 2}, {X: 3, Y: 4}}, Valid: true}
		result := convertPgValue(b)
		assert.Equal(t, "(1,2),(3,4)", result)
	})
}

func TestConvertPgValue_Timestamp(t *testing.T) {
	t.Run("time.Time midnight passes through as time.Time", func(t *testing.T) {
		d := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		result := convertPgValue(d)
		assert.Equal(t, d, result) // must stay time.Time, not become "2026-01-15"
	})

	t.Run("time.Time with time component passes through", func(t *testing.T) {
		d := time.Date(2026, 1, 15, 12, 30, 0, 0, time.UTC)
		result := convertPgValue(d)
		assert.Equal(t, d, result)
	})

	t.Run("pgtype.Date", func(t *testing.T) {
		d := pgtype.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true}
		result := convertPgValue(d)
		assert.Equal(t, "2026-01-15", result)
	})

	t.Run("pgtype.Date invalid", func(t *testing.T) {
		d := pgtype.Date{Valid: false}
		assert.Nil(t, convertPgValue(d))
	})
}

func TestConvertPgValue_Stringer(t *testing.T) {
	// net.IP implements Stringer
	// Test simple string passthrough
	val := "simple string"
	result := convertPgValue(val)
	assert.Equal(t, val, result)
}
