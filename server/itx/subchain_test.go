// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestGetSubChainDBPath(t *testing.T) {
	t.Parallel()

	chainDBPath := getSubChainDBPath(1, config.Default.Chain.ChainDBPath)
	trieDBPath := getSubChainDBPath(1, config.Default.Chain.TrieDBPath)
	assert.Equal(t, "chain-1-chain.db", chainDBPath)
	assert.Equal(t, "chain-1-trie.db", trieDBPath)
}
func TestHandleBlock(t *testing.T) {
	require := require.New(t)
	cfg, err := config.New()
	require.NoError(err)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	ss, err := NewServer(cfg)
	require.NoError(err)
	require.NotNil(ss)
	ctx, cancel := context.WithCancel(context.Background())
	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	err = probeSvr.Start(ctx)
	require.NoError(err)
	go StartServer(ctx, ss, probeSvr, cfg)
	time.Sleep(time.Second * 2)

	subChain := &mainchain.SubChain{10, big.NewInt(10), big.NewInt(10), 10, 12, 1, nil, 5, 3}
	err = ss.runSubChain(identityset.Address(10), subChain)
	require.NoError(err)

	rap := block.RunnableActionsBuilder{}
	ra := rap.
		SetHeight(1).
		SetTimeStamp(time.Now()).
		Build(identityset.PrivateKey(0).PublicKey())
	blk, err := block.NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)

	go func() {
		err = ss.HandleBlock(&blk)
		require.NoError(err)
	}()

	time.Sleep(time.Second * 5)
	cancel()
	err = probeSvr.Stop(livenessCtx)
	require.NoError(err)
	livenessCancel()
}
