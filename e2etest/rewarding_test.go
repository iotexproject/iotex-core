package e2etest

import (
	"context"
	"math/big"
	"os"
	"path"
	"testing"
	"time"

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
	dir := os.TempDir()
	cleanDB := func() {
		testutil.CleanupPath(t, path.Join(dir, "./trie.db"))
		testutil.CleanupPath(t, path.Join(dir, "./chain.db"))
	}
	cleanDB()

	cfg := config.Default
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Consensus.BlockCreationInterval = time.Second
	cfg.Chain.ProducerPrivKey = "507f8c8b08358d7ab1d020889a4fa0b8a123b41b6459cb436df4d0d02d8f0ca6"
	cfg.Chain.ProducerPubKey = "044f22cfc4cf0b417fe7a5ba7205fd7743d26f6b53901a5fa5e61b555af834e98717a1537bdd18b9cded6020fa8cacff1db0520ca32debcbd4468cea79a7114561"
	cfg.Chain.TrieDBPath = path.Join(dir, "./trie.db")
	cfg.Chain.ChainDBPath = path.Join(dir, "./chain.db")
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	require.NoError(t, svr.Start(context.Background()))
	defer func() {
		cleanDB()
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

	sk, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	pk := &sk.PublicKey
	pkHash1 := keypair.HashPubKey(pk)
	addr, err := address.FromBytes(pkHash1[:])

	blockReward, err := rp.BlockReward(ctx, ws)
	require.NoError(t, err)
	balance, err := rp.UnclaimedBalance(ctx, ws, addr)
	require.NoError(t, err)
	assert.True(t, balance.Cmp(big.NewInt(0).Mul(blockReward, big.NewInt(5))) >= 0)
}
