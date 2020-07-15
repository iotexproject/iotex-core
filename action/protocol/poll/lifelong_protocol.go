// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

type lifeLongDelegatesProtocol struct {
	delegates state.CandidateList
	addr      address.Address
}

// NewLifeLongDelegatesProtocol creates a poll protocol with life long delegates
func NewLifeLongDelegatesProtocol(delegates []genesis.Delegate) Protocol {
	var l state.CandidateList
	for _, delegate := range delegates {
		rewardAddress := delegate.RewardAddr()
		if rewardAddress == nil {
			rewardAddress = delegate.OperatorAddr()
		}
		l = append(l, &state.Candidate{
			Address: delegate.OperatorAddr().String(),
			// TODO: load votes from genesis
			Votes:         delegate.Votes(),
			RewardAddress: rewardAddress.String(),
		})
	}
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}
	return &lifeLongDelegatesProtocol{delegates: l, addr: addr}
}

func (p *lifeLongDelegatesProtocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight != 0 {
		return errors.Errorf("Cannot create genesis state for height %d", blkCtx.BlockHeight)
	}
	log.L().Info("Creating genesis states for lifelong delegates protocol")
	return setCandidates(ctx, sm, nil, p.delegates, uint64(1))
}

func (p *lifeLongDelegatesProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	if err := validate(ctx, sm, p, act); err != nil {
		return nil, err
	}
	return handle(ctx, act, sm, nil, p.addr.String())
}

func (p *lifeLongDelegatesProtocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	return validate(ctx, sr, p, act)
}

func (p *lifeLongDelegatesProtocol) CalculateCandidatesByHeight(ctx context.Context, _ protocol.StateReader, _ uint64) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *lifeLongDelegatesProtocol) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.readActiveBlockProducers(ctx, sr, false)
}

func (p *lifeLongDelegatesProtocol) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.readActiveBlockProducers(ctx, sr, true)
}

func (p *lifeLongDelegatesProtocol) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *lifeLongDelegatesProtocol) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *lifeLongDelegatesProtocol) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	return nil, nil
}

func (p *lifeLongDelegatesProtocol) ReadState(
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
	case "CandidatesByEpoch":
		fallthrough
	case "BlockProducersByEpoch":
		fallthrough
	case "ActiveBlockProducersByEpoch":
		bp, err := p.readBlockProducers()
		return bp, height, err
	case "GetGravityChainStartHeight":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		return args[0], height, nil
	default:
		return nil, uint64(0), errors.New("corresponding method isn't found")
	}
}

// Register registers the protocol with a unique ID
func (p *lifeLongDelegatesProtocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *lifeLongDelegatesProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

// Name returns the name of protocol
func (p *lifeLongDelegatesProtocol) Name() string {
	return protocolID
}

func (p *lifeLongDelegatesProtocol) readBlockProducers() ([]byte, error) {
	return p.delegates.Serialize()
}

func (p *lifeLongDelegatesProtocol) readActiveBlockProducers(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, error) {
	var blockProducerList []string
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	blockProducerMap := make(map[string]*state.Candidate)
	delegates := p.delegates
	if len(p.delegates) > int(rp.NumCandidateDelegates()) {
		delegates = p.delegates[:rp.NumCandidateDelegates()]
	}
	for _, bp := range delegates {
		blockProducerList = append(blockProducerList, bp.Address)
		blockProducerMap[bp.Address] = bp
	}
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	crypto.SortCandidates(blockProducerList, targetEpochStartHeight, crypto.CryptoSeed)
	length := int(rp.NumDelegates())
	if len(blockProducerList) < length {
		// TODO: if the number of delegates is smaller than expected, should it return error or not?
		length = len(blockProducerList)
	}

	var activeBlockProducers state.CandidateList
	for i := 0; i < length; i++ {
		activeBlockProducers = append(activeBlockProducers, blockProducerMap[blockProducerList[i]])
	}
	return activeBlockProducers, nil
}
