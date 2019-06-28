// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/unit"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestNewGasStation(t *testing.T) {
	require := require.New(t)
	require.NotNil(NewGasStation(nil, config.Default.API))
}
func TestSuggestGasPriceForUserAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.Registry{}
	acc := account.NewProtocol(0)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	blkState := blockchain.InMemStateFactoryOption()
	blkMemDao := blockchain.InMemDaoOption()
	blkRegistryOption := blockchain.RegistryOption(&registry)
	bc := blockchain.NewBlockchain(cfg, blkState, blkMemDao, blkRegistryOption)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, 0, 0)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		tsf, err := action.NewTransfer(
			uint64(i)+1,
			big.NewInt(100),
			identityset.Address(27).String(),
			[]byte{}, uint64(100000),
			big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev)),
		)
		require.NoError(t, err)

		bd := &action.EnvelopeBuilder{}
		elp1 := bd.SetAction(tsf).
			SetNonce(uint64(i) + 1).
			SetGasLimit(100000).
			SetGasPrice(big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev))).Build()
		selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
		require.NoError(t, err)

		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp1}

		blk, err := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(t, err)
		require.Equal(t, 2, len(blk.Actions))
		require.Equal(t, 1, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		err = bc.ValidateBlock(blk)
		require.NoError(t, err)
		err = bc.CommitBlock(blk)
		require.NoError(t, err)
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, cfg.API)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, big.NewInt(1).Mul(big.NewInt(int64(31)), big.NewInt(unit.Qev)).Uint64(), gp)

	act := getActionWithContractCreate()
	require.NotNil(t, act)
	ret, err := gs.EstimateGasForAction(act)
	require.NoError(t, err)
	require.Equal(t, uint64(1000000), ret)
}

func TestSuggestGasPriceForSystemAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.Registry{}
	acc := account.NewProtocol(0)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	blkState := blockchain.InMemStateFactoryOption()
	blkMemDao := blockchain.InMemDaoOption()
	blkRegistryOption := blockchain.RegistryOption(&registry)
	bc := blockchain.NewBlockchain(cfg, blkState, blkMemDao, blkRegistryOption)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, 0, 0)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		actionMap := make(map[string][]action.SealedEnvelope)

		blk, err := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(t, err)
		require.Equal(t, 1, len(blk.Actions))
		require.Equal(t, 0, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		err = bc.ValidateBlock(blk)
		require.NoError(t, err)
		err = bc.CommitBlock(blk)
		require.NoError(t, err)
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, cfg.API)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	fmt.Println(gp)
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, gs.cfg.GasStation.DefaultGas, gp)
}

func TestEstimateGasForAction(t *testing.T) {
	require := require.New(t)
	act := getAction()
	require.NotNil(act)
	cfg := config.Default
	bc := blockchain.NewBlockchain(cfg, blockchain.InMemDaoOption(), blockchain.InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	gs := NewGasStation(bc, config.Default.API)
	require.NotNil(gs)
	ret, err := gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000
	require.Equal(uint64(10000), ret)

	// test for payload
	act = getActionWithPayload()
	require.NotNil(act)
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	ret, err = gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000,plus data size*ExecutionDataGas
	require.Equal(uint64(10000)+10*action.ExecutionDataGas, ret)
}
func getAction() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
	}
	return
}
func getActionWithPayload() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2, Payload: []byte("1234567890")},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
	}
	return
}
func getActionWithContractCreate() (act *iotextypes.Action) {
	//pubKey1 := identityset.PrivateKey(20).PublicKey()
	pubKey1 := identityset.PrivateKey(0)
	exec, _ := action.NewExecution(
		"",
		30,
		big.NewInt(0),
		10000,
		big.NewInt(10),
		[]byte("608060405234801561001057600080fd5b50610205806100206000396000f30060806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806370a0823114610051578063a9059cbb146100a8575b600080fd5b34801561005d57600080fd5b50610092600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506100f5565b6040518082815260200191505060405180910390f35b3480156100b457600080fd5b506100f3600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061013d565b005b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282540392505081905550806000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555050505600a165627a7a72305820bf5914d696d0145114f3d3d789bc6aef8493d1dbeaa1320b0583e28bfe50b8cb0029"),
	)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	selp, _ := action.Sign(elp, pubKey1)

	act = &iotextypes.Action{
		Core:         selp.Proto().Core,
		SenderPubKey: pubKey1.PublicKey().Bytes(),
	}
	return
}
