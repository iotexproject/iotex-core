// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// A warrper for Zerolog (https://github.com/rs/zerolog)
//
// Package log provides a global logger for zerolog.
// derived from https://github.com/rs/zerolog/blob/master/log/log.go
// putting here to get a better integration

package log

import (
	"log"
	"net/http"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	_globalCfg   GlobalConfig
	_globalCfgMu sync.RWMutex
	_globalMux   = http.NewServeMux()
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config `json:"zap" yaml:"zap"`
	StderrRedirectFile *string     `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool        `json:"stdLogRedirect" yaml:"stdLogRedirect"`
}

func init() {
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapCfg.Level.SetLevel(zap.InfoLevel)
	l, err := zapCfg.Build()
	if err != nil {
		log.Println("Failed to init zap global logger, no zap log will be shown till zap is properly initialized: ", err)
		return
	}
	_globalCfgMu.Lock()
	_globalCfg.Zap = &zapCfg
	_globalCfgMu.Unlock()
	zap.ReplaceGlobals(l)
}

// L wraps zap.L().
func L() *zap.Logger { return zap.L() }

// S wraps zap.S().
func S() *zap.SugaredLogger { return zap.S() }

// InitGlobal initializes the global logger.
func InitGlobal(cfg GlobalConfig, opts ...zap.Option) error {
	if cfg.Zap == nil {
		zapCfg := zap.NewProductionConfig()
		cfg.Zap = &zapCfg
	} else {
		cfg.Zap.EncoderConfig = zap.NewProductionEncoderConfig()
	}

	l, err := cfg.Zap.Build(opts...)
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

	if cfg.RedirectStdLog {
		zap.RedirectStdLog(l)
	}

	_globalCfgMu.Lock()
	_globalCfg = cfg
	_globalMux.HandleFunc("/global", _globalCfg.Zap.Level.ServeHTTP)
	_globalCfgMu.Unlock()

	zap.ReplaceGlobals(l)
	return nil
}

// RegisterLevelConfigMux registers log's level config http mux.
func RegisterLevelConfigMux(root *http.ServeMux) {
	_globalCfgMu.Lock()
	root.Handle("/logging/", http.StripPrefix("/logging", _globalMux))
	_globalCfgMu.Unlock()
}
