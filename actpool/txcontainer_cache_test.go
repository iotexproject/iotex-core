package actpool

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// sanityCheckValidator stands in for GenericValidator's stateless check so the
// test isolates whether the actpool cache short-circuits validation.
type sanityCheckValidator struct{}

func (sanityCheckValidator) Validate(_ context.Context, selp *action.SealedEnvelope) error {
	return selp.Envelope.SanityCheck()
}

// buildFoldedAndUnfoldedDeposit returns the same signed depositToStake(1, 0)
// twice: once as the folded TX_CONTAINER a peer gossips, and once as the
// native action a proposer unfolds and bakes into a block. Both share the
// action hash; only the encoding differs.
func buildFoldedAndUnfoldedDeposit(t *testing.T) (*action.SealedEnvelope, *action.SealedEnvelope) {
	t.Helper()
	r := require.New(t)

	const chainID, evmNetworkID = uint32(1), uint32(1)
	ds, err := action.NewDepositToStake(1, "0", nil)
	r.NoError(err)
	data, err := ds.EthData()
	r.NoError(err)

	to := common.BytesToAddress(address.StakingProtocolAddrHash[:])
	key := identityset.PrivateKey(28)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(1000),
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
	container, err := (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         core,
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	})
	r.NoError(err)

	native := *container
	checker := func(_ context.Context, dst *common.Address) (bool, bool, bool, error) {
		if dst != nil && *dst == to {
			return false, true, false, nil
		}
		return false, false, false, nil
	}
	tc, ok := container.Envelope.(action.TxContainer)
	r.True(ok)
	r.NoError(tc.Unfold(&native, context.Background(), checker))

	ch, err := container.Hash()
	r.NoError(err)
	nh, err := native.Hash()
	r.NoError(err)
	r.Equal(ch, nh, "folded container and unfolded native action must share the hash")
	r.NotEqual(container.Encoding(), native.Encoding())
	return container, &native
}

// TestActPool_CacheHitRequiresSameEncoding checks that a native action does
// not skip validation just because a folded container with the same hash was
// cached at admission.
func TestActPool_CacheHitRequiresSameEncoding(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	g := genesis.TestDefault()
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	Ap, err := NewActPool(g, sf, getActPoolCfg())
	r.NoError(err)
	ap, ok := Ap.(*actPool)
	r.True(ok)
	ap.AddActionEnvelopeValidators(sanityCheckValidator{})

	ctx := genesis.WithGenesisContext(context.Background(), g)
	container, native := buildFoldedAndUnfoldedDeposit(t)

	// The node admitted and cached the gossiped container.
	ch, err := container.Hash()
	r.NoError(err)
	ap.allActions.Set(ch, container)

	// The folded container's own stateless check passes (it checks only the
	// outer tx), and the cache hit keeps admission cheap.
	r.NoError(ap.Validate(ctx, container))

	// The block carries the unfolded native action under the same hash. It
	// must not be waved through on the cached container: depositToStake(_, 0)
	// is stateless-invalid, so a validator without the container rejects it.
	err = ap.Validate(ctx, native)
	r.Error(err)
	r.ErrorIs(err, action.ErrInvalidAmount)
}
