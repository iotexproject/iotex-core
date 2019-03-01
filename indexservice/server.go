// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/log"
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
		store:              nil,
		hexEncodedNodeAddr: "",
	}
	if err := bc.AddSubscriber(indexer); err != nil {
		log.L().Error("Error when subscribe to block.", zap.Error(err))
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
		addr = s.cfg.ProducerAddress().String()
	}
	s.idx.hexEncodedNodeAddr = addr

	if s.cfg.Indexer.WhetherLocalStore {
		// local store use sqlite3
		s.idx.store = sql.NewSQLite3(s.cfg.DB.SQLITE3)
	} else {
		// remote store use aws rds
		s.idx.store = sql.NewAwsRDS(s.cfg.DB.RDS)
	}
	if err := s.idx.store.Start(ctx); err != nil {
		return errors.Wrap(err, "error when start store")
	}

	// create local table
	if err := s.idx.CreateTablesIfNotExist(); err != nil {
		return errors.Wrap(err, "error when initial tables")
	}

	return nil
}

// Stop stops the explorer server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.idx.store.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when shutting down explorer http server")
	}
	log.L().Info("Unsubscribe block creation.", zap.Uint32("chainID", s.bc.ChainID()))
	if err := s.bc.RemoveSubscriber(s.idx); err != nil {
		return errors.Wrap(err, "error when unsubscribe block creation")
	}

	return nil
}

// Indexer return indexer interface
func (s *Server) Indexer() *Indexer { return s.idx }
