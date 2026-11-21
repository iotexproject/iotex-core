// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// These tests run a proposer and an independent validator over the same chain,
// with all forks through Zanzibar Gamma active from height 1. Block 1 registers
// cand1 and has _ccVoter vote for it; block 2 registers cand2.
var (
	_ccOwner     = identityset.Address(20) // cand1's owner
	_ccOperator  = identityset.Address(21) // cand1's operator
	_ccOwner2    = identityset.Address(22) // cand2's owner
	_ccOperator2 = identityset.Address(23) // cand2's operator
	_ccNewOwner  = identityset.Address(24) // receives cand1
	_ccVoter     = identityset.Address(25) // owns the vote bucket
	_ccNewOp     = identityset.Address(26) // a new operator
	_ccProducer  = identityset.PrivateKey(27)
)

// _ccBucket is the voter's bucket; 0 and 2 are the two self-stakes.
const _ccBucket = uint64(1)

type capChain struct {
	t         *testing.T
	cfg       Config
	proposer  Factory
	validator Factory
	tip       protocol.TipInfo
}

func capConfig() Config {
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.ZanzibarBlockHeight = 1
	cfg.Genesis.ZanzibarBetaBlockHeight = 1
	cfg.Genesis.ZanzibarGammaBlockHeight = 1
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return cfg
}

func newCapFactory(t *testing.T, cfg Config) Factory {
	r := require.New(t)
	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	sp, err := staking.NewProtocol(
		staking.HelperCtx{
			DepositGas:    rewarding.DepositGas,
			BlockInterval: func(uint64) time.Duration { return 5 * time.Second },
		},
		&staking.BuilderConfig{
			Staking:                       cfg.Genesis.Staking,
			PersistStakingPatchBlock:      math.MaxUint64,
			SkipContractStakingViewHeight: math.MaxUint64,
			Revise: staking.ReviseConfig{
				VoteWeight: cfg.Genesis.Staking.VoteWeightCalConsts,
			},
		},
		nil, nil, nil, nil,
	)
	r.NoError(err)
	r.NoError(sp.Register(registry))

	dbPath, err := testutil.PathOfTempFile("candidate-cap-statedb")
	r.NoError(err)
	t.Cleanup(func() { testutil.CleanupPath(dbPath) })
	cfg.Chain.TrieDBPath = dbPath
	cfg.Chain.TrieDBPatchFile = ""
	kv, err := db.CreateKVStoreWithCache(db.DefaultConfig, dbPath, cfg.Chain.StateDBCacheSize)
	r.NoError(err)
	f, err := NewStateDB(cfg, kv, RegistryStateDBOption(registry))
	r.NoError(err)
	startCtx := protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)))
	r.NoError(f.Start(startCtx))
	t.Cleanup(func() { r.NoError(f.Stop(startCtx)) })
	return f
}

// newCapChain boots the proposer and the validator and commits
// blocks 1 and 2 on both.
func newCapChain(t *testing.T) *capChain {
	r := require.New(t)
	cfg := capConfig()
	c := &capChain{
		t:         t,
		cfg:       cfg,
		proposer:  newCapFactory(t, cfg),
		validator: newCapFactory(t, cfg),
	}
	selfStake := unit.ConvertIotxToRau(1200000).String()
	reg1, err := action.NewCandidateRegister("cand1", _ccOperator.String(), _ccOwner.String(), _ccOwner.String(), selfStake, 0, false, nil)
	r.NoError(err)
	reg2, err := action.NewCandidateRegister("cand2", _ccOperator2.String(), _ccOwner2.String(), _ccOwner2.String(), selfStake, 0, false, nil)
	r.NoError(err)
	stake, err := action.NewCreateStake("cand1", unit.ConvertIotxToRau(10000).String(), 0, false, nil)
	r.NoError(err)
	// the stake is priced lower so it runs after cand1 is registered
	blk, _ := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String(): {c.sign(_ccOwner, env(0, 3).SetAction(reg1).Build())},
		_ccVoter.String(): {c.sign(_ccVoter, env(0, 1).SetAction(stake).Build())},
	})
	r.Len(blockHashes(t, blk), 2)
	blk2, _ := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner2.String(): {c.sign(_ccOwner2, env(0, 3).SetAction(reg2).Build())},
	})
	r.Len(blockHashes(t, blk2), 1)
	for _, receipt := range append(blk.Receipts, blk2.Receipts...) {
		r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)
	}
	bucket, err := c.candidates(c.validator).NativeBucket(_ccBucket)
	r.NoError(err)
	r.Equal(_ccVoter.String(), bucket.Owner.String())
	return c
}

