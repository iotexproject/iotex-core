// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// createGenesisStates initialize the genesis states
func createGenesisStates(ctx context.Context, cfg config.Config, ws WorkingSet) error {
	if cfg.Chain.EmptyGenesis {
		return nil
	}
	if err := createAccountGenesisStates(ctx, ws, cfg); err != nil {
		return err
	}
	if cfg.Consensus.Scheme == config.RollDPoSScheme && cfg.Genesis.EnableGravityChainVoting {
		if err := createPollGenesisStates(ctx, ws); err != nil {
			return err
		}
	}
	if cfg.Genesis.NativeStakingContractCode != "" {
		if err := createNativeStakingContract(ctx, ws, cfg); err != nil {
			return err
		}
	}
	if err := createRewardingGenesisStates(ctx, ws, cfg); err != nil {
		return err
	}
	_ = ws.UpdateBlockLevelInfo(0)

	return nil
}

func createAccountGenesisStates(ctx context.Context, sm protocol.StateManager, cfg config.Config) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	p, ok := raCtx.Registry.Find(account.ProtocolID)
	if !ok {
		return nil
	}
	ap, ok := p.(*account.Protocol)
	if !ok {
		return errors.Errorf("error when casting protocol")
	}
	addrs, balances := cfg.Genesis.InitBalances()
	return ap.Initialize(ctx, sm, addrs, balances)
}

func createRewardingGenesisStates(ctx context.Context, sm protocol.StateManager, cfg config.Config) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	p, ok := raCtx.Registry.Find(rewarding.ProtocolID)
	if !ok {
		return nil
	}
	rp, ok := p.(*rewarding.Protocol)
	if !ok {
		return errors.Errorf("error when casting protocol")
	}
	return rp.Initialize(
		ctx,
		sm,
		cfg.Genesis.InitBalance(),
		cfg.Genesis.BlockReward(),
		cfg.Genesis.EpochReward(),
		cfg.Genesis.NumDelegatesForEpochReward,
		cfg.Genesis.ExemptAddrsFromEpochReward(),
		cfg.Genesis.FoundationBonus(),
		cfg.Genesis.NumDelegatesForFoundationBonus,
		cfg.Genesis.FoundationBonusLastEpoch,
		cfg.Genesis.ProductivityThreshold,
	)
}

func createPollGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	p, ok := raCtx.Registry.Find(poll.ProtocolID)
	if !ok {
		return errors.Errorf("protocol %s is not found", poll.ProtocolID)
	}
	pp, ok := p.(poll.Protocol)
	if !ok {
		return errors.Errorf("error when casting poll protocol")
	}
	return pp.Initialize(ctx, sm)
}

func createNativeStakingContract(ctx context.Context, sm protocol.StateManager, cfg config.Config) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	p, ok := raCtx.Registry.Find(poll.ProtocolID)
	if !ok {
		return nil
	}

	raCtx.Producer, _ = address.FromString(address.ZeroAddress)
	raCtx.Caller, _ = address.FromString(address.ZeroAddress)
	raCtx.GasLimit = cfg.Genesis.BlockGasLimit
	raCtx.Genesis = cfg.Genesis
	bytes, err := hexutil.Decode(cfg.Genesis.NativeStakingContractCode)
	if err != nil {
		return err
	}
	execution, err := action.NewExecution(
		"",
		0,
		big.NewInt(0),
		cfg.Genesis.BlockGasLimit,
		big.NewInt(0),
		bytes,
	)
	if err != nil {
		return err
	}
	_, receipt, err := evm.ExecuteContract(
		protocol.WithRunActionsCtx(ctx, raCtx),
		sm,
		execution,
		func(height uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
	)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying native staking contract, status=%d", receipt.Status)
	}
	pp, ok := p.(poll.Protocol)
	if ok {
		pp.SetNativeStakingContract(receipt.ContractAddress)
		log.L().Info("Deployed native staking contract", zap.String("address", receipt.ContractAddress))
	}
	return nil
}

// CreateTestAccount adds a new account with initial balance to the factory for test usage
func CreateTestAccount(sf Factory, cfg config.Config, registry *protocol.Registry, addr string, init *big.Int) (*state.Account, error) {
	gasLimit := cfg.Genesis.BlockGasLimit
	if sf == nil {
		return nil, errors.New("empty state factory")
	}

	ws, err := sf.NewWorkingSet()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create clean working set")
	}

	account, err := accountutil.LoadOrCreateAccount(ws, addr, init)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new account %s", addr)
	}

	callerAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			GasLimit:   gasLimit,
			Caller:     callerAddr,
			ActionHash: hash.ZeroHash256,
			Nonce:      0,
			Registry:   registry,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run the account creation")
	}

	if err = sf.Commit(ws); err != nil {
		return nil, errors.Wrap(err, "failed to commit the account creation")
	}

	return account, nil
}
