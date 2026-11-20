// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
)

func newExecCapFactory(t *testing.T, cfg Config) Factory {
	r := require.New(t)
	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	r.NoError(execution.NewProtocol(
		func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		rewarding.DepositGas,
		func(uint64) (time.Time, error) { return time.Time{}, nil },
		nil,
	).Register(registry))
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	r.NoError(err)
	startCtx := protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)))
	r.NoError(f.Start(startCtx))
	t.Cleanup(func() { r.NoError(f.Stop(startCtx)) })
	return f
}

// mintBundles has the proposer mint the next block from bundles alone and
// returns it once the validator has accepted it.
func (c *capChain) mintBundles(bundles ...[]*action.SealedEnvelope) *block.Block {
	return c.mintDraft(map[string][]*action.SealedEnvelope{}, bundles...)
}

// mintDraft has the proposer mint the next block from bundles and the pending
// pool and returns it once the validator has accepted it.
func (c *capChain) mintDraft(pending map[string][]*action.SealedEnvelope, bundles ...[]*action.SealedEnvelope) *block.Block {
	r := require.New(c.t)
	ctx := c.ctx()
	ap := mock_actpool.NewMockActPool(gomock.NewController(c.t))
	ap.EXPECT().BundlePool().Return(c.bundles(bundles...)).AnyTimes()
	ap.EXPECT().PendingActionMap().Return(pending).AnyTimes()
	ap.EXPECT().DeleteAction(gomock.Any()).AnyTimes()
	blk, err := c.proposer.Mint(ctx, ap, _ccProducer)
	r.NoError(err)
	vblk, err := (&block.Deserializer{}).SetEvmNetworkID(c.cfg.Chain.EVMNetworkID).FromBlockProto(blk.ConvertToBlockPb())
	r.NoError(err)
	r.NoError(c.validator.Validate(ctx, vblk), "mint must never emit a block its validators reject")
	return blk
}

// setCodeFrom returns a SetCode tx from signer carrying one authorization
// signed by authority at authNonce.
func (c *capChain) setCodeFrom(signer, authority int, nonce, authNonce uint64) *action.SealedEnvelope {
	return c.setCodeTo(signer, authority, nonce, authNonce, common.BytesToAddress(identityset.Address(12).Bytes()))
}

// setCodeTo is setCodeFrom with the authorization delegating to delegate.
func (c *capChain) setCodeTo(signer, authority int, nonce, authNonce uint64, delegate common.Address) *action.SealedEnvelope {
	r := require.New(c.t)
	auth, err := types.SignSetCode(identityset.PrivateKey(authority).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(0),
		Address: delegate,
		Nonce:   authNonce,
	})
	r.NoError(err)
	evmID := uint64(c.cfg.Chain.EVMNetworkID)
	tx := types.MustSignNewTx(identityset.PrivateKey(signer).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(new(big.Int).SetUint64(evmID)), &types.SetCodeTx{
		ChainID:   uint256.NewInt(evmID),
		Nonce:     nonce,
		GasTipCap: uint256.NewInt(uint64(action.InitialBaseFee)),
		GasFeeCap: uint256.NewInt(uint64(action.InitialBaseFee) * 4),
		Gas:       1000000,
		To:        common.BytesToAddress(identityset.Address(signer).Bytes()),
		Value:     uint256.NewInt(0),
		AuthList:  []types.SetCodeAuthorization{auth},
	})
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	r.NoError(err)
	raw, err := tx.MarshalBinary()
	r.NoError(err)
	core, err := action.EthRawToContainer(1, hex.EncodeToString(raw))
	r.NoError(err)
	selp, err := (&action.Deserializer{}).SetEvmNetworkID(c.cfg.Chain.EVMNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
		Core: core, SenderPubKey: pubkey.Bytes(), Signature: sig, Encoding: iotextypes.Encoding_TX_CONTAINER,
	})
	r.NoError(err)
	return selp
}

