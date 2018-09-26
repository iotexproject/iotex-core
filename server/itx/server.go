// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"

	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/network"

	"github.com/pkg/errors"
)

// Server is the iotex server instance containing all components.
type Server struct {
	chainservices map[uint32]*chainservice.ChainService
	p2p           network.Overlay
	dispatcher    dispatcher.Dispatcher
}

// NewServer creates a new server
// TODO clean up config, make root config contains network, dispatch and chainservice
func NewServer(cfg *config.Config) (*Server, error) {
	return newServer(cfg, false)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg *config.Config) (*Server, error) {
	return newServer(cfg, true)
}

func newServer(cfg *config.Config, testing bool) (*Server, error) {
	// create P2P network and BlockSync
	p2p := network.NewOverlay(&cfg.Network)

	// create dispatcher instance
	dispatcher, err := dispatch.NewDispatcher(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create dispatcher")
	}
	p2p.AttachDispatcher(dispatcher)

	chains := make(map[uint32]*chainservice.ChainService)

	var cs *chainservice.ChainService

	if testing {
		cs, err = chainservice.NewTesting(cfg, p2p, dispatcher)
	} else {
		cs, err = chainservice.New(cfg, p2p, dispatcher)
	}
	if err != nil {
		return nil, errors.Wrap(err, "fail to create chain service")
	}

	chains[cs.ChainID()] = cs
	dispatcher.AddSubscriber(cs.ChainID(), cs)
	return &Server{
		p2p:           p2p,
		dispatcher:    dispatcher,
		chainservices: chains,
	}, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	for _, cs := range s.chainservices {
		if err := cs.Start(ctx); err != nil {
			return errors.Wrap(err, "error when stopping blockchain")
		}
	}
	if err := s.dispatcher.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}
	if err := s.p2p.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting P2P networks")
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.p2p.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping P2P networks")
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping dispatcher")
	}
	for _, cs := range s.chainservices {
		if err := cs.Stop(ctx); err != nil {
			return errors.Wrap(err, "error when stopping blockchain")
		}
	}
	return nil
}

// NewChainService creates a new chain service in this server.
func (s *Server) NewChainService(cfg *config.Config) error {
	cs, err := chainservice.New(cfg, s.p2p, s.dispatcher)
	if err != nil {
		return err
	}
	s.chainservices[cs.ChainID()] = cs
	s.dispatcher.AddSubscriber(cs.ChainID(), cs)
	return nil
}

// NewTestingChainService creates a new testing chain service in this server.
func (s *Server) NewTestingChainService(cfg *config.Config) error {
	cs, err := chainservice.NewTesting(cfg, s.p2p, s.dispatcher)
	if err != nil {
		return err
	}
	s.chainservices[cs.ChainID()] = cs
	s.dispatcher.AddSubscriber(cs.ChainID(), cs)
	return nil
}

// StartChainService starts the chain service run in the server.
func (s *Server) StartChainService(ctx context.Context, id uint32) error {
	c, ok := s.chainservices[id]
	if !ok {
		return errors.New("Chain ID does not match any existing chains")
	}
	return c.Start(ctx)
}

// StopChainService stops the chain service run in the server.
func (s *Server) StopChainService(ctx context.Context, id uint32) error {
	c, ok := s.chainservices[id]
	if !ok {
		return errors.New("Chain ID does not match any existing chains")
	}
	return c.Stop(ctx)
}

// P2P returns the P2P network
func (s *Server) P2P() network.Overlay {
	return s.p2p
}

// ChainService returns the chainservice hold in Server with given id.
func (s *Server) ChainService(id uint32) *chainservice.ChainService { return s.chainservices[id] }

// Dispatcher returns the Dispatcher
func (s *Server) Dispatcher() dispatcher.Dispatcher {
	return s.dispatcher
}