func (c *capChain) ctx() context.Context {
	height := c.tip.Height + 1
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight:    height,
		BlockTimeStamp: time.Unix(c.cfg.Genesis.Timestamp, 0).Add(time.Duration(height) * time.Second),
		Producer:       _ccProducer.PublicKey().Address(),
		GasLimit:       testutil.TestGasLimit * 100000,
		BaseFee:        big.NewInt(action.InitialBaseFee),
	})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, c.cfg.Genesis),
		protocol.BlockchainCtx{ChainID: 1, Tip: c.tip},
	)
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

// env starts an envelope priced at price times the base fee; the action
// iterator picks by price, which fixes the order within a block.
func env(nonce uint64, price int64) *action.EnvelopeBuilder {
	return (&action.EnvelopeBuilder{}).
		SetGasLimit(1000000).
		SetGasPrice(new(big.Int).Mul(big.NewInt(price), big.NewInt(action.InitialBaseFee))).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1)
}

func (c *capChain) key(sender address.Address) crypto.PrivateKey {
	for i := 0; i < identityset.Size(); i++ {
		if identityset.Address(i).String() == sender.String() {
			return identityset.PrivateKey(i)
		}
	}
	c.t.Fatalf("no key for %s", sender.String())
	return nil
}

func (c *capChain) sign(sender address.Address, elp action.Envelope) *action.SealedEnvelope {
	selp, err := action.Sign(elp, c.key(sender))
	require.NoError(c.t, err)
	return selp
}

// container wraps a staking action as an eth tx in a tx container, the only
// way a staking action can go into a bundle.
func (c *capChain) container(sender address.Address, nonce uint64, act interface{ EthData() ([]byte, error) }) *action.SealedEnvelope {
	r := require.New(c.t)
	staking, err := address.FromString(address.StakingProtocolAddr)
	r.NoError(err)
	to := common.BytesToAddress(staking.Bytes())
	data, err := act.EthData()
	r.NoError(err)
	signer := types.NewEIP155Signer(big.NewInt(int64(c.cfg.Chain.EVMNetworkID)))
	tx, err := types.SignNewTx(c.key(sender).EcdsaPrivateKey().(*ecdsa.PrivateKey), signer, &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(action.InitialBaseFee * 2),
		Gas:      1000000,
		To:       &to,
		Value:    big.NewInt(0),
		Data:     data,
	})
	r.NoError(err)
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	r.NoError(err)
	raw, err := tx.MarshalBinary()
	r.NoError(err)
	core, err := action.EthRawToContainer(1, hex.EncodeToString(raw))
	r.NoError(err)
	selp, err := (&action.Deserializer{}).SetEvmNetworkID(c.cfg.Chain.EVMNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         core,
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	})
	r.NoError(err)
	return selp
}

// bundles puts each bundle into a fresh bundle pool for the next block.
func (c *capChain) bundles(bundles ...[]*action.SealedEnvelope) *actpool.BundlePool {
	bp := actpool.NewBundlePool(c.cfg.Genesis)
	bp.SetMaxLookahead(math.MaxUint64)
	for i, acts := range bundles {
		b := action.NewBundle()
		require.NoError(c.t, b.Add(acts...))
		b.SetTargetBlockHeight(c.tip.Height + 1)
		require.NoError(c.t, bp.AddBundle(context.Background(), _ccProducer.PublicKey().Address(), fmt.Sprintf("b%d", i), b))
	}
	return bp
}

