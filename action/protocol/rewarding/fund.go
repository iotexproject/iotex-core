// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/state"
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
	totalBalance, ok := new(big.Int).SetString(gen.TotalBalance, 10)
	if !ok {
		return errors.New("failed to set total balance")
	}
	unclaimedBalance, ok := new(big.Int).SetString(gen.UnclaimedBalance, 10)
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
	transactionLogType iotextypes.TransactionLogType,
	opts ...protocol.Option,
) ([]*action.TransactionLog, error) {
	var (
		actionCtx           = protocol.MustGetActionCtx(ctx)
		accountCreationOpts = []state.AccountCreationOption{}
		options             = protocol.Options{}
	)
	// apply protocol options
	for _, o := range opts {
		o(&options)
	}
	if isZero(amount) && isZero(options.BurnAmount) {
		return nil, nil
	}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
	}
	// Subtract balance from caller
	acc, err := accountutil.LoadAccount(sm, actionCtx.Caller, accountCreationOpts...)
	if err != nil {
		return nil, err
	}
	if !isZero(amount) {
		if err := acc.SubBalance(amount); err != nil {
			return nil, err
		}
	}
	burnAmount := options.BurnAmount
	if !isZero(burnAmount) {
		if err := acc.SubBalance(burnAmount); err != nil {
			return nil, err
		}
	}
	if err := accountutil.StoreAccount(sm, actionCtx.Caller, acc); err != nil {
		return nil, err
	}
	// Add balance to fund
	var (
		f           = fund{}
		burnAddr, _ = address.FromString(address.ZeroAddress)
		tLog        = []*action.TransactionLog{}
	)
	if !isZero(amount) {
		tLog = append(tLog, &action.TransactionLog{
			Type:      transactionLogType,
			Sender:    actionCtx.Caller.String(),
			Recipient: address.RewardingPoolAddr,
			Amount:    amount,
		})
	}
	if _, err := p.state(ctx, sm, _fundKey, &f); err != nil {
		return nil, err
	}
	f.totalBalance = big.NewInt(0).Add(f.totalBalance, amount)
	f.unclaimedBalance = big.NewInt(0).Add(f.unclaimedBalance, amount)
	if !isZero(burnAmount) {
		// add burnAmount to burnAddr
		burn, err := accountutil.LoadAccount(sm, burnAddr, accountCreationOpts...)
		if err != nil {
			return nil, err
		}
		if err := burn.AddBalance(burnAmount); err != nil {
			return nil, err
		}
		if err := accountutil.StoreAccount(sm, burnAddr, burn); err != nil {
			return nil, err
		}
		tLog = append(tLog, &action.TransactionLog{
			Type:      options.BurnLogType,
			Sender:    actionCtx.Caller.String(),
			Recipient: burnAddr.String(),
			Amount:    burnAmount,
		})
	}
	if err := p.putState(ctx, sm, _fundKey, &f); err != nil {
		return nil, err
	}
	return tLog, nil
}

// TotalBalance returns the total balance of the rewarding fund
func (p *Protocol) TotalBalance(
	ctx context.Context,
	sm protocol.StateReader,
) (*big.Int, uint64, error) {
	f := fund{}
	height, err := p.state(ctx, sm, _fundKey, &f)
	if err != nil {
		return nil, height, err
	}
	return f.totalBalance, height, nil
}

// AvailableBalance returns the available balance of the rewarding fund
func (p *Protocol) AvailableBalance(
	ctx context.Context,
	sm protocol.StateReader,
) (*big.Int, uint64, error) {
	f := fund{}
	height, err := p.state(ctx, sm, _fundKey, &f)
	if err != nil {
		return nil, height, err
	}
	return f.unclaimedBalance, height, nil
}

// DepositGas deposits gas into the rewarding fund
func DepositGas(ctx context.Context, sm protocol.StateManager, amount *big.Int, opts ...protocol.Option) ([]*action.TransactionLog, error) {
	// TODO: we bypass the gas deposit for the actions in genesis block. Later we should remove this after we remove
	// genesis actions
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight == 0 {
		return nil, nil
	}
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, nil
	}
	rp := FindProtocol(reg)
	if rp == nil {
		return nil, nil
	}
	opt := &protocol.Options{}
	for _, o := range opts {
		o(opt)
	}
	logType := iotextypes.TransactionLogType_GAS_FEE
	if opt.GasLogType != nil {
		logType = *opt.GasLogType
	}
	return rp.Deposit(ctx, sm, amount, logType, opts...)
}

func isZero(a *big.Int) bool {
	return a == nil || len(a.Bytes()) == 0
}
