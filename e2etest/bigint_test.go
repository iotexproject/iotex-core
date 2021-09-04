// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"github.com/iotexproject/iotex-address/address"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state/factory"
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
	bc, sf, ap := prepareBlockchain(ctx, executor, r)
	defer r.NoError(bc.Stop(ctx))
	executorAddress, err := address.FromString(executor)
	r.NoError(err)
	stateBeforeTransfer, err := accountutil.AccountState(sf, executorAddress)
	r.NoError(err)
	blk, err := prepareTransfer(bc, sf, ap, r)
	r.NoError(err)
	r.Equal(2, len(blk.Actions))
	r.Error(bc.ValidateBlock(blk))
	state, err := accountutil.AccountState(sf, executorAddress)
	r.NoError(err)
	r.Equal(0, state.Balance.Cmp(stateBeforeTransfer.Balance))
}

func TestAction_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc, sf, ap := prepareBlockchain(ctx, executor, r)
	defer r.NoError(bc.Stop(ctx))
	executorAddress, err := address.FromString(executor)
	r.NoError(err)
	stateBeforeTransfer, err := accountutil.AccountState(sf, executorAddress)
	r.NoError(err)
	blk, err := prepareAction(bc, sf, ap, r)
	r.NoError(err)
	r.NotNil(blk)
	r.Equal(2, len(blk.Actions))
	r.Error(bc.ValidateBlock(blk))
	state, err := accountutil.AccountState(sf, executorAddress)
	r.NoError(err)
	r.Equal(0, state.Balance.Cmp(stateBeforeTransfer.Balance))
}

func prepareBlockchain(ctx context.Context, executor string, r *require.Assertions) (blockchain.Blockchain, factory.Factory, actpool.ActPool) {
	cfg := config.Default
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.InitBalanceMap[executor] = "1000000000000000000000000000"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	r.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	r.NoError(err)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	r.NotNil(bc)
	reward := rewarding.NewProtocol(0, 0)
	r.NoError(reward.Register(registry))

	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	r.NoError(ep.Register(registry))
	r.NoError(bc.Start(ctx))
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)
	r.NoError(sf.Start(ctx))
	return bc, sf, ap
}

func prepareTransfer(bc blockchain.Blockchain, sf factory.Factory, ap actpool.ActPool, r *require.Assertions) (*block.Block, error) {
	exec, err := action.NewTransfer(1, big.NewInt(-10000), recipient, nil, uint64(1000000), big.NewInt(9000000000000))
	r.NoError(err)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	return prepare(bc, sf, ap, elp, r)
}

func prepareAction(bc blockchain.Blockchain, sf factory.Factory, ap actpool.ActPool, r *require.Assertions) (*block.Block, error) {
	exec, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(-100), uint64(1000000), big.NewInt(9000000000000), []byte{})
	r.NoError(err)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	return prepare(bc, sf, ap, elp, r)
}

func prepare(bc blockchain.Blockchain, sf factory.Factory, ap actpool.ActPool, elp action.Envelope, r *require.Assertions) (*block.Block, error) {
	priKey, err := crypto.HexStringToPrivateKey(executorPriKey)
	r.NoError(err)
	selp, err := action.Sign(elp, priKey)
	r.NoError(err)
	r.NoError(ap.Add(context.Background(), selp))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	r.NoError(err)
	// when validate/commit a blk, the workingset and receipts of blk should be nil
	blk.Receipts = nil
	return blk, nil
}
