package util

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/server/itx"
)

// StartNode starts a node server
func StartNode(svr *itx.Server, cfg *config.Config) {
	ctx := context.Background()
	if err := svr.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server.")
		return
	}
	defer func() {
		if err := svr.Stop(ctx); err != nil {
			logger.Panic().Err(err).Msg("Failed to stop server.")
		}
	}()

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(itx.NewHeartbeatHandler(svr).Log, cfg.System.HeartbeatInterval)
		if err := task.Start(ctx); err != nil {
			logger.Panic().Err(err).Msg("Failed to start heartbeat routine.")
		}
		defer func() {
			if err := task.Stop(ctx); err != nil {
				logger.Panic().Err(err).Msg("Failed to stop heartbeat routine.")
			}
		}()
	}

	if cfg.System.HTTPProfilingPort > 0 {
		go func() {
			if err := http.ListenAndServe(
				fmt.Sprintf(":%d", cfg.System.HTTPProfilingPort),
				nil,
			); err != nil {
				logger.Error().Err(err).Msg("error when serving performance profiling data")
			}
		}()
	}

	if cfg.System.HTTPMetricsPort > 0 {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		port := fmt.Sprintf(":%d", cfg.System.HTTPMetricsPort)
		go func() {
			if err := http.ListenAndServe(port, mux); err != nil {
				logger.Error().Err(err).Msg("error when serving performance profiling data")
			}
		}()
	}
	select {}
}
