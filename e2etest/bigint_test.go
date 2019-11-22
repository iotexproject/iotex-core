// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
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
	defer bc.Stop(ctx)
	balanceBeforeTransfer, err := bc.Factory().Balance(executor)
	r.NoError(err)
	blk, err := prepareTransfer(bc, r)
	r.NoError(err)
	r.Error(bc.ValidateBlock(blk))
	r.Panics(func() { bc.CommitBlock(blk) })
	balance, err := bc.Factory().Balance(executor)
	r.NoError(err)
	r.Equal(0, balance.Cmp(balanceBeforeTransfer))
}
func TestAction_Negative(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc := prepareBlockchain(ctx, executor, r)
	defer bc.Stop(ctx)
	balanceBeforeTransfer, err := bc.Factory().Balance(executor)
	r.NoError(err)
	blk, err := prepareAction(bc, r)
	r.NoError(err)
	r.NotNil(blk)
	r.Error(bc.ValidateBlock(blk))
	r.Panics(func() { bc.CommitBlock(blk) })
	balance, err := bc.Factory().Balance(executor)
	r.NoError(err)
	r.Equal(0, balance.Cmp(balanceBeforeTransfer))
}

func prepareBlockchain(ctx context.Context, executor string, r *require.Assertions) blockchain.Blockchain {
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")

	cfg := newConfig(testDBFile.Name(), testTrieFile.Name(), testIndexFile.Name(), identityset.PrivateKey(27),
		4689, 14014, uint64(24))
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	r.NoError(registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(registry.Register(rolldpos.ProtocolID, rp))

	dbConfig := cfg.DB
	sf, err := factory.NewStateDB(cfg, factory.DefaultStateDBOption())
	r.NoError(err)
	// create indexer
	dbConfig.DbPath = cfg.Chain.IndexDBPath
	indexer, err := blockindex.NewIndexer(db.NewBoltDB(dbConfig), cfg.Genesis.Hash())
	r.NoError(err)
	// create BlockDAO
	dbConfig.DbPath = cfg.Chain.ChainDBPath
	dao := blockdao.NewBlockDAO(db.NewBoltDB(dbConfig), indexer, cfg.Chain.CompressBlock, dbConfig)
	r.NotNil(dao)
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.RegistryOption(&registry),
	)
	r.NotNil(bc)
	reward := rewarding.NewProtocol(bc, rp)
	r.NoError(registry.Register(rewarding.ProtocolID, reward))
	p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	r.NoError(registry.Register(poll.ProtocolID, p))

	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc.Factory().Nonce))
	exec := execution.NewProtocol(bc.BlockDAO().GetBlockHash)
	r.NoError(registry.Register(execution.ProtocolID, exec))
	r.NoError(bc.Start(ctx))
	ws, err := sf.NewWorkingSet(&registry)
	r.NoError(err)
	balance, ok := new(big.Int).SetString("1000000000000000000000000000", 10)
	r.True(ok)
	_, err = accountutil.LoadOrCreateAccount(ws, executor, balance)
	r.NoError(err)

	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: uint64(10000000),
			Genesis:  cfg.Genesis,
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
	r.NoError(err)
	selp, err := action.Sign(elp, priKey)
	r.NoError(err)
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[executor] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	r.NoError(err)
	// when validate/commit a blk, the workingset and receipts of blk should be nil
	blk.WorkingSet = nil
	blk.Receipts = nil
	return blk, nil
}
