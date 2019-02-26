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
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/enc"
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
		Balance: a.balance.Bytes(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into account state
func (a *rewardAccount) Deserialize(data []byte) error {
	gen := rewardingpb.Account{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	a.balance = big.NewInt(0).SetBytes(gen.Balance)
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
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	if err := p.updateAvailableBalance(sm, a.BlockReward); err != nil {
		return err
	}
	if err := p.grantToAccount(sm, raCtx.Producer, a.BlockReward); err != nil {
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
	epochNum := rolldpos.GetEpochNum(raCtx.BlockHeight, p.numDelegates, p.numSubEpochs)
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
	if err := p.updateAvailableBalance(sm, a.EpochReward); err != nil {
		return err
	}
	addrs, amounts, err := p.splitEpochReward(raCtx.BlockHeight, a.EpochReward)
	if err != nil {
		return err
	}
	for i := range addrs {
		if err := p.grantToAccount(sm, addrs[i], amounts[i]); err != nil {
			return err
		}
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
	} else if balance.Cmp(big.NewInt(0)) == 0 {
		// If the account balance is cleared, delete if from the store
		if err := p.deleteState(sm, accKey); err != nil {
			return err
		}
	} else {
		acc.balance = balance
		if err := p.putState(sm, accKey, &acc); err != nil {
			return err
		}
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

func (p *Protocol) splitEpochReward(blkHeight uint64, totalAmount *big.Int) ([]address.Address, []*big.Int, error) {
	candidates, err := p.cm.CandidatesByHeight(blkHeight)
	if err != nil {
		return nil, nil, err
	}
	totalWeight := big.NewInt(0)
	for _, candidate := range candidates {
		totalWeight = big.NewInt(0).Add(totalWeight, candidate.Votes)
	}
	addrs := make([]address.Address, 0)
	amounts := make([]*big.Int, 0)
	for _, candidate := range candidates {
		addr, err := address.FromString(candidate.Address)
		if err != nil {
			return nil, nil, err
		}
		addrs = append(addrs, addr)
		amountPerAddr := big.NewInt(0).Div(big.NewInt(0).Mul(totalAmount, candidate.Votes), totalWeight)
		amounts = append(amounts, amountPerAddr)
	}
	return addrs, amounts, nil
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
	lastBlkHeight := rolldpos.GetEpochLastBlockHeight(epochNum, p.numDelegates, p.numSubEpochs)
	if blkHeight != lastBlkHeight {
		return errors.Errorf("current block %d is not the last block of epoch %d", blkHeight, epochNum)
	}
	return nil
}
