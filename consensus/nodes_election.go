// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	rp "github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/pkg/errors"
)

type nodesElection struct {
	bc      blockchain.Blockchain
	sf      factory.Factory
	pp      poll.Protocol
	rp      *rp.Protocol
	genesis genesis.Genesis
}

func newNodesElection(bc blockchain.Blockchain, sf factory.Factory, pp poll.Protocol, rp *rp.Protocol, genesis genesis.Genesis) *nodesElection {
	return &nodesElection{
		bc:      bc,
		sf:      sf,
		pp:      pp,
		rp:      rp,
		genesis: genesis,
	}
}

func (ne *nodesElection) Delegates(epochNum uint64) ([]string, error) {
	re := protocol.NewRegistry()
	if err := ne.rp.Register(re); err != nil {
		return nil, err
	}
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), re),
		ne.genesis,
	)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	tipHeight := ne.bc.TipHeight()
	tipEpochNum := ne.rp.GetEpochNum(tipHeight)
	var candidatesList state.CandidateList
	var err error
	switch epochNum {
	case tipEpochNum:
		candidatesList, err = ne.pp.Delegates(ctx, ne.sf)
	case tipEpochNum + 1:
		candidatesList, err = ne.pp.NextDelegates(ctx, ne.sf)
	default:
		err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochNum, tipEpochNum)
	}
	if err != nil {
		return nil, err
	}
	addrs := []string{}
	for _, candidate := range candidatesList {
		addrs = append(addrs, candidate.Address)
	}
	return addrs, nil
}

func (ne *nodesElection) Proposors(epochNum uint64) ([]string, error) {
	// TODO implement select proposors in each epoch
	// use delegates as proposors at present
	return ne.Delegates(epochNum)
}
