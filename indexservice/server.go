// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/rds"
	"github.com/iotexproject/iotex-core/logger"
)

// Server is the container of the index service
type Server struct {
	cfg config.Config
	idx *Indexer
	bc  blockchain.Blockchain
}

// NewServer instantiates an index service
func NewServer(
	cfg config.Config,
	bc blockchain.Blockchain,
) *Server {
	indexer := &Indexer{
		cfg:                cfg.Indexer,
		rds:                nil,
		hexEncodedNodeAddr: "",
	}
	if err := bc.AddSubscriber(indexer); err != nil {
		logger.Error().Err(err).Msg("error when subscribe to block")
		return nil
	}

	return &Server{
		cfg: cfg,
		idx: indexer,
		bc:  bc,
	}
}

// Start starts the explorer server
func (s *Server) Start(ctx context.Context) error {
	addr := s.cfg.Indexer.NodeAddr
	if addr == "" {
		blockAddr, err := s.cfg.BlockchainAddress()
		if err != nil {
			return errors.Wrap(err, "error when get the blockchain address")
		}
		addr = hex.EncodeToString(blockAddr.Bytes()[:])
	}
	s.idx.hexEncodedNodeAddr = addr

	s.idx.rds = rds.NewAwsRDS(&s.cfg.DB.RDS)
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
	logger.Info().Msgf("Unsubscribe block creation for chain %d", s.bc.ChainID())
	if err := s.bc.RemoveSubscriber(s.idx); err != nil {
		return errors.Wrap(err, "error when unsubscribe block creation")
	}

	return nil
}

// Indexer return indexer interface
func (s *Server) Indexer() *Indexer { return s.idx }
