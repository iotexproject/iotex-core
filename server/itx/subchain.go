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

	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
)

func (s *Server) runSubChain(addr address.Address, subChain *mainchain.SubChain) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, ok := s.chainservices[subChain.ChainID]
	if ok {
		return nil
	}
	// TODO: get rid of the hack config modification
	cfg := s.cfg
	cfg.Chain.ID = subChain.ChainID
	cfg.Chain.Address = addr.IotxAddress()
	cfg.Chain.ChainDBPath = getSubChainDBPath(subChain.ChainID, cfg.Chain.ChainDBPath)
	cfg.Chain.TrieDBPath = getSubChainDBPath(subChain.ChainID, cfg.Chain.TrieDBPath)
	cfg.Chain.GenesisActionsPath = ""
	cfg.Chain.EnableSubChainStartInGenesis = false
	cfg.Chain.EmptyGenesis = true
	cfg.Explorer.Port = cfg.Explorer.Port - int(s.rootChainService.ChainID()) + int(subChain.ChainID)
	if err := s.newSubChainService(cfg); err != nil {
		return err
	}
	cs, ok := s.chainservices[subChain.ChainID]
	if !ok {
		return errors.New("failed to get the newly created chain service")
	}
	// TODO: pass in the parent context instead
	if err := cs.Start(context.Background()); err != nil {
		return err
	}
	// TODO: we may also need to unsubscribe this before stopping sub-cahin
	s.dispatcher.AddSubscriber(cs.ChainID(), cs)
	return nil
}

func (s *Server) isSubChainRunning(chainID uint32) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, ok := s.chainservices[chainID]
	return ok
}

// HandleBlock implements interface BlockCreationSubscriber
func (s *Server) HandleBlock(blk *blockchain.Block) error {
	runnableSubChains, err := s.mainChainProtocol.SubChainsInOperation()
	if err != nil {
		logger.Error().Err(err).Msg("error when getting the sub-chains in operation slice")
	}
	for _, runnableSubChain := range runnableSubChains {
		if s.isSubChainRunning(runnableSubChain.ID) {
			continue
		}
		addr, err := address.BytesToAddress(runnableSubChain.Addr)
		if err != nil {
			logger.Error().
				Err(err).
				Uint32("chainID", runnableSubChain.ID).
				Msg("error when getting the sub-chain address")
			continue
		}
		subChain, err := s.mainChainProtocol.SubChain(addr)
		if err != nil {
			logger.Error().
				Err(err).
				Uint32("chainID", subChain.ChainID).
				Msg("error when getting the sub-chain state")
			continue
		}
		if subChain.StartHeight <= blk.Height() {
			if err := s.runSubChain(addr, subChain); err != nil {
				logger.Error().
					Err(err).
					Uint32("chainID", subChain.ChainID).
					Msg("error when put sub-chain service in operation")
				continue
			}
			logger.Info().
				Uint32("chainID", subChain.ChainID).
				Msg("started the sub-chain")
		}
	}
	return nil
}

func getSubChainDBPath(chainID uint32, p string) string {
	dir, file := path.Split(p)
	return path.Join(dir, fmt.Sprintf("chain-%d-%s", chainID, file))
}
