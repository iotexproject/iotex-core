// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package log

import (
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/pkg/errors"
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config `json:"zap" yaml:"zap"`
	StderrRedirectFile *string     `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool        `json:"stdLogRedirect" yaml:"stdLogRedirect"`
	EcsIntegration     bool        `json:"ecsIntegration" yaml:"ecsIntegration"`
	RotateFileName     string      `json:"rotateFileName" yaml:"rotateFileName"`
	RotateMaxBackups   int         `json:"rotateMaxBackups" yaml:"rotateMaxBackups"`
	RotateTimeFormat   string      `json:"rotateTimeFormat" yaml:"rotateTimeFormat"`
	RotateLocalTime    bool        `json:"rotateLocalTime" yaml:"rotateLocalTime"`
}

var (
	_globalCfg        GlobalConfig
	_logMu            sync.RWMutex
	_logServeMux      = http.NewServeMux()
	_subLoggers       map[string]*zap.Logger
	_globalLoggerName = "global"
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
		var logger *zap.Logger
		var err error
		if cfg.RotateFileName != "" {
			logger, err = buildZap(cfg, opts...)
		} else {
			logger, err = cfg.Zap.Build(opts...)
		}
		if err != nil {
			return err
		}
		if cfg.StderrRedirectFile != nil {
			stderrF, err := os.OpenFile(*cfg.StderrRedirectFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			if err := redirectStderr(stderrF); err != nil {
				return err
			}
		}
		_logMu.Lock()
		if name == _globalLoggerName {
			_globalCfg = cfg
			if cfg.RedirectStdLog {
				zap.RedirectStdLog(logger)
			}
			zap.ReplaceGlobals(logger)
		} else {
			_subLoggers[name] = logger
		}
		_logServeMux.HandleFunc("/"+name, cfg.Zap.Level.ServeHTTP)
		_logMu.Unlock()
	}

	return nil
}

func buildZap(cfg GlobalConfig, opts ...zap.Option) (*zap.Logger, error) {
	var enc zapcore.Encoder
	if cfg.Zap.Encoding == "json" {
		enc = zapcore.NewJSONEncoder(cfg.Zap.EncoderConfig)
	} else {
		enc = zapcore.NewConsoleEncoder(cfg.Zap.EncoderConfig)
	}

	var ws zapcore.WriteSyncer
	rotateFile := &RotateFile{
		Filename:         cfg.RotateFileName,
		MaxBackups:       cfg.RotateMaxBackups,
		BackupTimeFormat: cfg.RotateTimeFormat,
	}
	ws = zapcore.AddSync(rotateFile)

	zapCore := zapcore.NewCore(
		enc,
		zapcore.AddSync(ws),
		cfg.Zap.Level,
	)
	log := zap.New(zapCore, buildZapOptions(cfg.Zap, ws)...)
	if len(opts) > 0 {
		log = log.WithOptions(opts...)
	}
	return log, nil
}

func buildZapOptions(cfg *zap.Config, errSink zapcore.WriteSyncer) []zap.Option {
	opts := []zap.Option{zap.ErrorOutput(errSink)}

	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	if !cfg.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}

	stackLevel := zap.ErrorLevel
	if cfg.Development {
		stackLevel = zap.WarnLevel
	}
	if !cfg.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(stackLevel))
	}

	if scfg := cfg.Sampling; scfg != nil {
		opts = append(opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			var samplerOpts []zapcore.SamplerOption
			if scfg.Hook != nil {
				samplerOpts = append(samplerOpts, zapcore.SamplerHook(scfg.Hook))
			}
			return zapcore.NewSamplerWithOptions(
				core,
				time.Second,
				cfg.Sampling.Initial,
				cfg.Sampling.Thereafter,
				samplerOpts...,
			)
		}))
	}

	if len(cfg.InitialFields) > 0 {
		fs := make([]zap.Field, 0, len(cfg.InitialFields))
		keys := make([]string, 0, len(cfg.InitialFields))
		for k := range cfg.InitialFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fs = append(fs, zap.Any(k, cfg.InitialFields[k]))
		}
		opts = append(opts, zap.Fields(fs...))
	}

	return opts
}

// RegisterLevelConfigMux registers log's level config http mux.
func RegisterLevelConfigMux(root *http.ServeMux) {
	_logMu.Lock()
	root.Handle("/logging/", http.StripPrefix("/logging", _logServeMux))
	_logMu.Unlock()
}
