package ioswarm

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"strings"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// fakeSettlerClient records sent nonces; failNext makes the next send report
// an error, and acceptOnFail makes the node keep the tx anyway.
type fakeSettlerClient struct {
	pending      uint64
	sent         []uint64
	failNext     bool
	acceptOnFail bool
}

func (f *fakeSettlerClient) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return f.pending, nil
}

func (f *fakeSettlerClient) SuggestGasPrice(context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}

func (f *fakeSettlerClient) SendTransaction(_ context.Context, tx *types.Transaction) error {
	if f.failNext {
		f.failNext = false
		if f.acceptOnFail {
			f.sent = append(f.sent, tx.Nonce())
			f.pending = tx.Nonce() + 1
		}
		return errors.New("timeout")
	}
	f.sent = append(f.sent, tx.Nonce())
	f.pending = tx.Nonce() + 1
	return nil
}

func (f *fakeSettlerClient) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return nil, errors.New("not found")
}

func newTestSettler(t *testing.T, c settlerClient) *OnChainSettler {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	parsed, err := abi.JSON(strings.NewReader(rewardPoolABI))
	require.NoError(t, err)
	return &OnChainSettler{client: c, chainID: big.NewInt(4689), privKey: key, abi: parsed, logger: zap.NewNop()}
}

func TestSettleDoesNotPayAnEpochTwice(t *testing.T) {
	agents := []string{"0x0000000000000000000000000000000000000001"}
	weights := []*big.Int{big.NewInt(1)}
	for _, accepted := range []bool{true, false} {
		c := &fakeSettlerClient{pending: 5, failNext: true, acceptOnFail: accepted}
		s := newTestSettler(t, c)
		require.Error(t, s.Settle(context.Background(), agents, weights, big.NewInt(100), nil))
		// the coordinator retries the same frozen summary
		require.NoError(t, s.Settle(context.Background(), agents, weights, big.NewInt(100), nil))
		// either way the epoch went out under nonce 5 exactly once
		require.Equal(t, []uint64{5}, c.sent, "accepted=%v", accepted)
		// and the next epoch uses the next nonce
		require.NoError(t, s.Settle(context.Background(), agents, weights, big.NewInt(100), nil))
		require.Equal(t, []uint64{5, 6}, c.sent, "accepted=%v", accepted)
	}
}

func TestRegisterNormalisesWalletAddress(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	resp, err := h.Register(context.Background(), &pb.RegisterRequest{AgentID: "io", WalletAddress: "io1vkcvq4ywarvfj4u9zwlqedfsttalq55jmtmqcu"})
	r.NoError(err)
	r.True(resp.Accepted)
	r.Equal("0x65B0C0548ee8D899578513be0CB5305AFBf05292", c.reward.CurrentWork()["io"].WalletAddress)

	resp, err = h.Register(context.Background(), &pb.RegisterRequest{AgentID: "bad", WalletAddress: "not-an-address"})
	r.NoError(err)
	r.False(resp.Accepted)
	_, ok := c.registry.GetAgent("bad")
	r.False(ok)
}
