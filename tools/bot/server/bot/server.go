// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bot

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/bot/config"
)

// Service defines service interface
type Service interface {
	Start(ctx context.Context) error
	Stop()
	Name() string
}

// Server is the iotex server instance containing all components.
type Server struct {
	cfg         config.Config
	runServices map[string]Service
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewServer creates a new server
func NewServer(cfg config.Config) (*Server, error) {
	rs := make(map[string]Service)
	svr := Server{
		cfg:         cfg,
		runServices: rs,
	}
	return &svr, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s.startTicker()
}

// Stop stops the server
func (s *Server) Stop() {
	s.cancel()
	for _, service := range s.runServices {
		service.Stop()
	}
}

// Register register services
func (s *Server) Register(ss ...Service) error {
	for _, service := range ss {
		s.runServices[service.Name()] = service
	}
	return nil
}

func (s *Server) startTicker() error {
	go func() {
		d := time.Duration(s.cfg.RunInterval) * time.Second
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				log.L().Info("start run :")
				s.runRegisterOnce()
			case <-s.ctx.Done():
				log.L().Info("exit :")
				return
			}

		}
	}()
	return nil
}
func (s *Server) runRegisterOnce() {
	for _, service := range s.runServices {
		err := service.Start(s.ctx)
		if err != nil {
			log.L().Error("err:", zap.String("service", service.Name()), zap.Error(err))
		}
	}
}
