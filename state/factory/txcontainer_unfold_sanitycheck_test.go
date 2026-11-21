package factory

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// signedDepositContainer builds a TX_CONTAINER around a signed
// depositToStake(1, 0). The outer tx is stateless-valid, but the unfolded
// native action fails SanityCheck (amount must be positive).
func signedDepositContainer(t *testing.T, chainID, evmNetworkID uint32, nonce uint64) *action.SealedEnvelope {
	t.Helper()
	r := require.New(t)

	ds, err := action.NewDepositToStake(1, "0", nil)
	r.NoError(err)
	data, err := ds.EthData()
	r.NoError(err)

	to := common.BytesToAddress(address.StakingProtocolAddrHash[:])
	key := identityset.PrivateKey(28)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(unit.Qev),
		Gas:      100000,
		To:       &to,
		Value:    big.NewInt(0),
		Data:     data,
	})
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(int64(evmNetworkID))), key.EcdsaPrivateKey().(*ecdsa.PrivateKey))
	r.NoError(err)

	_, sig, pubkey, err := action.ExtractTypeSigPubkey(signed)
	r.NoError(err)
	raw, err := signed.MarshalBinary()
	r.NoError(err)
	core, err := action.EthRawToContainer(chainID, hex.EncodeToString(raw))
	r.NoError(err)
	selp, err := (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         core,
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	})
	r.NoError(err)
	return selp
}

// TestMintDropsStatelessInvalidUnfoldedContainer checks that mint leaves out
// an action that becomes stateless-invalid only after Unfold, instead of
// minting it into a block.
func TestMintDropsStatelessInvalidUnfoldedContainer(t *testing.T) {
	r := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	r.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)

	a := identityset.Address(28)
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[a.String()] = unit.ConvertIotxToRau(10000000).String()

	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	sp, err := staking.NewProtocol(
		staking.HelperCtx{
			DepositGas: rewarding.DepositGas,
			BlockInterval: func(uint64) time.Duration {
				return time.Second
			},
		},
		&staking.BuilderConfig{
			Staking:                       genesis.TestDefault().Staking,
			PersistStakingPatchBlock:      math.MaxUint64,
			SkipContractStakingViewHeight: math.MaxUint64,
		},
		nil, nil, nil, nil,
	)
	r.NoError(err)
	r.NoError(sp.Register(registry))

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	r.NoError(err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption(), RegistryStateDBOption(registry))
	r.NoError(err)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	r.NoError(sdb.Start(ctx))
	defer func() {
		r.NoError(sdb.Stop(ctx))
	}()

	selp := signedDepositContainer(t, cfg.Chain.ID, cfg.Chain.EVMNetworkID, 1)

	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: 1,
		Producer:    identityset.Address(27),
		GasLimit:    testutil.TestGasLimit * 100,
	})
	ctx = protocol.WithFeatureCtx(ctx)
	mockActPool := mock_actpool.NewMockActPool(gomock.NewController(t))
	mockActPool.EXPECT().BundlePool().Return(nil).AnyTimes()
	mockActPool.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		a.String(): {selp},
	}).Times(1)
	// the unfolded action is stateless-invalid and must be dropped, evicting
	// its sender so the next mint does not pick it again
	mockActPool.EXPECT().DeleteAction(gomock.Any()).Times(1)

	blk, err := sdb.Mint(
		protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
			ChainID: 1,
			Tip:     protocol.TipInfo{Height: 0, Hash: hash.ZeroHash256},
		}),
		mockActPool,
		identityset.PrivateKey(27),
	)
	r.NoError(err)
	r.NotNil(blk)
	r.Empty(blk.Actions, "a container whose unfolded action fails SanityCheck must not be included")
}