// TestMint_BundleNoncesAgainstBlockStart checks bundle nonces at mint against
// the state at the start of the block, which block validation uses, rather
// than against the state left by earlier bundles.
func TestMint_BundleNoncesAgainstBlockStart(t *testing.T) {
	newChain := func(t *testing.T) (*capChain, uint64) {
		cfg := capConfig() // forks through Zanzibar Gamma, incl. Yap, from height 1
		c := &capChain{t: t, cfg: cfg, proposer: newExecCapFactory(t, cfg), validator: newExecCapFactory(t, cfg)}
		st, err := accountutil.AccountState(c.ctx(), c.validator, identityset.Address(11))
		require.NoError(t, err)
		return c, st.PendingNonceConsideringFreshAccount()
	}
	transferFrom := func(c *capChain, signer int, nonce uint64) *action.SealedEnvelope {
		return c.sign(identityset.Address(signer), env(nonce, 2).SetAction(action.NewTransfer(big.NewInt(1), identityset.Address(10).String(), nil)).Build())
	}

	t.Run("authorization moves the nonce of a later bundle's sender", func(t *testing.T) {
		c, start := newChain(t)
		// bundle 0 moves account 11's nonce to start+1 through an
		// authorization; bundle 1 relies on that
		setCode := c.setCodeFrom(10, 11, 0, start)
		after := transferFrom(c, 11, start+1)
		blk := c.mintBundles([]*action.SealedEnvelope{setCode}, []*action.SealedEnvelope{after})
		require.Equal(t, []hash.Hash256{hashOf(t, setCode)}, blockHashes(t, blk))
	})

	t.Run("nonces continue across bundles", func(t *testing.T) {
		c, start := newChain(t)
		first, second := transferFrom(c, 11, start), transferFrom(c, 11, start+1)
		blk := c.mintBundles([]*action.SealedEnvelope{first}, []*action.SealedEnvelope{second})
		require.Equal(t, []hash.Hash256{hashOf(t, first), hashOf(t, second)}, blockHashes(t, blk))
	})

	t.Run("a skipped bundle's nonces are not counted", func(t *testing.T) {
		c, start := newChain(t)
		// the first bundle has a gap and is skipped; the second starts at
		// the same nonce and must still be admitted
		gapped := []*action.SealedEnvelope{transferFrom(c, 11, start), transferFrom(c, 11, start+2)}
		next := c.sign(identityset.Address(11), env(start, 3).SetAction(action.NewTransfer(big.NewInt(2), identityset.Address(10).String(), nil)).Build())
		blk := c.mintBundles(gapped, []*action.SealedEnvelope{next})
		require.Equal(t, []hash.Hash256{hashOf(t, next)}, blockHashes(t, blk))
	})
}

