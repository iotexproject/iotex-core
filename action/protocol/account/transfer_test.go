// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestProtocol_HandleTransfer(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	ctx := context.Background()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()

	p := NewProtocol()
	reward := rewarding.NewProtocol(nil, rolldpos.NewProtocol(1, 1, 1))
	registry := protocol.NewRegistry()
	require.NoError(registry.Register(rewarding.ProtocolID, reward))
	cfg.Genesis.Rewarding.InitBalanceStr = "0"
	cfg.Genesis.Rewarding.BlockRewardStr = "0"
	cfg.Genesis.Rewarding.EpochRewardStr = "0"
	cfg.Genesis.Rewarding.NumDelegatesForEpochReward = 1
	cfg.Genesis.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	cfg.Genesis.Rewarding.FoundationBonusStr = "0"
	cfg.Genesis.Rewarding.NumDelegatesForFoundationBonus = 0
	cfg.Genesis.Rewarding.FoundationBonusLastEpoch = 0
	cfg.Genesis.Rewarding.ProductivityThreshold = 0
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Registry: registry,
			Genesis:  cfg.Genesis,
		})
	ctx = protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		})
	ctx = protocol.WithActionCtx(ctx,
		protocol.ActionCtx{
			Caller: identityset.Address(28),
		})
	require.NoError(
		reward.CreateGenesisStates(
			ctx,
			sm,
		),
	)

	accountAlfa := state.Account{
		Balance: big.NewInt(50005),
	}
	accountBravo := state.Account{}
	accountCharlie := state.Account{}
	pubKeyAlfa := hash.BytesToHash160(identityset.Address(28).Bytes())
	pubKeyBravo := hash.BytesToHash160(identityset.Address(29).Bytes())
	pubKeyCharlie := hash.BytesToHash160(identityset.Address(30).Bytes())

	require.NoError(sm.PutState(pubKeyAlfa, &accountAlfa))
	require.NoError(sm.PutState(pubKeyBravo, &accountBravo))
	require.NoError(sm.PutState(pubKeyCharlie, &accountCharlie))

	transfer, err := action.NewTransfer(
		uint64(1),
		big.NewInt(2),
		identityset.Address(29).String(),
		[]byte{},
		uint64(10000),
		big.NewInt(1),
	)
	require.NoError(err)
	gas, err := transfer.IntrinsicGas()
	require.NoError(err)

	ctx = protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       identityset.Address(28),
		IntrinsicGas: gas,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: 1,
		Producer:    identityset.Address(27),
		GasLimit:    testutil.TestGasLimit,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Registry: registry,
	})

	receipt, err := p.Handle(ctx, transfer, sm)
	require.NoError(err)
	require.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)

	var acct state.Account
	require.NoError(sm.State(pubKeyAlfa, &acct))
	require.Equal("40003", acct.Balance.String())
	require.Equal(uint64(1), acct.Nonce)
	require.NoError(sm.State(pubKeyBravo, &acct))
	require.Equal("2", acct.Balance.String())

	contractAcct := state.Account{
		CodeHash: []byte("codeHash"),
	}
	contractAddr := hash.BytesToHash160(identityset.Address(32).Bytes())
	require.NoError(sm.PutState(contractAddr, &contractAcct))
	transfer, err = action.NewTransfer(
		uint64(2),
		big.NewInt(3),
		identityset.Address(32).String(),
		[]byte{},
		uint64(10000),
		big.NewInt(2),
	)
	require.NoError(err)
	// Assume that the gas of this transfer is the same as previous one
	receipt, err = p.Handle(ctx, transfer, sm)
	require.NoError(err)
	require.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
	require.NoError(sm.State(pubKeyAlfa, &acct))
	require.Equal(uint64(2), acct.Nonce)
	require.Equal("20003", acct.Balance.String())
}

func TestProtocol_ValidateTransfer(t *testing.T) {
	require := require.New(t)
	protocol := NewProtocol()
	// Case I: Oversized data
	tmpPayload := [32769]byte{}
	payload := tmpPayload[:]
	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), "2", payload, uint64(0),
		big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Negative amount
	tsf, err = action.NewTransfer(uint64(1), big.NewInt(-100), "2", nil,
		uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case III: Invalid recipient address
	tsf, err = action.NewTransfer(
		1,
		big.NewInt(1),
		identityset.Address(28).String()+"aaa",
		nil,
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
	// Case IV: Negative gas fee
	tsf, err = action.NewTransfer(uint64(1), big.NewInt(100), "2", nil,
		uint64(100000), big.NewInt(-1))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrGasPrice, errors.Cause(err))
}