// mintAndCommit has the proposer mint the next block from pending, which the
// mock pool hands out once, then has the validator validate it on its own
// working set as a peer would, and commits it on both. It returns the block
// with the validator's receipts. The mock pool fails the test if the proposer
// evicts any sender.
func (c *capChain) mintAndCommit(pending map[string][]*action.SealedEnvelope) (*block.Block, map[hash.Hash256]*action.Receipt) {
	return c.mintWithBundles(nil, pending)
}

func (c *capChain) mintWithBundles(bp *actpool.BundlePool, pending map[string][]*action.SealedEnvelope) (*block.Block, map[hash.Hash256]*action.Receipt) {
	r := require.New(c.t)
	ctx := c.ctx()
	ap := mock_actpool.NewMockActPool(gomock.NewController(c.t))
	ap.EXPECT().BundlePool().Return(bp).Times(1)
	ap.EXPECT().PendingActionMap().Return(pending).Times(1)

	blk, err := c.proposer.Mint(ctx, ap, _ccProducer)
	r.NoError(err)
	r.Equal(c.tip.Height+1, blk.Height())

	// validate a copy, so the receipts are the validator's own
	vblk, err := (&block.Deserializer{}).SetEvmNetworkID(c.cfg.Chain.EVMNetworkID).FromBlockProto(blk.ConvertToBlockPb())
	r.NoError(err)
	r.NoError(c.validator.Validate(ctx, vblk))
	r.NoError(c.proposer.PutBlock(ctx, blk))
	r.NoError(c.validator.PutBlock(ctx, vblk))
	r.Equal(blk.ReceiptRoot(), vblk.ReceiptRoot())

	receipts := make(map[hash.Hash256]*action.Receipt, len(vblk.Receipts))
	for _, receipt := range vblk.Receipts {
		receipts[receipt.ActionHash] = receipt
	}
	c.tip = protocol.TipInfo{
		Height:        blk.Height(),
		Hash:          blk.HashBlock(),
		Timestamp:     blk.Timestamp(),
		GasUsed:       blk.GasUsed(),
		BaseFee:       blk.BaseFee(),
		BlobGasUsed:   blk.BlobGasUsed(),
		ExcessBlobGas: blk.ExcessBlobGas(),
	}
	return vblk, receipts
}

func (c *capChain) candidates(f Factory) staking.CandidateStateReader {
	csr, err := staking.ConstructBaseView(f)
	require.NoError(c.t, err)
	return csr
}

// votes returns cand1's and cand2's votes and cand1's owner, as the validator
// committed them, and checks the proposer agrees.
func (c *capChain) votes() (*big.Int, *big.Int, address.Address) {
	r := require.New(c.t)
	v, p := c.candidates(c.validator), c.candidates(c.proposer)
	a, b := v.GetCandidateByName("cand1"), v.GetCandidateByName("cand2")
	r.NotNil(a)
	r.NotNil(b)
	r.True(a.Equal(p.GetCandidateByName("cand1")))
	r.True(b.Equal(p.GetCandidateByName("cand2")))
	return a.Votes, b.Votes, a.Owner
}

func hashOf(t *testing.T, selp *action.SealedEnvelope) hash.Hash256 {
	h, err := selp.Hash()
	require.NoError(t, err)
	return h
}

func blockHashes(t *testing.T, blk *block.Block) []hash.Hash256 {
	var hs []hash.Hash256
	for _, selp := range blk.Actions {
		if !action.IsSystemAction(selp) {
			hs = append(hs, hashOf(t, selp))
		}
	}
	return hs
}

func mustUpdate(t *testing.T, name, operator, reward string) *action.CandidateUpdate {
	act, err := action.NewCandidateUpdate(name, operator, reward)
	require.NoError(t, err)
	return act
}

func requireSuccess(t *testing.T, receipts map[hash.Hash256]*action.Receipt, acts ...*action.SealedEnvelope) {
	for _, act := range acts {
		require.EqualValues(t, iotextypes.ReceiptStatus_Success, receipts[hashOf(t, act)].Status)
	}
}

