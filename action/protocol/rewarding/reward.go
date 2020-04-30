// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

// rewardHistory is the dummy struct to record a reward. Only key matters.
type rewardHistory struct{}

// Serialize serializes reward history state into bytes
func (b rewardHistory) Serialize() ([]byte, error) {
	gen := rewardingpb.RewardHistory{}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into reward history state
func (b *rewardHistory) Deserialize(data []byte) error { return nil }

// rewardHistory stores the unclaimed balance of an account
type rewardAccount struct {
	balance *big.Int
}

// Serialize serializes account state into bytes
func (a rewardAccount) Serialize() ([]byte, error) {
	gen := rewardingpb.Account{
		Balance: a.balance.String(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into account state
func (a *rewardAccount) Deserialize(data []byte) error {
	gen := rewardingpb.Account{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	balance, ok := big.NewInt(0).SetString(gen.Balance, 10)
	if !ok {
		return errors.New("failed to set reward account balance")
	}
	a.balance = balance
	return nil
}

// GrantBlockReward grants the block reward (token) to the block producer
func (p *Protocol) GrantBlockReward(
	ctx context.Context,
	sm protocol.StateManager,
) (*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if err := p.assertNoRewardYet(sm, blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, err
	}

	producerAddrStr := blkCtx.Producer.String()
	rewardAddrStr := ""
	pp := poll.FindProtocol(protocol.MustGetRegistry(ctx))
	if pp != nil {
		candidates, err := pp.Candidates(ctx, sm)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if candidate.Address == producerAddrStr {
				rewardAddrStr = candidate.RewardAddress
				break
			}
		}
	}
	// If reward address doesn't exist, do nothing
	if rewardAddrStr == "" {
		log.S().Debugf("Producer %s doesn't have a reward address", producerAddrStr)
		return nil, nil
	}
	rewardAddr, err := address.FromString(rewardAddrStr)

	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	if err := p.updateAvailableBalance(sm, a.blockReward); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if err := p.grantToAccount(sm, rewardAddr, a.blockReward); err != nil {
		return nil, err
	}
	if err := p.updateRewardHistory(sm, blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, err
	}
	rewardLog := rewardingpb.RewardLog{
		Type:   rewardingpb.RewardLog_BLOCK_REWARD,
		Addr:   rewardAddrStr,
		Amount: a.blockReward.String(),
	}
	data, err := proto.Marshal(&rewardLog)
	if err != nil {
		return nil, err
	}
	return &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actionCtx.ActionHash,
	}, nil
}

// GrantEpochReward grants the epoch reward (token) to all beneficiaries of a epoch
func (p *Protocol) GrantEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	if err := p.assertNoRewardYet(sm, epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, err
	}
	if err := p.assertLastBlockInEpoch(blkCtx.BlockHeight, epochNum, rp); err != nil {
		return nil, err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}

	// Get the delegate list who exempts epoch reward
	e := exempt{}
	if err := p.state(sm, exemptKey, &e); err != nil {
		return nil, err
	}
	exemptAddrs := make(map[string]interface{})
	for _, addr := range e.addrs {
		exemptAddrs[addr.String()] = nil
	}

	var err error
	uqd := make(map[string]bool)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	if hu.IsPre(config.Easter, epochStartHeight) {
		// Get unqualified delegate list
		if uqd, err = p.unqualifiedDelegates(ctx, sm, rp, epochNum, a.productivityThreshold); err != nil {
			return nil, err
		}
	}
	candidates, err := poll.MustGetProtocol(protocol.MustGetRegistry(ctx)).Candidates(ctx, sm)
	if err != nil {
		return nil, err
	}
	addrs, amounts, err := p.splitEpochReward(epochStartHeight, sm, candidates, a.epochReward, a.numDelegatesForEpochReward, exemptAddrs, uqd)
	if err != nil {
		return nil, err
	}
	actualTotalReward := big.NewInt(0)
	rewardLogs := make([]*action.Log, 0)
	for i := range addrs {
		// If reward address doesn't exist, do nothing
		if addrs[i] == nil {
			continue
		}
		// If 0 epoch reward due to low productivity, do nothing
		if amounts[i].Cmp(big.NewInt(0)) == 0 {
			continue
		}
		if err := p.grantToAccount(sm, addrs[i], amounts[i]); err != nil {
			return nil, err
		}
		rewardLog := rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_EPOCH_REWARD,
			Addr:   addrs[i].String(),
			Amount: amounts[i].String(),
		}
		data, err := proto.Marshal(&rewardLog)
		if err != nil {
			return nil, err
		}
		rewardLogs = append(rewardLogs, &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		})
		actualTotalReward = big.NewInt(0).Add(actualTotalReward, amounts[i])
	}

	// Reward additional bootstrap bonus
	if epochNum <= a.foundationBonusLastEpoch || (epochNum >= p.foundationBonusP2StartEpoch && epochNum <= p.foundationBonusP2EndEpoch) {
		for i, count := 0, uint64(0); i < len(candidates) && count < a.numDelegatesForFoundationBonus; i++ {
			if _, ok := exemptAddrs[candidates[i].Address]; ok {
				continue
			}
			if candidates[i].Votes.Cmp(big.NewInt(0)) == 0 {
				// hard probation
				continue
			}
			count++
			// If reward address doesn't exist, do nothing
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			rewardAddr, err := address.FromString(candidates[i].RewardAddress)
			if err != nil {
				return nil, err
			}
			if err := p.grantToAccount(sm, rewardAddr, a.foundationBonus); err != nil {
				return nil, err
			}
			rewardLog := rewardingpb.RewardLog{
				Type:   rewardingpb.RewardLog_FOUNDATION_BONUS,
				Addr:   candidates[i].RewardAddress,
				Amount: a.foundationBonus.String(),
			}
			data, err := proto.Marshal(&rewardLog)
			if err != nil {
				return nil, err
			}
			rewardLogs = append(rewardLogs, &action.Log{
				Address:     p.addr.String(),
				Topics:      nil,
				Data:        data,
				BlockHeight: blkCtx.BlockHeight,
				ActionHash:  actionCtx.ActionHash,
			})
			actualTotalReward = big.NewInt(0).Add(actualTotalReward, a.foundationBonus)
		}
	}

	// Update actual reward
	if err := p.updateAvailableBalance(sm, actualTotalReward); err != nil {
		return nil, err
	}
	if err := p.updateRewardHistory(sm, epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, err
	}
	return rewardLogs, nil
}

