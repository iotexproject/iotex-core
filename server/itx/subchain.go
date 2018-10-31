// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"fmt"
	"path"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocols/subchain"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

func (s *Server) newSubChainStarter(protocol *subchain.Protocol) *routine.RecurringTask {
	return routine.NewRecurringTask(
		func() {
			subChainsInOp, err := protocol.SubChainsInOperation()
			if err != nil {
				logger.Error().Err(err).Msg("error when getting the sub-chains in operation slice")
			}
			for _, e := range subChainsInOp {
				subChainInOp, ok := e.(subchain.InOperation)
				if !ok {
					logger.Error().Msg("error when casting the element in the sorted slice into InOperation")
					continue
				}
				if _, ok := s.chainservices[subChainInOp.ID]; ok {
					// Sub-chain service is already started
					continue
				}
				addr, err := address.BytesToAddress(subChainInOp.Addr)
				if err != nil {
					logger.Error().Err(err).Msg("error when converting bytes to address")
					continue
				}
				subChain, err := protocol.SubChain(addr)
				if err != nil {
					logger.Error().Err(err).
						Uint32("sub-chain", subChain.ChainID).
						Msg("error when getting the sub-chain state")
					continue
				}
				if err := s.startSubChainService(addr.IotxAddress(), subChain); err != nil {
					logger.Error().Err(err).
						Uint32("sub-chain", subChain.ChainID).
						Msg("error when starting the sub-chain service")
				}
			}
		},
		s.cfg.System.StartSubChainInterval,
	)
}

func (s *Server) startSubChainService(addr string, sc *subchain.SubChain) error {
	block := make(chan *blockchain.Block)
	if err := s.rootChainService.Blockchain().SubscribeBlockCreation(block); err != nil {
		return errors.Wrap(err, "error when subscribing block creation")
	}

	go func() {
		for started := false; !started; {
			select {
			case blk := <-block:
				if blk.Height() < sc.StartHeight {
					continue
				}
				// TODO: get rid of the hack config modification
				cfg := *s.cfg
				cfg.Chain.ID = sc.ChainID
				cfg.Chain.Address = addr
				cfg.Chain.ChainDBPath = getSubChainDBPath(sc.ChainID, cfg.Chain.ChainDBPath)
				cfg.Chain.TrieDBPath = getSubChainDBPath(sc.ChainID, cfg.Chain.TrieDBPath)
				cfg.Chain.EnableSubChainStartInGenesis = false
				cfg.Explorer.Port = cfg.Explorer.Port - int(subchain.MainChainID) + int(sc.ChainID)
				if err := s.NewChainService(&cfg); err != nil {
					logger.Error().Err(err).Msgf("error when constructing the sub-chain %d", sc.ChainID)
					continue
				}
				// TODO: inherit ctx from root chain
				if err := s.StartChainService(context.Background(), sc.ChainID); err != nil {
					logger.Error().Err(err).Msgf("error when starting the sub-chain %d", sc.ChainID)
					continue
				}
				logger.Info().Msgf("started the sub-chain %d", sc.ChainID)
				// No matter if the start process failed or not
				started = true
			}
		}
		if err := s.rootChainService.Blockchain().UnsubscribeBlockCreation(block); err != nil {
			logger.Error().Err(err).Msg("error when unsubscribing block creation")
		}
	}()

	return nil
}

func getSubChainDBPath(chainID uint32, p string) string {
	dir, file := path.Split(p)
	return path.Join(dir, fmt.Sprintf("chain-%d-%s", chainID, file))
}
