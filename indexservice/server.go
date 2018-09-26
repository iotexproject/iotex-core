// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package indexservice

import (
	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/cloudrds"
	"github.com/iotexproject/iotex-core/config"
)

// Server is the container of the explorer service
type Server struct {
	cfg config.IndexService
	idx *IndexService
}

// NewServer instantiates an explorer server
func NewServer(
	cfg config.IndexService,
) *Server {
	return &Server{
		cfg: cfg,
		idx: &IndexService{
			cfg: cfg,
			rds: nil,
		},
	}
}

// Start starts the explorer server
func (s *Server) Start(ctx context.Context) error {
	s.idx.rds = cloudrds.NewAwsRDS()
	if err := s.idx.rds.Start(ctx); err != nil {
		return errors.Wrap(err, "error when start rds store")
	}

	return nil
}

// Stop stops the explorer server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.idx.rds.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when shutting down explorer http server")
	}
	return nil
}

// logFilter example of Filter implementation
/*type logFilter struct{}

// PreInvoke implement empty preinvoke
func (f logFilter) PreInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("logFilter: PreInvoke of method:", r.Method)
	return true
}

// PostInvoke implement empty postinvoke
func (f logFilter) PostInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("logFilter: PostInvoke of method:", r.Method)
	return true
}*/