// Claim claims the token from the rewarding fund
func (p *Protocol) Claim(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	actionCtx := protocol.MustGetActionCtx(ctx)
	if err := p.assertAmount(amount); err != nil {
		return err
	}
	if err := p.updateTotalBalance(sm, amount); err != nil {
		return err
	}
	return p.claimFromAccount(sm, actionCtx.Caller, amount)
}

// UnclaimedBalance returns unclaimed balance of a given address
func (p *Protocol) UnclaimedBalance(
	ctx context.Context,
	sm protocol.StateReader,
	addr address.Address,
) (*big.Int, error) {
	acc := rewardAccount{}
	accKey := append(adminKey, addr.Bytes()...)
	err := p.state(sm, accKey, &acc)
	if err == nil {
		return acc.balance, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		return big.NewInt(0), nil
	}
	return nil, err
}

func (p *Protocol) updateTotalBalance(sm protocol.StateManager, amount *big.Int) error {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return err
	}
	totalBalance := big.NewInt(0).Sub(f.totalBalance, amount)
	if totalBalance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough total balance")
	}
	f.totalBalance = totalBalance
	return p.putState(sm, fundKey, &f)
}

func (p *Protocol) updateAvailableBalance(sm protocol.StateManager, amount *big.Int) error {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return err
	}
	availableBalance := big.NewInt(0).Sub(f.unclaimedBalance, amount)
	if availableBalance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough available balance")
	}
	f.unclaimedBalance = availableBalance
	return p.putState(sm, fundKey, &f)
}

func (p *Protocol) grantToAccount(sm protocol.StateManager, addr address.Address, amount *big.Int) error {
	acc := rewardAccount{}
	accKey := append(adminKey, addr.Bytes()...)
	if err := p.state(sm, accKey, &acc); err != nil {
		if errors.Cause(err) != state.ErrStateNotExist {
			return err
		}
		acc = rewardAccount{
			balance: big.NewInt(0),
		}
	}
	acc.balance = big.NewInt(0).Add(acc.balance, amount)
	return p.putState(sm, accKey, &acc)
}

func (p *Protocol) claimFromAccount(sm protocol.StateManager, addr address.Address, amount *big.Int) error {
	// Update reward account
	acc := rewardAccount{}
	accKey := append(adminKey, addr.Bytes()...)
	if err := p.state(sm, accKey, &acc); err != nil {
		return err
	}
	balance := big.NewInt(0).Sub(acc.balance, amount)
	if balance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough available balance")
	}
	// TODO: we may want to delete the account when the unclaimed balance becomes 0
	acc.balance = balance
	if err := p.putState(sm, accKey, &acc); err != nil {
		return err
	}

	// Update primary account
	primAcc, err := accountutil.LoadOrCreateAccount(sm, addr.String())
	if err != nil {
		return err
	}
	primAcc.Balance = big.NewInt(0).Add(primAcc.Balance, amount)
	return accountutil.StoreAccount(sm, addr.String(), primAcc)
}

