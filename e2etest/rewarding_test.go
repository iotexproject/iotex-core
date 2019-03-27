package e2etest

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockReward(t *testing.T) {
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Genesis.BlockInterval = time.Second
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(0).HexString()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	require.NoError(t, svr.Start(context.Background()))
	defer func() {
		require.NoError(t, svr.Stop(context.Background()))
	}()

	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (b bool, e error) {
		return svr.ChainService(1).Blockchain().TipHeight() >= 5, nil
	}))

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{})

	p, ok := svr.ChainService(1).Registry().Find(rewarding.ProtocolID)
	require.True(t, ok)
	rp, ok := p.(*rewarding.Protocol)
	require.True(t, ok)
	sf := svr.ChainService(1).Blockchain().GetFactory()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)

	sk, err := keypair.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	addr, err := address.FromBytes(sk.PublicKey().Hash())
	require.NoError(t, err)

	blockReward, err := rp.BlockReward(ctx, ws)
	require.NoError(t, err)
	balance, err := rp.UnclaimedBalance(ctx, ws, addr)
	require.NoError(t, err)
	assert.True(t, balance.Cmp(big.NewInt(0).Mul(blockReward, big.NewInt(5))) >= 0)
}
