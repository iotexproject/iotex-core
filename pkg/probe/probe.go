// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package probe

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/httputil"
)

// Server is a http server for service probe.
type Server struct {
	lifecycle.Readiness
	server           http.Server
	readinessHandler http.Handler
}

// Option is ued to set probe server's options.
type Option interface {
	SetOption(*Server)
}

// New creates a new probe server.
func New(port int, opts ...Option) *Server {
	s := &Server{
		readinessHandler: http.HandlerFunc(successHandleFunc),
	}

	for _, opt := range opts {
		opt.SetOption(s)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", successHandleFunc)
	readiness := func(w http.ResponseWriter, r *http.Request) {
		if !s.IsReady() {
			failureHandleFunc(w, r)
			return
		}
		s.readinessHandler.ServeHTTP(w, r)
	}

	mux.HandleFunc("/readiness", readiness)
	mux.HandleFunc("/health", readiness)
	mux.Handle("/metrics", promhttp.Handler())

	s.server = httputil.NewServer(fmt.Sprintf(":%d", port), mux)
	return s
}

// Start starts the probe server and starts returning success status on liveness endpoint.
func (s *Server) Start(_ context.Context) error {
	go func() {
		ln, err := httputil.LimitListener(s.server.Addr)
		if err != nil {
			log.L().Error("Failed to listen on probe port", zap.Error(err))
			return
		}
		if err := s.server.Serve(ln); err != nil {
			log.L().Info("Probe server stopped.", zap.Error(err))
		}
	}()
	return nil
}

// Stop shutdown the probe server.
func (s *Server) Stop(ctx context.Context) error { return s.server.Shutdown(ctx) }

func successHandleFunc(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.L().Warn("Failed to send http response.", zap.Error(err))
	}
}

func failureHandleFunc(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	if _, err := w.Write([]byte("FAIL")); err != nil {
		log.L().Warn("Failed to send http response.", zap.Error(err))
	}
}
