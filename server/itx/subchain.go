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
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/logger"
)

func (s *Server) newOrGetSubChainService(subChainInOp mainchain.InOperation) (
	*chainservice.ChainService,
	*mainchain.SubChain,
	error,
) {
	chainID := subChainInOp.ID
	var cs *chainservice.ChainService
	addr, err := address.BytesToAddress(subChainInOp.Addr)
	if err != nil {
		return nil, nil, err
	}
	subChain, err := s.mainChainProtocol.SubChain(addr)
	if err != nil {
		return nil, nil, err
	}
	cs, ok := s.chainservices[chainID]
	if ok {
		return cs, subChain, nil
	}
	// TODO: get rid of the hack config modification
	cfg := s.cfg
	cfg.Chain.ID = chainID
	cfg.Chain.Address = addr.IotxAddress()
	cfg.Chain.ChainDBPath = getSubChainDBPath(chainID, cfg.Chain.ChainDBPath)
	cfg.Chain.TrieDBPath = getSubChainDBPath(chainID, cfg.Chain.TrieDBPath)
	cfg.Chain.GenesisActionsPath = ""
	cfg.Chain.EnableSubChainStartInGenesis = false
	cfg.Chain.EmptyGenesis = true
	cfg.Explorer.Port = cfg.Explorer.Port - int(s.rootChainService.ChainID()) + int(chainID)
	if err := s.newSubChainService(cfg); err != nil {
		return nil, nil, err
	}
	cs, ok = s.chainservices[chainID]
	if !ok {
		return nil, nil, errors.New("failed to get the chain service")
	}
	return cs, subChain, nil
}

// HandleBlock implements interface BlockCreationSubscriber
func (s *Server) HandleBlock(blk *blockchain.Block) error {
	subChainsInOp, err := s.mainChainProtocol.SubChainsInOperation()
	if err != nil {
		logger.Error().Err(err).Msg("error when getting the sub-chains in operation slice")
	}
	for _, e := range subChainsInOp {
		subChainInOp, ok := e.(mainchain.InOperation)
		if !ok {
			logger.Error().Msg("error when casting the element in the sorted slice into InOperation")
			continue
		}
		cs, subChain, err := s.newOrGetSubChainService(subChainInOp)
		if err != nil {
			logger.Error().Err(err).Msg("error when getting sub-chain service")
			continue
		}
		if subChain.StartHeight <= blk.Height() && !cs.IsRunning() {
			if err := cs.Start(context.Background()); err != nil {
				logger.Error().Err(err).
					Uint32("sub-chain", subChain.ChainID).
					Msg("error when starting the sub-chain")
				continue
			}
			logger.Info().Uint32("sub-chain", subChain.ChainID).Msg("started the sub-chain")
		}
	}
	return nil
}

func getSubChainDBPath(chainID uint32, p string) string {
	dir, file := path.Split(p)
	return path.Join(dir, fmt.Sprintf("chain-%d-%s", chainID, file))
}
