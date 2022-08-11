// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/iotexproject/iotex-address/address"

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
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_executor       = "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"
	_recipient      = "io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"
	_executorPriKey = "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1"
)

func TestTransfer_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc, sf, ap := prepareBlockchain(ctx, _executor, r)
	defer r.NoError(bc.Stop(ctx))
	ctx = genesis.WithGenesisContext(ctx, bc.Genesis())
	addr, err := address.FromString(_executor)
	r.NoError(err)
	stateBeforeTransfer, err := accountutil.AccountState(ctx, sf, addr)
	r.NoError(err)
	blk, err := prepareTransfer(bc, sf, ap, r)
	r.NoError(err)
	r.Equal(1, len(blk.Actions))
	r.NoError(bc.ValidateBlock(blk))
	state, err := accountutil.AccountState(ctx, sf, addr)
	r.NoError(err)
	r.Equal(0, state.Balance.Cmp(stateBeforeTransfer.Balance))
}

func TestAction_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc, sf, ap := prepareBlockchain(ctx, _executor, r)
	defer r.NoError(bc.Stop(ctx))
	addr, err := address.FromString(_executor)
	r.NoError(err)
	ctx = genesis.WithGenesisContext(ctx, bc.Genesis())
	stateBeforeTransfer, err := accountutil.AccountState(ctx, sf, addr)
	r.NoError(err)
	blk, err := prepareAction(bc, sf, ap, r)
	r.NoError(err)
	r.NotNil(blk)
	r.Equal(1, len(blk.Actions))
	r.NoError(bc.ValidateBlock(blk))
	state, err := accountutil.AccountState(ctx, sf, addr)
	r.NoError(err)
	r.Equal(0, state.Balance.Cmp(stateBeforeTransfer.Balance))
}

func prepareBlockchain(ctx context.Context, _executor string, r *require.Assertions) (blockchain.Blockchain, factory.Factory, actpool.ActPool) {
	cfg := config.Default
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.InitBalanceMap[_executor] = "1000000000000000000000000000"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))
	factoryCfg := factory.Config{
		DB:      cfg.DB,
		Chain:   cfg.Chain,
		Genesis: cfg.Genesis,
	}
	sf, err := factory.NewFactory(factoryCfg, db.NewMemKVStore(), factory.RegistryOption(registry))
	r.NoError(err)
	genericValidator := protocol.NewGenericValidator(sf, accountutil.AccountState)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	r.NoError(err)
	ap.AddActionEnvelopeValidators(genericValidator)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			genericValidator,
		)),
	)
	r.NotNil(bc)
	reward := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(reward.Register(registry))

	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	r.NoError(ep.Register(registry))
	r.NoError(bc.Start(ctx))
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)
	r.NoError(sf.Start(ctx))
	return bc, sf, ap
}

func prepareTransfer(bc blockchain.Blockchain, sf factory.Factory, ap actpool.ActPool, r *require.Assertions) (*block.Block, error) {
	exec, err := action.NewTransfer(1, big.NewInt(-10000), _recipient, nil, uint64(1000000), big.NewInt(9000000000000))
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
	priKey, err := crypto.HexStringToPrivateKey(_executorPriKey)
	r.NoError(err)
	selp, err := action.Sign(elp, priKey)
	r.NoError(err)
	r.Error(ap.Add(context.Background(), selp))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	r.NoError(err)
	// when validate/commit a blk, the workingset and receipts of blk should be nil
	blk.Receipts = nil
	return blk, nil
}
