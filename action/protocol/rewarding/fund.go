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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
)

// fund stores the balance of the rewarding fund. The difference between total and available balance should be
// equal to the unclaimed balance in all reward accounts
type fund struct {
	totalBalance     *big.Int
	unclaimedBalance *big.Int
}

// Serialize serializes fund state into bytes
func (f fund) Serialize() ([]byte, error) {
	gen := rewardingpb.Fund{
		TotalBalance:     f.totalBalance.String(),
		UnclaimedBalance: f.unclaimedBalance.String(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into fund state
func (f *fund) Deserialize(data []byte) error {
	gen := rewardingpb.Fund{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	totalBalance, ok := big.NewInt(0).SetString(gen.TotalBalance, 10)
	if !ok {
		return errors.New("failed to set total balance")
	}
	unclaimedBalance, ok := big.NewInt(0).SetString(gen.UnclaimedBalance, 10)
	if !ok {
		return errors.New("failed to set unclaimed balance")
	}
	f.totalBalance = totalBalance
	f.unclaimedBalance = unclaimedBalance
	return nil
}

// Deposit deposits token into the rewarding fund
func (p *Protocol) Deposit(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	actionCtx := protocol.MustGetActionCtx(ctx)
	if err := p.assertAmount(amount); err != nil {
		return err
	}
	if err := p.assertEnoughBalance(actionCtx, sm, amount); err != nil {
		return err
	}
	// Subtract balance from caller
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return err
	}
	acc.Balance = big.NewInt(0).Sub(acc.Balance, amount)
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), acc); err != nil {
		return err
	}
	// Add balance to fund
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return err
	}
	f.totalBalance = big.NewInt(0).Add(f.totalBalance, amount)
	f.unclaimedBalance = big.NewInt(0).Add(f.unclaimedBalance, amount)
	return p.putState(sm, fundKey, &f)
}

// TotalBalance returns the total balance of the rewarding fund
func (p *Protocol) TotalBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return nil, err
	}
	return f.totalBalance, nil
}

// AvailableBalance returns the available balance of the rewarding fund
func (p *Protocol) AvailableBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return nil, err
	}
	return f.unclaimedBalance, nil
}

func (p *Protocol) assertEnoughBalance(
	actionCtx protocol.ActionCtx,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return err
	}
	if acc.Balance.Cmp(amount) < 0 {
		return errors.New("balance is not enough for donation")
	}
	return nil
}

// DepositGas deposits gas into the rewarding fund
func DepositGas(ctx context.Context, sm protocol.StateManager, amount *big.Int) error {
	// If the gas fee is 0, return immediately
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}
	// TODO: we bypass the gas deposit for the actions in genesis block. Later we should remove this after we remove
	// genesis actions
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight == 0 {
		return nil
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if bcCtx.Registry == nil {
		return nil
	}
	rp := FindProtocol(bcCtx.Registry)
	if rp == nil {
		return nil
	}
	return rp.Deposit(ctx, sm, amount)
}