// TestMint_PoolNoncesAgainstBlockStart checks pool actions at mint against
// the nonces at the start of the block, which block validation uses: once an
// action has moved another account's nonce, that account's later actions in
// the block are left out, and an action that leaves nonces alone holds nobody
// back.
func TestMint_PoolNoncesAgainstBlockStart(t *testing.T) {
	newChain := func(t *testing.T) (*capChain, uint64) {
		cfg := capConfig()
		c := &capChain{t: t, cfg: cfg, proposer: newExecCapFactory(t, cfg), validator: newExecCapFactory(t, cfg)}
		return c, c.pendingNonce(11)
	}
	transferFrom := func(c *capChain, signer int, nonce uint64, price int64) *action.SealedEnvelope {
		return c.sign(identityset.Address(signer), env(nonce, price).SetAction(action.NewTransfer(big.NewInt(1), identityset.Address(12).String(), nil)).Build())
	}
	authority := identityset.Address(11).String()

	t.Run("authorization in the pool", func(t *testing.T) {
		c, start := newChain(t)
		// the SetCode tx is priced above the authority's own actions, so it
		// runs first and moves the authority's nonce to start+1
		setCode := c.setCodeFrom(10, 11, 0, start)
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			identityset.Address(10).String(): {setCode},
			authority:                        {transferFrom(c, 11, start, 1), transferFrom(c, 11, start+1, 1)},
		})
		require.Equal(t, []hash.Hash256{hashOf(t, setCode)}, blockHashes(t, blk))
	})

	t.Run("authorization in a bundle", func(t *testing.T) {
		c, start := newChain(t)
		setCode := c.setCodeFrom(10, 11, 0, start)
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			authority: {transferFrom(c, 11, start, 1), transferFrom(c, 11, start+1, 1)},
		}, []*action.SealedEnvelope{setCode})
		require.Equal(t, []hash.Hash256{hashOf(t, setCode)}, blockHashes(t, blk))
	})

	t.Run("authority's earlier action is kept", func(t *testing.T) {
		c, start := newChain(t)
		// the authority's first action runs before the SetCode tx, whose
		// authorization then signs the next nonce and moves it to start+2;
		// the authority's later actions wait
		first := transferFrom(c, 11, start, 10)
		setCode := c.setCodeFrom(10, 11, 0, start+1)
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			identityset.Address(10).String(): {setCode},
			authority:                        {first, transferFrom(c, 11, start+1, 1), transferFrom(c, 11, start+2, 1)},
		})
		require.Equal(t, []hash.Hash256{hashOf(t, first), hashOf(t, setCode)}, blockHashes(t, blk))
	})

	t.Run("authorization that does not apply", func(t *testing.T) {
		c, start := newChain(t)
		// signed for a nonce the authority is not at, so it leaves the
		// authority alone, and so does the mint
		setCode := c.setCodeFrom(10, 11, 0, start+5)
		own := transferFrom(c, 11, start, 1)
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			identityset.Address(10).String(): {setCode},
			authority:                        {own},
		})
		require.Equal(t, []hash.Hash256{hashOf(t, setCode), hashOf(t, own)}, blockHashes(t, blk))
	})

	t.Run("pool continues after a bundle", func(t *testing.T) {
		c, start := newChain(t)
		// the bundle takes the authority's first nonce; the pool still holds
		// that nonce and the next one, which must go in after the bundle
		bundled := transferFrom(c, 11, start, 1)
		next := transferFrom(c, 11, start+1, 1)
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			authority: {transferFrom(c, 11, start, 2), next},
		}, []*action.SealedEnvelope{bundled})
		require.Equal(t, []hash.Hash256{hashOf(t, bundled), hashOf(t, next)}, blockHashes(t, blk))
	})

	t.Run("create by delegated code", func(t *testing.T) {
		r := require.New(t)
		c, start := newChain(t)
		// block 1 deploys code that runs a CREATE whenever it is called
		// (runtime PUSH1 0 PUSH1 0 PUSH1 0 CREATE POP STOP)
		initCode, err := hex.DecodeString("68600060006000f0500060005260096017f3")
		r.NoError(err)
		deploy := c.sign(identityset.Address(12), env(c.pendingNonce(12), 2).SetAction(action.NewExecution("", big.NewInt(0), initCode)).Build())
		_, receipts := c.mintWithBundles(nil, map[string][]*action.SealedEnvelope{identityset.Address(12).String(): {deploy}})
		r.EqualValues(iotextypes.ReceiptStatus_Success, receipts[hashOf(t, deploy)].Status)
		code, err := address.FromString(receipts[hashOf(t, deploy)].ContractAddress)
		r.NoError(err)
		// block 2 delegates the authority to it
		setCode := c.setCodeTo(10, 11, c.pendingNonce(10), start, common.BytesToAddress(code.Bytes()))
		_, receipts = c.mintWithBundles(nil, map[string][]*action.SealedEnvelope{identityset.Address(10).String(): {setCode}})
		r.EqualValues(iotextypes.ReceiptStatus_Success, receipts[hashOf(t, setCode)].Status)
		// block 3: a plain call to the authority runs the CREATE and moves
		// its nonce, with no authorization in this block
		next := c.pendingNonce(11)
		call := c.sign(identityset.Address(13), env(c.pendingNonce(13), 10).SetAction(action.NewExecution(authority, big.NewInt(0), nil)).Build())
		blk := c.mintDraft(map[string][]*action.SealedEnvelope{
			identityset.Address(13).String(): {call},
			authority:                        {transferFrom(c, 11, next, 1), transferFrom(c, 11, next+1, 1)},
		})
		require.Equal(t, []hash.Hash256{hashOf(t, call)}, blockHashes(t, blk))
	})
}

// pendingNonce is the next nonce of identityset account i on the validator
func (c *capChain) pendingNonce(i int) uint64 {
	st, err := accountutil.AccountState(c.ctx(), c.validator, identityset.Address(i))
	require.NoError(c.t, err)
	return st.PendingNonceConsideringFreshAccount()
}
