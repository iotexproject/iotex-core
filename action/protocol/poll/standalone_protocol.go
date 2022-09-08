// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

type standaloneProtocol struct {
	delegates state.CandidateList
	addr      address.Address
}

// NewStandaloneProtocol creates a poll protocol with one producer delegate in standalone mode
func NewStandaloneProtocol(producer address.Address) Protocol {
	var l state.CandidateList
	l = append(l, &state.Candidate{
		Address:       producer.String(),
		Votes:         big.NewInt(0),
		RewardAddress: producer.String(),
	})
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}
	return &standaloneProtocol{delegates: l, addr: addr}
}

func (p *standaloneProtocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	return nil
}

func (p *standaloneProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	if err := validate(ctx, sm, p, act); err != nil {
		return nil, err
	}
	return handle(ctx, act, sm, nil, p.addr.String())
}

func (p *standaloneProtocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	return validate(ctx, sr, p, act)
}

func (p *standaloneProtocol) CalculateCandidatesByHeight(ctx context.Context, _ protocol.StateReader, _ uint64) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *standaloneProtocol) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *standaloneProtocol) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *standaloneProtocol) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *standaloneProtocol) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *standaloneProtocol) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	return nil, nil
}

func (p *standaloneProtocol) ReadState(
	ctx context.Context,
	sr protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, uint64, error) {
	height, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}
	switch string(method) {
	case "BlockProducersByEpoch":
		fallthrough
	case "ActiveBlockProducersByEpoch":
		bp, err := p.readBlockProducers()
		return bp, height, err
	case "ProbationListByEpoch":
		fallthrough
	case "GetGravityChainStartHeight":
		return nil, height, nil
	default:
		return nil, uint64(0), errors.New("corresponding method isn't found")
	}
}

// Register registers the protocol with a unique ID
func (p *standaloneProtocol) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *standaloneProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, p)
}

// Name returns the name of protocol
func (p *standaloneProtocol) Name() string {
	return _protocolID
}

func (p *standaloneProtocol) readBlockProducers() ([]byte, error) {
	return p.delegates.Serialize()
}