// The owner transfers cand1 away, the voter moves its bucket to cand2, and the
// former owner sends CandidateUpdate. The update must wait for the next block,
// where it fails, and votes stay conserved.
func TestMintCandidateActionCapOwner(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	a1, b1, _ := c.votes()
	total := new(big.Int).Add(a1, b1)

	transfer, err := action.NewCandidateTransferOwnership(_ccNewOwner.String(), nil)
	r.NoError(err)
	transferAct := c.sign(_ccOwner, env(1, 3).SetAction(transfer).Build())
	changeAct := c.sign(_ccVoter, env(1, 2).SetAction(action.NewChangeCandidate("cand2", _ccBucket, nil)).Build())
	updateAct := c.sign(_ccOwner, env(2, 1).SetAction(mustUpdate(t, "cand1", "", "")).Build())

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String(): {transferAct, updateAct},
		_ccVoter.String(): {changeAct},
	})
	r.Equal([]hash.Hash256{hashOf(t, transferAct), hashOf(t, changeAct)}, blockHashes(t, blk))
	requireSuccess(t, receipts, transferAct, changeAct)
	a2, b2, owner := c.votes()
	r.Equal(_ccNewOwner.String(), owner.String())
	r.Equal(1, new(big.Int).Sub(a1, a2).Sign())
	r.Equal(total, new(big.Int).Add(a2, b2))

	blk, receipts = c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String(): {updateAct},
	})
	r.Equal([]hash.Hash256{hashOf(t, updateAct)}, blockHashes(t, blk))
	r.NotEqualValues(iotextypes.ReceiptStatus_Success, receipts[hashOf(t, updateAct)].Status)
	a3, b3, owner := c.votes()
	r.Equal(_ccNewOwner.String(), owner.String())
	r.Equal(a2, a3)
	r.Equal(b2, b3)
}

// Operator variant: the owner moves cand1's operator, the voter moves its
// bucket away, and the former operator sends CandidateUpdate.
func TestMintCandidateActionCapOperator(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	a1, b1, _ := c.votes()
	total := new(big.Int).Add(a1, b1)

	// since Yap an operator may only update the BLS key, with a PoP
	seed := hash.Hash256b([]byte("cap-operator"))
	sk, err := crypto.GenerateBLS12381PrivateKey(seed[:])
	r.NoError(err)
	pop, err := staking.SignBLSPop(sk, c.candidates(c.validator).GetCandidateByName("cand1").GetIdentifier())
	r.NoError(err)
	update, err := action.NewCandidateUpdateWithBLS("cand1", "", "", sk.PublicKey().Bytes(), pop)
	r.NoError(err)
	moveOpAct := c.sign(_ccOwner, env(1, 3).SetAction(mustUpdate(t, "cand1", _ccNewOp.String(), "")).Build())
	changeAct := c.sign(_ccVoter, env(1, 2).SetAction(action.NewChangeCandidate("cand2", _ccBucket, nil)).Build())
	updateAct := c.sign(_ccOperator, env(0, 1).SetAction(update).Build())

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String():    {moveOpAct},
		_ccVoter.String():    {changeAct},
		_ccOperator.String(): {updateAct},
	})
	r.Equal([]hash.Hash256{hashOf(t, moveOpAct), hashOf(t, changeAct)}, blockHashes(t, blk))
	requireSuccess(t, receipts, moveOpAct, changeAct)
	a2, b2, _ := c.votes()
	r.Equal(1, new(big.Int).Sub(a1, a2).Sign())
	r.Equal(total, new(big.Int).Add(a2, b2))

	blk, receipts = c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOperator.String(): {updateAct},
	})
	r.Equal([]hash.Hash256{hashOf(t, updateAct)}, blockHashes(t, blk))
	r.NotEqualValues(iotextypes.ReceiptStatus_Success, receipts[hashOf(t, updateAct)].Status)
	a3, b3, _ := c.votes()
	r.Equal(a2, a3)
	r.Equal(b2, b3)
	r.Equal(_ccNewOp.String(), c.candidates(c.validator).GetCandidateByName("cand1").Operator.String())
}

