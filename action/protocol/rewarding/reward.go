// Copyright (c) 2019 IoTeX
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

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/address"
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
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertNoRewardYet(sm, blockRewardHistoryKeyPrefix, raCtx.BlockHeight); err != nil {
		return err
	}

	// Get the reward address for the block producer
	epochNum := p.rp.GetEpochNum(raCtx.BlockHeight)
	candidates, err := p.cm.CandidatesByHeight(p.rp.GetEpochHeight(epochNum))
	if err != nil {
		return err
	}
	producerAddrStr := raCtx.Producer.String()
	rewardAddrStr := ""
	for _, candidate := range candidates {
		if candidate.Address == producerAddrStr {
			rewardAddrStr = candidate.RewardAddress
			break
		}
	}
	// If reward address doesn't exist, do nothing
	if rewardAddrStr == "" {
		log.S().Warnf("Producer %s doesn't have a reward address", producerAddrStr)
		return nil
	}
	rewardAddr, err := address.FromString(rewardAddrStr)

	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	if err := p.updateAvailableBalance(sm, a.blockReward); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if err := p.grantToAccount(sm, rewardAddr, a.blockReward); err != nil {
		return err
	}
	return p.updateRewardHistory(sm, blockRewardHistoryKeyPrefix, raCtx.BlockHeight)
}

// GrantEpochReward grants the epoch reward (token) to all beneficiaries of a epoch
func (p *Protocol) GrantEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	epochNum := p.rp.GetEpochNum(raCtx.BlockHeight)
	if err := p.assertNoRewardYet(sm, epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return err
	}
	if err := p.assertLastBlockInEpoch(raCtx.BlockHeight, epochNum); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	// We need to consistently use the votes on of first block height in this epoch
	candidates, err := p.cm.CandidatesByHeight(p.rp.GetEpochHeight(epochNum))
	if err != nil {
		return err
	}

	// Get unqualified delegate list
	uqd, err := p.unqualifiedDelegates(raCtx.Producer, epochNum, a.productivityThreshold)
	if err != nil {
		return err
	}

	addrs, amounts, err := p.splitEpochReward(sm, candidates, a.epochReward, a.numDelegatesForEpochReward, uqd)
	if err != nil {
		return err
	}
	actualTotalReward := big.NewInt(0)
	for i := range addrs {
		// If reward address doesn't exist, do nothing
		if addrs[i] == nil {
			continue
		}
		if err := p.grantToAccount(sm, addrs[i], amounts[i]); err != nil {
			return err
		}
		actualTotalReward = big.NewInt(0).Add(actualTotalReward, amounts[i])
	}

	// Reward additional bootstrap bonus
	if epochNum <= a.foundationBonusLastEpoch {
		l := uint64(len(candidates))
		if l > a.numDelegatesForFoundationBonus {
			l = a.numDelegatesForFoundationBonus
		}
		for i := uint64(0); i < l; i++ {
			// If reward address doesn't exist, do nothing
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			rewardAddr, err := address.FromString(candidates[i].RewardAddress)
			if err != nil {
				return err
			}
			if err := p.grantToAccount(sm, rewardAddr, a.foundationBonus); err != nil {
				return err
			}
			actualTotalReward = big.NewInt(0).Add(actualTotalReward, a.foundationBonus)
		}
	}

	// Update actual reward
	if err := p.updateAvailableBalance(sm, actualTotalReward); err != nil {
		return err
	}
	return p.updateRewardHistory(sm, epochRewardHistoryKeyPrefix, epochNum)
}

// Claim claims the token from the rewarding fund
func (p *Protocol) Claim(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertAmount(amount); err != nil {
		return err
	}
	if err := p.updateTotalBalance(sm, amount); err != nil {
		return err
	}
	return p.claimFromAccount(sm, raCtx.Caller, amount)
}

// UnclaimedBalance returns unclaimed balance of a given address
func (p *Protocol) UnclaimedBalance(
	ctx context.Context,
	sm protocol.StateManager,
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
	primAcc, err := accountutil.LoadOrCreateAccount(sm, addr.String(), big.NewInt(0))
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
	sm protocol.StateManager,
	candidates []*state.Candidate,
	totalAmount *big.Int,
	numDelegatesForEpochReward uint64,
	uqd map[string]interface{},
) ([]address.Address, []*big.Int, error) {
	// Remove the candidates who exempt from the epoch reward
	e := exempt{}
	if err := p.state(sm, exemptKey, &e); err != nil {
		return nil, nil, err
	}
	exemptAddrs := make(map[string]interface{})
	for _, addr := range e.addrs {
		exemptAddrs[addr.String()] = nil
	}
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
	for _, candidate := range candidates {
		// If not qualified, skip the epoch reward
		if _, ok := uqd[candidate.Address]; ok {
			amounts = append(amounts, big.NewInt(0))
			continue
		}
		var amountPerAddr *big.Int
		if totalWeight.Cmp(big.NewInt(0)) == 0 {
			amountPerAddr = big.NewInt(0)
		} else {
			amountPerAddr = big.NewInt(0).Div(big.NewInt(0).Mul(totalAmount, candidate.Votes), totalWeight)
		}
		amounts = append(amounts, amountPerAddr)
	}
	return rewardAddrs, amounts, nil
}

func (p *Protocol) unqualifiedDelegates(
	producer address.Address,
	epochNum uint64,
	productivityThreshold uint64,
) (map[string]interface{}, error) {
	unqualifiedDelegates := make(map[string]interface{}, 0)
	numBlks, produce, err := p.cm.ProductivityByEpoch(epochNum)
	if err != nil {
		return nil, err
	}
	// The current block is not included, so that we need to add it to the stats
	numBlks++
	produce[producer.String()]++

	expectedNumBlks := numBlks / uint64(len(produce))
	for addr, actualNumBlks := range produce {
		if actualNumBlks*100/expectedNumBlks < productivityThreshold {
			unqualifiedDelegates[addr] = nil
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

func (p *Protocol) assertLastBlockInEpoch(blkHeight uint64, epochNum uint64) error {
	lastBlkHeight := p.rp.GetEpochLastBlockHeight(epochNum)
	if blkHeight != lastBlkHeight {
		return errors.Errorf("current block %d is not the last block of epoch %d", blkHeight, epochNum)
	}
	return nil
}
