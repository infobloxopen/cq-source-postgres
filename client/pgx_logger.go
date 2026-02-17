package client

import (
	"context"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

// PgxLogAdapter bridges zerolog and pgx's tracelog.Logger interface.
type PgxLogAdapter struct {
	logger zerolog.Logger
}

// NewPgxLogAdapter creates a PgxLogAdapter that writes pgx log events at the specified level.
func NewPgxLogAdapter(logger zerolog.Logger, level string) *PgxLogAdapter {
	zl := mapPgxLevelToZerolog(level)
	return &PgxLogAdapter{
		logger: logger.Level(zl),
	}
}

// Log implements tracelog.Logger.
func (a *PgxLogAdapter) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	var evt *zerolog.Event
	switch level {
	case tracelog.LogLevelTrace:
		evt = a.logger.Trace()
	case tracelog.LogLevelDebug:
		evt = a.logger.Debug()
	case tracelog.LogLevelInfo:
		evt = a.logger.Info()
	case tracelog.LogLevelWarn:
		evt = a.logger.Warn()
	case tracelog.LogLevelError:
		evt = a.logger.Error()
	default:
		evt = a.logger.Info()
	}
	if data != nil {
		evt.Fields(data)
	}
	evt.Msg(msg)
}

// mapPgxLevelToZerolog converts a pgx log level string to a zerolog.Level.
func mapPgxLevelToZerolog(level string) zerolog.Level {
	switch level {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.ErrorLevel
	}
}

// PgxLogLevel converts a spec pgx_log_level string to a tracelog.LogLevel.
func PgxLogLevel(level string) tracelog.LogLevel {
	switch level {
	case "trace":
		return tracelog.LogLevelTrace
	case "debug":
		return tracelog.LogLevelDebug
	case "info":
		return tracelog.LogLevelInfo
	case "warn":
		return tracelog.LogLevelWarn
	case "error":
		return tracelog.LogLevelError
	default:
		return tracelog.LogLevelError
	}
}