// Ordinary candidate actions are not limited; after an operator change, later
// candidate actions wait for the next block, and bucket actions do not.
func TestMintCandidateActionCapPerBlock(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)

	stake, err := action.NewCreateStake("cand2", unit.ConvertIotxToRau(100).String(), 0, false, nil)
	r.NoError(err)
	stake2, err := action.NewCreateStake("cand1", unit.ConvertIotxToRau(100).String(), 0, false, nil)
	r.NoError(err)
	update1 := c.sign(_ccOwner, env(1, 5).SetAction(mustUpdate(t, "cand1", "", _ccOwner2.String())).Build())
	moveOp2 := c.sign(_ccOwner2, env(1, 4).SetAction(mustUpdate(t, "cand2", _ccNewOp.String(), "")).Build())
	optIn := c.sign(_ccOwner, env(2, 3).SetAction(action.NewSetVoterRewardOptIn()).Build())
	bucketActs := []*action.SealedEnvelope{
		c.sign(_ccVoter, env(1, 2).SetAction(stake).Build()),
		c.sign(_ccVoter, env(2, 2).SetAction(action.NewChangeCandidate("cand2", _ccBucket, nil)).Build()),
		c.sign(_ccVoter, env(3, 2).SetAction(stake2).Build()),
	}

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String():  {update1, optIn},
		_ccOwner2.String(): {moveOp2},
		_ccVoter.String():  bucketActs,
	})
	want := append([]*action.SealedEnvelope{update1, moveOp2}, bucketActs...)
	wantHashes := make([]hash.Hash256, 0, len(want))
	for _, act := range want {
		wantHashes = append(wantHashes, hashOf(t, act))
	}
	r.Equal(wantHashes, blockHashes(t, blk))
	requireSuccess(t, receipts, want...)

	blk, receipts = c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String(): {optIn},
	})
	r.Equal([]hash.Hash256{hashOf(t, optIn)}, blockHashes(t, blk))
	requireSuccess(t, receipts, optIn)
}

// A no-op candidate update does not hold back another candidate's action.
func TestMintCandidateActionCapNoopDoesNotConsumeSlot(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	noop := c.sign(_ccOwner, env(1, 5).SetAction(mustUpdate(t, "cand1", "", "")).Build())
	update2 := c.sign(_ccOwner2, env(1, 4).SetAction(mustUpdate(t, "cand2", "", "")).Build())

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String():  {noop},
		_ccOwner2.String(): {update2},
	})
	r.Equal([]hash.Hash256{hashOf(t, noop), hashOf(t, update2)}, blockHashes(t, blk))
	requireSuccess(t, receipts, noop, update2)
}

// A failed operator change, from an account that owns no candidate, does not
// hold back another candidate's action.
func TestMintCandidateActionCapFailedUpdateDoesNotConsumeSlot(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	failed := c.sign(_ccNewOwner, env(0, 5).SetAction(mustUpdate(t, "cand1", _ccNewOp.String(), "")).Build())
	update2 := c.sign(_ccOwner2, env(1, 4).SetAction(mustUpdate(t, "cand2", "", _ccOwner.String())).Build())

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccNewOwner.String(): {failed},
		_ccOwner2.String():   {update2},
	})
	r.Equal([]hash.Hash256{hashOf(t, failed), hashOf(t, update2)}, blockHashes(t, blk))
	r.EqualValues(iotextypes.ReceiptStatus_ErrCandidateNotExist, receipts[hashOf(t, failed)].Status)
	requireSuccess(t, receipts, update2)
}

