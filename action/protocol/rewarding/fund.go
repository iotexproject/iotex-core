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
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
		TotalBalance:     f.totalBalance.Bytes(),
		UnclaimedBalance: f.unclaimedBalance.Bytes(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into fund state
func (f *fund) Deserialize(data []byte) error {
	gen := rewardingpb.Fund{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	f.totalBalance = big.NewInt(0).SetBytes(gen.TotalBalance)
	f.unclaimedBalance = big.NewInt(0).SetBytes(gen.UnclaimedBalance)
	return nil
}

// Donate donates token into the rewarding fund
func (p *Protocol) Donate(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss run action context")
	}
	if err := p.assertEnoughBalance(raCtx, sm, amount); err != nil {
		return err
	}
	// Subtract balance from caller
	acc, err := account.LoadOrCreateAccount(sm, raCtx.Caller.Bech32(), big.NewInt(0))
	if err != nil {
		return err
	}
	acc.Balance = big.NewInt(0).Sub(acc.Balance, amount)
	account.StoreAccount(sm, raCtx.Caller.Bech32(), acc)
	// Add balance to fund
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return err
	}
	f.totalBalance = big.NewInt(0).Add(f.totalBalance, amount)
	f.unclaimedBalance = big.NewInt(0).Add(f.unclaimedBalance, amount)
	if err := p.putState(sm, fundKey, &f); err != nil {
		return err
	}
	return nil
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
	raCtx protocol.RunActionsCtx,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	acc, err := account.LoadAccount(sm, byteutil.BytesTo20B(raCtx.Caller.Payload()))
	if err != nil {
		return err
	}
	if acc.Balance.Cmp(amount) < 0 {
		return errors.New("balance is not enough for donation")
	}
	return nil
}
