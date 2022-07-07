// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package zlog

import (
	"io"
	stdlog "log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"go.uber.org/zap"

	zaplog "github.com/iotexproject/iotex-core/pkg/log"
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config `json:"zap" yaml:"zap"`
	StderrRedirectFile *string     `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool        `json:"stdLogRedirect" yaml:"stdLogRedirect"`
	EcsIntegration     bool        `json:"ecsIntegration" yaml:"ecsIntegration"`
}

var (
	_globalCfg        zaplog.GlobalConfig
	_logMu            sync.RWMutex
	_logServeMux      = http.NewServeMux()
	_subLoggers       map[string]*zerolog.Logger
	_globalLoggerName = "zlogglobal"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	_logMu.Lock()
	_subLoggers = make(map[string]*zerolog.Logger)
	_logMu.Unlock()
}

// L wraps global logger.
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
func InitLoggers(globalCfg zaplog.GlobalConfig, subCfgs map[string]zaplog.GlobalConfig, fields interface{}) error {
	if _, exists := subCfgs[_globalLoggerName]; exists {
		return errors.New("'" + _globalLoggerName + "' is a reserved name for global logger")
	}
	subCfgs[_globalLoggerName] = globalCfg
	for name, cfg := range subCfgs {
		if _, exists := _subLoggers[name]; exists {
			return errors.Errorf("duplicate sub logger name: %s", name)
		}

		writers := []io.Writer{zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}}
		if cfg.StderrRedirectFile != nil {
			stderrF, err := os.OpenFile(*cfg.StderrRedirectFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			writers = append(writers, &partsSortWriter{
				f: stderrF,
			})
		}
		log.Logger = log.Output(zerolog.MultiLevelWriter(writers...)).
			With().
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
