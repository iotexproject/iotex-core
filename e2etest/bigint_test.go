// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	executor       = "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"
	recipient      = "io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"
	executorPriKey = "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1"
)

func TestTransfer_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc := prepareBlockchain(ctx, executor, r)
	defer r.NoError(bc.Stop(ctx))
	balanceBeforeTransfer, err := bc.Balance(executor)
	r.NoError(err)
	blk, err := prepareTransfer(bc, r)
	r.NoError(err)
	err = bc.ValidateBlock(blk)
	r.Error(err)
	err = bc.CommitBlock(blk)
	r.NoError(err)
	balance, err := bc.Balance(executor)
	r.NoError(err)
	r.Equal(0, balance.Cmp(balanceBeforeTransfer))
}
func TestAction_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc := prepareBlockchain(ctx, executor, r)
	defer r.NoError(bc.Stop(ctx))
	balanceBeforeTransfer, err := bc.Balance(executor)
	r.NoError(err)
	blk, err := prepareAction(bc, r)
	r.NoError(err)
	r.NotNil(blk)
	err = bc.ValidateBlock(blk)
	r.Error(err)
	err = bc.CommitBlock(blk)
	r.NoError(err)
	balance, err := bc.Balance(executor)
	r.NoError(err)
	r.Equal(-1, balance.Cmp(balanceBeforeTransfer))
}

func prepareBlockchain(
	ctx context.Context, executor string, r *require.Assertions) blockchain.Blockchain {
	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	registry.Register(account.ProtocolID, acc)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	registry.Register(rolldpos.ProtocolID, rp)
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemDaoOption(),
		blockchain.InMemStateFactoryOption(),
		blockchain.RegistryOption(&registry),
	)
	r.NotNil(bc)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(account.NewProtocol(), execution.NewProtocol(bc))
	sf := bc.GetFactory()
	r.NotNil(sf)
	sf.AddActionHandlers(execution.NewProtocol(bc))
	r.NoError(bc.Start(ctx))
	ws, err := sf.NewWorkingSet()
	r.NoError(err)
	balance, ok := new(big.Int).SetString("1000000000000000000000000000", 10)
	r.True(ok)
	_, err = accountutil.LoadOrCreateAccount(ws, executor, balance)
	r.NoError(err)

	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: uint64(10000000),
		})
	_, err = ws.RunActions(ctx, 0, nil)
	r.NoError(err)
	r.NoError(sf.Commit(ws))
	return bc
}
func prepareTransfer(bc blockchain.Blockchain, r *require.Assertions) (*block.Block, error) {
	exec, err := action.NewTransfer(1, big.NewInt(-10000), recipient, nil, uint64(1000000), big.NewInt(9000000000000))
	r.NoError(err)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	return prepare(bc, elp, r)
}
func prepareAction(bc blockchain.Blockchain, r *require.Assertions) (*block.Block, error) {
	exec, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(-100), uint64(1000000), big.NewInt(9000000000000), []byte{})
	r.NoError(err)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	return prepare(bc, elp, r)
}
func prepare(bc blockchain.Blockchain, elp action.Envelope, r *require.Assertions) (*block.Block, error) {
	priKey, err := crypto.HexStringToPrivateKey(executorPriKey)
	selp, err := action.Sign(elp, priKey)
	r.NoError(err)
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[executor] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	r.NoError(err)
	return blk, nil
}
