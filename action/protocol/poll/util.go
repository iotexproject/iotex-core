// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

func validateDelegates(cs state.CandidateList) error {
	zero := big.NewInt(0)
	addrs := map[string]bool{}
	lastVotes := zero
	for _, candidate := range cs {
		if _, exists := addrs[candidate.Address]; exists {
			return errors.Errorf("duplicate candidate %s", candidate.Address)
		}
		addrs[candidate.Address] = true
		if candidate.Votes.Cmp(zero) < 0 {
			return errors.New("votes for candidate cannot be negative")
		}
		if lastVotes.Cmp(zero) > 0 && lastVotes.Cmp(candidate.Votes) < 0 {
			return errors.New("candidate list is not sorted")
		}
	}
	return nil
}

func handle(ctx context.Context, act action.Action, sm protocol.StateManager, indexer *CandidateIndexer, protocolAddr string) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	zap.L().Debug("Handle PutPollResult Action", zap.Uint64("height", r.Height()))

	if err := setCandidates(ctx, sm, indexer, r.Candidates(), r.Height()); err != nil {
		return nil, errors.Wrap(err, "failed to set candidates")
	}
	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		ActionHash:      actionCtx.ActionHash,
		BlockHeight:     blkCtx.BlockHeight,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: protocolAddr,
	}, nil
}

func validate(ctx context.Context, sr protocol.StateReader, p Protocol, act action.Action) error {
	ppr, ok := act.(*action.PutPollResult)
	if !ok {
		return nil
	}
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if blkCtx.Producer.String() != actionCtx.Caller.String() {
		return errors.New("Only producer could create this protocol")
	}
	proposedDelegates := ppr.Candidates()
	if err := validateDelegates(proposedDelegates); err != nil {
		return err
	}
	ds, err := p.CalculateCandidatesByHeight(ctx, sr, blkCtx.BlockHeight)
	if err != nil {
		return err
	}
	if len(ds) != len(proposedDelegates) {
		msg := fmt.Sprintf(", %d, is not as expected, %d",
			len(proposedDelegates),
			len(ds))
		return errors.Wrap(ErrProposedDelegatesLength, msg)
	}
	for i, d := range ds {
		if !proposedDelegates[i].Equal(d) {
			msg := fmt.Sprintf(", %v vs %v (expected)",
				proposedDelegates,
				ds)
			return errors.Wrap(ErrDelegatesNotAsExpected, msg)
		}
	}
	return nil
}

func createPostSystemActions(ctx context.Context, sr protocol.StateReader, p Protocol) ([]action.Envelope, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	lastBlkHeight := rp.GetEpochLastBlockHeight(epochNum)
	epochHeight := rp.GetEpochHeight(epochNum)
	nextEpochHeight := rp.GetEpochHeight(epochNum + 1)
	// make sure that putpollresult action is created around half of each epoch
	if blkCtx.BlockHeight < epochHeight+(nextEpochHeight-epochHeight)/2 {
		return nil, nil
	}
	loadCandidatesLegacy := featureCtx.LoadCandidatesLegacy(nextEpochHeight)
	if _, _, err := candidatesutil.CandidatesFromDB(sr, nextEpochHeight, loadCandidatesLegacy, true); errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	log.L().Debug(
		"createPutPollResultAction",
		zap.Uint64("height", blkCtx.BlockHeight),
		zap.Uint64("epochNum", epochNum),
		zap.Uint64("epochHeight", epochHeight),
		zap.Uint64("nextEpochHeight", nextEpochHeight),
	)
	l, err := p.CalculateCandidatesByHeight(ctx, sr, epochHeight)
	if err == nil && len(l) == 0 {
		err = errors.Wrapf(
			ErrDelegatesNotExist,
			"failed to fetch delegates by epoch height %d, empty list",
			epochHeight,
		)
	}

	if err != nil && blkCtx.BlockHeight == lastBlkHeight {
		return nil, errors.Wrapf(
			err,
			"failed to prepare delegates for next epoch %d",
			epochNum+1,
		)
	}

	nonce := uint64(0)
	pollAction := action.NewPutPollResult(nonce, nextEpochHeight, l)
	builder := action.EnvelopeBuilder{}

	return []action.Envelope{builder.SetNonce(nonce).SetAction(pollAction).Build()}, nil
}

