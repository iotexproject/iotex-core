package log

import (
	"context"
	"fmt"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	_traceLogger = otelzap.New(zap.NewNop())
)

// Ctx is a wrapper of otelzap.Ctx, returns a logger with context.
func Ctx(ctx context.Context) otelzap.LoggerWithCtx {
	return _traceLogger.Ctx(ctx)
}

// TraceConfig defines the logger configurations for tracing.
type TraceConfig struct {
	MinLevel         string `json:"minLevel" yaml:"minLevel"`
	ErrorStatusLevel string `json:"errorStatusLevel" yaml:"errorStatusLevel"`
	Caller           bool   `json:"caller" yaml:"caller"`
	CallerDepth      int    `json:"callerDepth" yaml:"callerDepth"`
	StackTrace       bool   `json:"stackTrace" yaml:"stackTrace"`
	WithTraceID      bool   `json:"withTraceID" yaml:"withTraceID"`
}

func initTraceLogger(logger *zap.Logger, cfg *TraceConfig) error {
	opts, err := parseTraceConfig(cfg)
	if err != nil {
		return err
	}
	_traceLogger = otelzap.New(logger, opts...)
	return nil
}

func parseTraceConfig(cfg *TraceConfig) ([]otelzap.Option, error) {
	var opts []otelzap.Option
	if cfg == nil {
		return opts, nil
	}
	if cfg.MinLevel != "" {
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(cfg.MinLevel)); err != nil {
			return nil, fmt.Errorf("invalid minLevel: %w", err)
		}
		opts = append(opts, otelzap.WithMinLevel(level))
	}
	if cfg.ErrorStatusLevel != "" {
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(cfg.ErrorStatusLevel)); err != nil {
			return nil, fmt.Errorf("invalid errorStatusLevel: %w", err)
		}
		opts = append(opts, otelzap.WithErrorStatusLevel(level))
	}
	if cfg.Caller {
		opts = append(opts, otelzap.WithCaller(true))
	}
	if cfg.CallerDepth > 0 {
		opts = append(opts, otelzap.WithCallerDepth(cfg.CallerDepth))
	}
	if cfg.StackTrace {
		opts = append(opts, otelzap.WithStackTrace(true))
	}
	if cfg.WithTraceID {
		opts = append(opts, otelzap.WithTraceIDField(true))
	}
	return opts, nil
}
