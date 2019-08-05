// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package log

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GlobalConfig defines the global logger configurations.
type GlobalConfig struct {
	Zap                *zap.Config `json:"zap" yaml:"zap"`
	StderrRedirectFile *string     `json:"stderrRedirectFile" yaml:"stderrRedirectFile"`
	RedirectStdLog     bool        `json:"stdLogRedirect" yaml:"stdLogRedirect"`
}

var (
	_globalCfg  GlobalConfig
	_subLoggers map[string]*zap.Logger
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
	_globalCfg.Zap = &zapCfg
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
func InitLoggers(globalCfg GlobalConfig) error {

	if globalCfg.Zap == nil {
		zapCfg := zap.NewProductionConfig()
		globalCfg.Zap = &zapCfg
	} else {
		globalCfg.Zap.EncoderConfig = zap.NewProductionEncoderConfig()
	}
	logger, err := globalCfg.Zap.Build()
	if err != nil {
		return err
	}
	if globalCfg.StderrRedirectFile != nil {
		stderrF, err := os.OpenFile(*globalCfg.StderrRedirectFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		if err := redirectStderr(stderrF); err != nil {
			return err
		}
	}
	if globalCfg.RedirectStdLog {
		zap.RedirectStdLog(logger)
	}
	zap.ReplaceGlobals(logger)

	return nil
}