// setCandidates sets the candidates for the given state manager
func setCandidates(
	ctx context.Context,
	sm protocol.StateManager,
	indexer *CandidateIndexer,
	candidates state.CandidateList,
	height uint64, // epoch start height
) error {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(height)
	if height != rp.GetEpochHeight(epochNum) {
		return errors.New("put poll result height should be epoch start height")
	}
	loadCandidatesLegacy := featureCtx.LoadCandidatesLegacy(height)
	for _, candidate := range candidates {
		delegate, err := accountutil.LoadOrCreateAccount(sm, candidate.Address)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account for delegate %s", candidate.Address)
		}
		delegate.IsCandidate = true
		if loadCandidatesLegacy {
			if err := candidatesutil.LoadAndAddCandidates(sm, height, candidate.Address); err != nil {
				return err
			}
		}
		candAddr, err := address.FromString(candidate.Address)
		if err != nil {
			errors.Wrap(err, "failed to convert candidate address")
		}
		if err := accountutil.StoreAccount(sm, candAddr, delegate); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		log.L().Debug(
			"add candidate",
			zap.String("address", candidate.Address),
			zap.String("rewardAddress", candidate.RewardAddress),
			zap.String("score", candidate.Votes.String()),
		)
	}
	if indexer != nil {
		if err := indexer.PutCandidateList(height, &candidates); err != nil {
			return errors.Wrapf(err, "failed to put candidatelist into indexer at height %d", height)
		}
	}
	if loadCandidatesLegacy {
		_, err := sm.PutState(&candidates, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(height)))
		return err
	}
	nextKey := candidatesutil.ConstructKey(candidatesutil.NxtCandidateKey)
	_, err := sm.PutState(&candidates, protocol.KeyOption(nextKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
	return err
}

// setNextEpochProbationList sets the probation list with next key
func setNextEpochProbationList(
	sm protocol.StateManager,
	indexer *CandidateIndexer,
	height uint64,
	probationlist *vote.ProbationList,
) error {
	if indexer != nil {
		if err := indexer.PutProbationList(height, probationlist); err != nil {
			return errors.Wrapf(err, "failed to put probationlist into indexer at height %d", height)
		}
	}
	probationListKey := candidatesutil.ConstructKey(candidatesutil.NxtProbationKey)
	_, err := sm.PutState(probationlist, protocol.KeyOption(probationListKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
	return err
}

// setUnproductiveDelegates sets the upd struct with updkey
func setUnproductiveDelegates(
	sm protocol.StateManager,
	upd *vote.UnproductiveDelegate,
) error {
	updKey := candidatesutil.ConstructKey(candidatesutil.UnproductiveDelegateKey)
	_, err := sm.PutState(upd, protocol.KeyOption(updKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
	return err
}

// shiftCandidates updates current data with next data of candidate list
func shiftCandidates(sm protocol.StateManager) (uint64, error) {
	zap.L().Debug("Shift candidatelist from next key to current key")
	var next state.CandidateList
	var err error
	var stateHeight, putStateHeight, delStateHeight uint64
	nextKey := candidatesutil.ConstructKey(candidatesutil.NxtCandidateKey)
	if stateHeight, err = sm.State(&next, protocol.KeyOption(nextKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to read next candidateList when shifting to current candidateList",
		)
	}
	curKey := candidatesutil.ConstructKey(candidatesutil.CurCandidateKey)
	if putStateHeight, err = sm.PutState(&next, protocol.KeyOption(curKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to write current candidateList when shifting from next candidateList to current candidateList",
		)
	}
	if stateHeight != putStateHeight {
		return 0, errors.Wrap(ErrInconsistentHeight, "failed to shift candidates")
	}
	if delStateHeight, err = sm.DelState(protocol.KeyOption(nextKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to delete next candidatelist after shifting",
		)
	}
	if stateHeight != delStateHeight {
		return 0, errors.Wrap(ErrInconsistentHeight, "failed to shift candidates")
	}
	return stateHeight, nil
}

// shiftProbationList updates current data with next data of probation list
func shiftProbationList(sm protocol.StateManager) (uint64, error) {
	zap.L().Debug("Shift probationList from next key to current key")
	var err error
	var stateHeight, putStateHeight, delStateHeight uint64
	next := &vote.ProbationList{}
	nextKey := candidatesutil.ConstructKey(candidatesutil.NxtProbationKey)
	if stateHeight, err = sm.State(next, protocol.KeyOption(nextKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to read next probationlist when shifting to current probationlist",
		)
	}
	curKey := candidatesutil.ConstructKey(candidatesutil.CurProbationKey)
	if putStateHeight, err = sm.PutState(next, protocol.KeyOption(curKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to write current probationlist when shifting from next probationlist to current probationlist",
		)
	}
	if stateHeight != putStateHeight {
		return 0, errors.Wrap(ErrInconsistentHeight, "failed to shift candidates")
	}
	if delStateHeight, err = sm.DelState(protocol.KeyOption(nextKey[:]), protocol.NamespaceOption(protocol.SystemNamespace)); err != nil {
		return 0, errors.Wrap(
			err,
			"failed to delete next probationlist after shifting",
		)
	}
	if stateHeight != delStateHeight {
		return 0, errors.Wrap(ErrInconsistentHeight, "failed to shift probationlist")
	}
	return stateHeight, nil
}

// setCurrentBlockMeta sets the block meta struct with block meta key
func setCurrentBlockMeta(
	sm protocol.StateManager,
	blockmeta *BlockMeta,
	height uint64,
	blocksInEpoch uint64,
) error {
	key := blockMetaKey(height, blocksInEpoch)
	_, err := sm.PutState(blockmeta, protocol.KeyOption(key), protocol.NamespaceOption(protocol.SystemNamespace))
	return err
}

// allBlockMetasFromDB returns all latest block meta structs
func allBlockMetasFromDB(sr protocol.StateReader, blocksInEpoch uint64) ([]*BlockMeta, error) {
	keys := [][]byte{blockMetaKey(math.MaxUint64, blocksInEpoch)}
	for i := uint64(0); i < blocksInEpoch; i++ {
		keys = append(keys, blockMetaKey(i, blocksInEpoch))
	}
	stateHeight, iter, err := sr.States(
		protocol.NamespaceOption(protocol.SystemNamespace),
		protocol.KeysOption(func() ([][]byte, error) {
			return keys, nil
		}),
	)
	if err != nil {
		return nil, err
	}
	log.L().Debug(
		"allBockMetasFromDB",
		zap.Uint64("state height", stateHeight),
		zap.Error(err),
	)
	blockmetas := make([]*BlockMeta, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		bm := &BlockMeta{}
		switch err := iter.Next(bm); errors.Cause(err) {
		case nil:
			blockmetas = append(blockmetas, bm)
		case state.ErrNilValue:
		default:
			return nil, errors.Wrapf(err, "failed to deserialize block meta")
		}
	}
	return blockmetas, nil
}

// blockMetaKey returns key for storing block meta with prefix
func blockMetaKey(blkHeight uint64, blocksInEpoch uint64) []byte {
	prefixKey := candidatesutil.ConstructKey(blockMetaPrefix)
	if blkHeight == math.MaxUint64 {
		return append(prefixKey[:], byteutil.Uint64ToBytesBigEndian(blocksInEpoch)...)
	}
	return append(prefixKey[:], byteutil.Uint64ToBytesBigEndian(blkHeight%blocksInEpoch)...)
}