// A held-back sender's later actions wait too instead of running with a nonce
// gap, and other senders are not held back.
func TestMintCandidateActionCapHoldsSender(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	moveOp1 := c.sign(_ccOwner, env(1, 5).SetAction(mustUpdate(t, "cand1", _ccNewOp.String(), "")).Build())
	update2 := c.sign(_ccOwner2, env(1, 4).SetAction(mustUpdate(t, "cand2", "", _ccOwner.String())).Build())
	tsf := c.sign(_ccOwner2, env(2, 1).SetAction(action.NewTransfer(big.NewInt(1), _ccVoter.String(), nil)).Build())
	changeAct := c.sign(_ccVoter, env(1, 2).SetAction(action.NewChangeCandidate("cand2", _ccBucket, nil)).Build())

	blk, _ := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String():  {moveOp1},
		_ccOwner2.String(): {update2, tsf},
		_ccVoter.String():  {changeAct},
	})
	r.Equal([]hash.Hash256{hashOf(t, moveOp1), hashOf(t, changeAct)}, blockHashes(t, blk))

	blk, receipts := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner2.String(): {update2, tsf},
	})
	r.Equal([]hash.Hash256{hashOf(t, update2), hashOf(t, tsf)}, blockHashes(t, blk))
	requireSuccess(t, receipts, update2, tsf)
}

// A deferred candidate action that is its sender's last pending one does not
// hold back the next sender in line.
func TestMintCandidateActionCapKeepsOtherSenders(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	moveOp1 := c.sign(_ccOwner, env(1, 5).SetAction(mustUpdate(t, "cand1", _ccNewOp.String(), "")).Build())
	update2 := c.sign(_ccOwner2, env(1, 4).SetAction(mustUpdate(t, "cand2", "", "")).Build())
	tsf := c.sign(_ccVoter, env(1, 3).SetAction(action.NewTransfer(big.NewInt(1), _ccOwner.String(), nil)).Build())

	blk, _ := c.mintAndCommit(map[string][]*action.SealedEnvelope{
		_ccOwner.String():  {moveOp1},
		_ccOwner2.String(): {update2},
		_ccVoter.String():  {tsf},
	})
	r.Equal([]hash.Hash256{hashOf(t, moveOp1), hashOf(t, tsf)}, blockHashes(t, blk))
}

// A bundle dropped for a candidate action after its operator change leaves no
// sender held back and no operator change recorded.
func TestMintCandidateActionCapBundleRevert(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	bp := c.bundles([]*action.SealedEnvelope{
		c.container(_ccOwner, 1, mustUpdate(t, "cand1", _ccNewOp.String(), _ccOwner.String())),
		c.container(_ccOwner2, 1, mustUpdate(t, "cand2", _ccOperator2.String(), _ccOwner2.String())),
	})
	tsf := c.sign(_ccOwner2, env(1, 3).SetAction(action.NewTransfer(big.NewInt(1), _ccVoter.String(), nil)).Build())
	update1 := c.sign(_ccOwner, env(1, 2).SetAction(mustUpdate(t, "cand1", "", _ccOwner2.String())).Build())

	blk, receipts := c.mintWithBundles(bp, map[string][]*action.SealedEnvelope{
		_ccOwner2.String(): {tsf},
		_ccOwner.String():  {update1},
	})
	r.Equal([]hash.Hash256{hashOf(t, tsf), hashOf(t, update1)}, blockHashes(t, blk))
	requireSuccess(t, receipts, tsf, update1)
}

// A candidate action already run in a bundle is skipped in the pool as an old
// nonce, not counted again, so its sender's next action still goes in.
func TestMintCandidateActionCapBundledDuplicate(t *testing.T) {
	r := require.New(t)
	c := newCapChain(t)
	update := c.container(_ccOwner, 1, mustUpdate(t, "cand1", _ccOperator.String(), _ccOwner.String()))
	bp := c.bundles([]*action.SealedEnvelope{update})
	tsf := c.sign(_ccOwner, env(2, 3).SetAction(action.NewTransfer(big.NewInt(1), _ccVoter.String(), nil)).Build())

	blk, receipts := c.mintWithBundles(bp, map[string][]*action.SealedEnvelope{
		_ccOwner.String(): {c.container(_ccOwner, 1, mustUpdate(t, "cand1", _ccOperator.String(), _ccOwner.String())), tsf},
	})
	r.Equal([]hash.Hash256{hashOf(t, update), hashOf(t, tsf)}, blockHashes(t, blk))
	requireSuccess(t, receipts, update, tsf)
}