func (p *Protocol) updateRewardHistory(sm protocol.StateManager, prefix []byte, index uint64) error {
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], index)
	return p.putState(sm, append(prefix, indexBytes[:]...), &rewardHistory{})
}

func (p *Protocol) splitEpochReward(
	epochStartHeight uint64,
	sm protocol.StateManager,
	candidates []*state.Candidate,
	totalAmount *big.Int,
	numDelegatesForEpochReward uint64,
	exemptAddrs map[string]interface{},
	uqd map[string]bool,
) ([]address.Address, []*big.Int, error) {
	filteredCandidates := make([]*state.Candidate, 0)
	for _, candidate := range candidates {
		if _, ok := exemptAddrs[candidate.Address]; ok {
			continue
		}
		filteredCandidates = append(filteredCandidates, candidate)
	}
	candidates = filteredCandidates
	if len(candidates) == 0 {
		return nil, nil, nil
	}
	// We at most allow numDelegatesForEpochReward delegates to get the epoch reward
	if uint64(len(candidates)) > numDelegatesForEpochReward {
		candidates = candidates[:numDelegatesForEpochReward]
	}
	totalWeight := big.NewInt(0)
	rewardAddrs := make([]address.Address, 0)
	for _, candidate := range candidates {
		var rewardAddr address.Address
		var err error
		if candidate.RewardAddress != "" {
			rewardAddr, err = address.FromString(candidate.RewardAddress)
			if err != nil {
				return nil, nil, err
			}
		} else {
			log.S().Warnf("Candidate %s doesn't have a reward address", candidate.Address)
		}
		rewardAddrs = append(rewardAddrs, rewardAddr)
		totalWeight = big.NewInt(0).Add(totalWeight, candidate.Votes)
	}
	amounts := make([]*big.Int, 0)
	var amountPerAddr *big.Int
	for _, candidate := range candidates {
		if totalWeight.Cmp(big.NewInt(0)) == 0 {
			amounts = append(amounts, big.NewInt(0))
			continue
		}
		if _, ok := uqd[candidate.Address]; ok {
			// Before Easter, if not qualified, skip the epoch reward
			amounts = append(amounts, big.NewInt(0))
			continue
		}
		amountPerAddr = big.NewInt(0).Div(big.NewInt(0).Mul(totalAmount, candidate.Votes), totalWeight)
		amounts = append(amounts, amountPerAddr)
	}
	return rewardAddrs, amounts, nil
}

func (p *Protocol) unqualifiedDelegates(
	ctx context.Context,
	sm protocol.StateManager,
	rp *rolldpos.Protocol,
	epochNum uint64,
	productivityThreshold uint64,
) (map[string]bool, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	delegates, err := poll.MustGetProtocol(protocol.MustGetRegistry(ctx)).Delegates(ctx, sm)
	if err != nil {
		return nil, err
	}
	unqualifiedDelegates := make(map[string]bool, 0)
	numBlks, produce, err := rp.ProductivityByEpoch(epochNum, bcCtx.Tip.Height, p.productivity)
	if err != nil {
		return nil, err
	}
	// The current block is not included, so add it
	numBlks++
	if _, ok := produce[blkCtx.Producer.String()]; ok {
		produce[blkCtx.Producer.String()]++
	} else {
		produce[blkCtx.Producer.String()] = 1
	}
	for _, abp := range delegates {
		if _, ok := produce[abp.Address]; !ok {
			produce[abp.Address] = 0
		}
	}
	expectedNumBlks := numBlks / uint64(len(produce))
	for addr, actualNumBlks := range produce {
		if actualNumBlks*100/expectedNumBlks < productivityThreshold {
			unqualifiedDelegates[addr] = true
		}
	}
	return unqualifiedDelegates, nil
}

func (p *Protocol) assertNoRewardYet(sm protocol.StateManager, prefix []byte, index uint64) error {
	history := rewardHistory{}
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], index)
	err := p.state(sm, append(prefix, indexBytes[:]...), &history)
	if err == nil {
		return errors.Errorf("reward history already exists on index %d", index)
	}
	if errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	return nil
}

func (p *Protocol) assertLastBlockInEpoch(blkHeight uint64, epochNum uint64, rp *rolldpos.Protocol) error {
	lastBlkHeight := rp.GetEpochLastBlockHeight(epochNum)
	if blkHeight != lastBlkHeight {
		return errors.Errorf("current block %d is not the last block of epoch %d", blkHeight, epochNum)
	}
	return nil
}
