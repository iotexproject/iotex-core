// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"time"

	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// createGenesisStates initialize the genesis states
func createGenesisStates(cfg config.Config, registry *protocol.Registry, ws WorkingSet) error {
	if cfg.Chain.EmptyGenesis {
		return nil
	}
	if registry == nil {
		// TODO: return nil to avoid test cases to blame on missing rewarding protocol
		return nil
	}
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockHeight:    0,
		BlockTimeStamp: time.Unix(cfg.Genesis.Timestamp, 0),
		GasLimit:       0,
		Producer:       nil,
		Caller:         nil,
		ActionHash:     hash.ZeroHash256,
		Nonce:          0,
		Registry:       registry,
	})
	if err := createAccountGenesisStates(ctx, ws, registry, cfg); err != nil {
		return err
	}
	if cfg.Consensus.Scheme == config.RollDPoSScheme {
		if err := createPollGenesisStates(ctx, ws, registry, cfg); err != nil {
			return err
		}
	}
	if cfg.Genesis.NativeStakingContractCode != "" {
		if err := createNativeStakingContract(ctx, ws, registry, cfg); err != nil {
			return err
		}
	}
	if err := createRewardingGenesisStates(ctx, ws, registry, cfg); err != nil {
		return err
	}
	_ = ws.UpdateBlockLevelInfo(0)
	return nil
}

func createAccountGenesisStates(ctx context.Context, sm protocol.StateManager, registry *protocol.Registry, cfg config.Config) error {
	p, ok := registry.Find(account.ProtocolID)
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

func createRewardingGenesisStates(ctx context.Context, sm protocol.StateManager, registry *protocol.Registry, cfg config.Config) error {
	p, ok := registry.Find(rewarding.ProtocolID)
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

func createPollGenesisStates(ctx context.Context, sm protocol.StateManager, registry *protocol.Registry, cfg config.Config) error {
	if cfg.Genesis.EnableGravityChainVoting {
		p, ok := registry.Find(poll.ProtocolID)
		if !ok {
			return errors.Errorf("protocol %s is not found", poll.ProtocolID)
		}
		pp, ok := p.(poll.Protocol)
		if !ok {
			return errors.Errorf("error when casting poll protocol")
		}
		return pp.Initialize(ctx, sm)
	}
	return nil
}

func createNativeStakingContract(ctx context.Context, sm protocol.StateManager, registry *protocol.Registry, cfg config.Config) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	raCtx.Producer, _ = address.FromString(address.ZeroAddress)
	raCtx.Caller, _ = address.FromString(address.ZeroAddress)
	raCtx.GasLimit = cfg.Genesis.BlockGasLimit
	bytes, err := hexutil.Decode(cfg.Genesis.NativeStakingContractCode)
	if err != nil {
		return err
	}
	hu := config.NewHeightUpgrade(cfg)
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
		hu,
	)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying native staking contract, status=%d", receipt.Status)
	}
	p, ok := registry.Find(poll.ProtocolID)
	if ok {
		pp, ok := p.(poll.Protocol)
		if ok {
			pp.SetNativeStakingContract(receipt.ContractAddress)
			log.L().Info("Deployed native staking contract", zap.String("address", receipt.ContractAddress))
		}
	}
	return nil
}
