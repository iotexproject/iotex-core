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
	cfg     config.Config
	idx     *Indexer
	bc      blockchain.Blockchain
	blockCh chan *blockchain.Block
}

// NewServer instantiates an index service
func NewServer(
	cfg config.Config,
	bc blockchain.Blockchain,
) *Server {
	blockCh := make(chan *blockchain.Block)
	if err := bc.SubscribeBlockCreation(blockCh); err != nil {
		logger.Error().Err(err).Msg("error when subscribe to block")
		return nil
	}

	return &Server{
		cfg: cfg,
		idx: &Indexer{
			cfg:                cfg.Indexer,
			rds:                nil,
			hexEncodedNodeAddr: "",
		},
		bc:      bc,
		blockCh: blockCh,
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

	go func() {
		for {
			select {
			case blk := <-s.blockCh:
				if err := s.idx.BuildIndex(blk); err != nil {
					logger.Error().Err(err).Uint64("height", blk.Height()).Msg("failed to build index for block")
				}
			}
		}
	}()

	return nil
}

// Stop stops the explorer server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.idx.rds.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when shutting down explorer http server")
	}
	logger.Info().Msgf("Unsubscribe block creation for chain %d", s.bc.ChainID())
	if err := s.bc.UnsubscribeBlockCreation(s.blockCh); err != nil {
		return errors.Wrap(err, "error when un subscribe block creation")
	}
	close(s.blockCh)
	for range s.blockCh {
	}
	return nil
}

// Indexer return indexer interface
func (s *Server) Indexer() *Indexer { return s.idx }
