// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package log

import (
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/pkg/errors"
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config  `json:"zap" yaml:"zap"`
	Trace              *TraceConfig `json:"trace" yaml:"trace"`
	StderrRedirectFile *string      `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool         `json:"stdLogRedirect" yaml:"stdLogRedirect"`
	EcsIntegration     bool         `json:"ecsIntegration" yaml:"ecsIntegration"`
}

var (
	_globalCfg        GlobalConfig
	_logMu            sync.RWMutex
	_logServeMux      = http.NewServeMux()
	_subLoggers       map[string]*zap.Logger
	_globalLoggerName = "global"
	_dynamicFields    atomic.Value // []zap.Field
)

func init() {
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapCfg.Level.SetLevel(zap.InfoLevel)
	l, err := zapCfg.Build()
	if err != nil {
		log.Println("Failed to init zap global logger, no zap log will be shown till zap is properly initialized: ", err)
		return
	}
	_logMu.Lock()
	_globalCfg.Zap = &zapCfg
	_subLoggers = make(map[string]*zap.Logger)
	_logMu.Unlock()
	zap.ReplaceGlobals(l)
}

// L wraps zap.L().
func L() *zap.Logger { return zap.L() }

// S wraps zap.S().
func S() *zap.SugaredLogger { return zap.S() }

// Logger returns logger of the given name
func Logger(name string) *zap.Logger {
	logger, ok := _subLoggers[name]
	if !ok {
		return L()
	}
	return logger
}

// SetDynamicFields updates fields injected into every log entry at write time.
func SetDynamicFields(fields ...zap.Field) {
	cloned := append([]zap.Field(nil), fields...)
	_dynamicFields.Store(cloned)
}

func currentDynamicFields() []zap.Field {
	fields, _ := _dynamicFields.Load().([]zap.Field)
	return fields
}

type dynamicFieldsCore struct {
	zapcore.Core
}

func (c dynamicFieldsCore) With(fields []zapcore.Field) zapcore.Core {
	return dynamicFieldsCore{Core: c.Core.With(fields)}
}

func (c dynamicFieldsCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(ent.Level) {
		return ce
	}
	return ce.AddCore(ent, c)
}

func (c dynamicFieldsCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	dynamic := currentDynamicFields()
	if len(dynamic) == 0 {
		return c.Core.Write(ent, fields)
	}
	combined := make([]zapcore.Field, 0, len(fields)+len(dynamic))
	combined = append(combined, fields...)
	combined = append(combined, dynamic...)
	return c.Core.Write(ent, combined)
}

// InitLoggers initializes the global logger and other sub loggers.
func InitLoggers(globalCfg GlobalConfig, subCfgs map[string]GlobalConfig, opts ...zap.Option) error {
	if _, exists := subCfgs[_globalLoggerName]; exists {
		return errors.New("'" + _globalLoggerName + "' is a reserved name for global logger")
	}
	subCfgs[_globalLoggerName] = globalCfg
	for name, cfg := range subCfgs {
		if _, exists := _subLoggers[name]; exists {
			return errors.Errorf("duplicate sub logger name: %s", name)
		}
		if cfg.Zap == nil {
			zapCfg := zap.NewProductionConfig()
			cfg.Zap = &zapCfg
		} else {
			cfg.Zap.EncoderConfig = zap.NewProductionEncoderConfig()
		}
		if globalCfg.EcsIntegration {
			cfg.Zap.EncoderConfig = ecszap.ECSCompatibleEncoderConfig(cfg.Zap.EncoderConfig)
		}

		var cores []zapcore.Core
		if cfg.StderrRedirectFile != nil {
			stderrF, err := os.OpenFile(*cfg.StderrRedirectFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0600)
			if err != nil {
				return err
			}

			cores = append(cores, zapcore.NewCore(
				zapcore.NewJSONEncoder(cfg.Zap.EncoderConfig),
				zapcore.AddSync(stderrF),
				cfg.Zap.Level))
		}
		switch cfg.Zap.Encoding {
		case "console":
			consoleCfg := zap.NewDevelopmentConfig()
			consoleCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			cores = append(cores, zapcore.NewCore(
				zapcore.NewConsoleEncoder(consoleCfg.EncoderConfig),
				zapcore.AddSync(os.Stdout),
				cfg.Zap.Level))
		case "json":
			cfg.Zap.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			cores = append(cores, zapcore.NewCore(
				zapcore.NewJSONEncoder(cfg.Zap.EncoderConfig),
				zapcore.AddSync(os.Stdout),
				cfg.Zap.Level))
		default:
			return errors.Errorf("unknown encoding: %s", cfg.Zap.Encoding)
		}

		core := dynamicFieldsCore{Core: zapcore.NewTee(cores...)}
		logger := zap.New(core, opts...)

		_logMu.Lock()
		if name == _globalLoggerName {
			_globalCfg = cfg
			if cfg.RedirectStdLog {
				zap.RedirectStdLog(logger)
			}
			zap.ReplaceGlobals(logger)
			if err := initTraceLogger(logger, cfg.Trace); err != nil {
				return err
			}
		} else {
			_subLoggers[name] = logger
		}
		_logServeMux.HandleFunc("/"+name, cfg.Zap.Level.ServeHTTP)
		_logMu.Unlock()
	}

	return nil
}

// RegisterLevelConfigMux registers log's level config http mux.
func RegisterLevelConfigMux(root *http.ServeMux) {
	_logMu.Lock()
	root.Handle("/logging/", http.StripPrefix("/logging", _logServeMux))
	_logMu.Unlock()
}
