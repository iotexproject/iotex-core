// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package log

import (
	"io"
	stdlog "log"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config `json:"zap" yaml:"zap"`
	StderrRedirectFile *string     `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool        `json:"stdLogRedirect" yaml:"stdLogRedirect"`
	EcsIntegration     bool        `json:"ecsIntegration" yaml:"ecsIntegration"`
}

var (
	_globalCfg        GlobalConfig
	_logMu            sync.RWMutex
	_logServeMux      = http.NewServeMux()
	_subLoggers       map[string]*zerolog.Logger
	_globalLoggerName = "global"
)

func init() {
	// zapCfg := zap.NewDevelopmentConfig()
	// zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	// zapCfg.Level.SetLevel(zap.InfoLevel)
	// l, err := zapCfg.Build()
	// if err != nil {
	// 	log.Println("Failed to init zap global logger, no zap log will be shown till zap is properly initialized: ", err)
	// 	return
	// }

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	_logMu.Lock()
	// _globalCfg.Zap = &zapCfg
	_subLoggers = make(map[string]*zerolog.Logger)
	_logMu.Unlock()
}

// L wraps zap.L().
func L() *zerolog.Logger { return &log.Logger }

// Logger returns logger of the given name
func Logger(name string) *zerolog.Logger {
	logger, ok := _subLoggers[name]
	if !ok {
		return L()
	}
	return logger
}

// InitLoggers initializes the global logger and other sub loggers.
func InitLoggers(globalCfg GlobalConfig, subCfgs map[string]GlobalConfig, fields interface{}) error {
	if _, exists := subCfgs[_globalLoggerName]; exists {
		return errors.New("'" + _globalLoggerName + "' is a reserved name for global logger")
	}
	subCfgs[_globalLoggerName] = globalCfg
	for name, cfg := range subCfgs {
		if _, exists := _subLoggers[name]; exists {
			return errors.Errorf("duplicate sub logger name: %s", name)
		}
		// if cfg.Zap == nil {
		// 	zapCfg := zap.NewProductionConfig()
		// 	cfg.Zap = &zapCfg
		// } else {
		// 	cfg.Zap.EncoderConfig = zap.NewProductionEncoderConfig()
		// }
		// if globalCfg.EcsIntegration {
		// 	cfg.Zap.EncoderConfig = ecszap.ECSCompatibleEncoderConfig(cfg.Zap.EncoderConfig)
		// }
		// logger, err := cfg.Zap.Build(opts...)
		// if err != nil {
		// return err
		// }

		writers := []io.Writer{zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}}
		if cfg.StderrRedirectFile != nil {
			stderrF, err := os.OpenFile(*cfg.StderrRedirectFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			if err := redirectStderr(stderrF); err != nil {
				return err
			}
			writers = append(writers, stderrF)
		}
		log.Logger = log.Output(zerolog.MultiLevelWriter(writers...)).
			With().
			Timestamp().
			Caller().
			Fields(fields).
			Logger()

		_logMu.Lock()
		if name == _globalLoggerName {
			_globalCfg = cfg
			if cfg.RedirectStdLog {
				stdlog.SetFlags(0)
				stdlog.SetPrefix("")
				stdlog.SetOutput(log.Logger)
			}
		} else {
			_subLoggers[name] = &log.Logger
		}

		zerohandler := hlog.NewHandler(log.Logger)
		handler := zerohandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hlog.FromRequest(r).Info().Msg("")
		}))
		_logServeMux.HandleFunc("/"+name, handler.ServeHTTP)
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
