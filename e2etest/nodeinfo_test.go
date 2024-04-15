// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

func newConfigForNodeInfoTest(triePath, dBPath, idxDBPath, contractIdxDBPath string) (config.Config, func(), error) {
	cfg, err := newTestConfig()
	if err != nil {
		return cfg, nil, err
	}
	cfg.Genesis.PalauBlockHeight = 0
	testTriePath, err := testutil.PathOfTempFile(triePath)
	if err != nil {
		return cfg, nil, err
	}
	testDBPath, err := testutil.PathOfTempFile(dBPath)
	if err != nil {
		return cfg, nil, err
	}
	indexDBPath, err := testutil.PathOfTempFile(idxDBPath)
	if err != nil {
		return cfg, nil, err
	}
	contractIndexDBPath, err := testutil.PathOfTempFile(contractIdxDBPath)
	if err != nil {
		return cfg, nil, err
	}
	sgdIndexDBPath, err := testutil.PathOfTempFile(idxDBPath)
	if err != nil {
		return cfg, nil, err
	}
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath
	cfg.Chain.ContractStakingIndexDBPath = contractIndexDBPath
	cfg.Chain.SGDIndexDBPath = sgdIndexDBPath
	return cfg, func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(indexDBPath)
		testutil.CleanupPath(contractIndexDBPath)
	}, nil
}

func TestBroadcastNodeInfo(t *testing.T) {
	require := require.New(t)

	cfgSender, teardown, err := newConfigForNodeInfoTest("trie.test", "db.test", "indexdb.test", "contractidxdb.test")
	require.NoError(err)
	defer teardown()
	cfgSender.NodeInfo.EnableBroadcastNodeInfo = true
	cfgSender.NodeInfo.BroadcastNodeInfoInterval = time.Second
	cfgSender.Network.ReconnectInterval = 2 * time.Second
	srvSender, err := itx.NewServer(cfgSender)
	require.NoError(err)
	ctxSender := genesis.WithGenesisContext(context.Background(), cfgSender.Genesis)
	err = srvSender.Start(ctxSender)
	require.NoError(err)
	defer func() {
		require.NoError(srvSender.Stop(ctxSender))
	}()
	addrsSender, err := srvSender.P2PAgent().Self()
	require.NoError(err)

	cfgReciever, teardown2, err := newConfigForNodeInfoTest("trie2.test", "db2.test", "indexdb2.test", "contractidxdb2.test")
	require.NoError(err)
	defer teardown2()
	cfgReciever.Network.BootstrapNodes = []string{validNetworkAddr(addrsSender)}
	cfgReciever.Network.ReconnectInterval = 2 * time.Second
	srvReciever, err := itx.NewServer(cfgReciever)
	require.NoError(err)
	ctxReciever := genesis.WithGenesisContext(context.Background(), cfgReciever.Genesis)
	err = srvReciever.Start(ctxReciever)
	require.NoError(err)
	defer func() {
		require.NoError(srvReciever.Stop(ctxReciever))
	}()

	// check if there is sender's info in reciever delegatemanager
	require.NoError(srvSender.ChainService(cfgSender.Chain.ID).NodeInfoManager().BroadcastNodeInfo(context.Background()))
	addrSender := cfgSender.Chain.ProducerAddress().String()
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		_, ok := srvReciever.ChainService(cfgReciever.Chain.ID).NodeInfoManager().GetNodeInfo(addrSender)
		return ok, nil
	}))
}

func TestUnicastNodeInfo(t *testing.T) {
	require := require.New(t)

	cfgReciever, teardown2, err := newConfigForNodeInfoTest("trie2.test", "db2.test", "indexdb2.test", "contractidxdb2.test")
	require.NoError(err)
	defer teardown2()
	cfgReciever.Network.ReconnectInterval = 2 * time.Second
	srvReciever, err := itx.NewServer(cfgReciever)
	require.NoError(err)
	ctxReciever := genesis.WithGenesisContext(context.Background(), cfgReciever.Genesis)
	err = srvReciever.Start(ctxReciever)
	require.NoError(err)
	defer func() {
		require.NoError(srvReciever.Stop(ctxReciever))
	}()
	addrsReciever, err := srvReciever.P2PAgent().Self()
	require.NoError(err)

	cfgSender, teardown, err := newConfigForNodeInfoTest("trie.test", "db.test", "indexdb.test", "contractidxdb.test")
	require.NoError(err)
	defer teardown()
	cfgSender.Network.ReconnectInterval = 2 * time.Second
	cfgSender.Network.BootstrapNodes = []string{validNetworkAddr(addrsReciever)}
	srvSender, err := itx.NewServer(cfgSender)
	require.NoError(err)
	ctxSender := genesis.WithGenesisContext(context.Background(), cfgSender.Genesis)
	err = srvSender.Start(ctxSender)
	require.NoError(err)
	defer func() {
		require.NoError(srvSender.Stop(ctxSender))
	}()

	// check if there is reciever's info in sender delegatemanager
	peerReciever, err := srvReciever.P2PAgent().Info()
	require.NoError(err)
	dmSender := srvSender.ChainService(cfgSender.Chain.ID).NodeInfoManager()
	err = dmSender.RequestSingleNodeInfoAsync(context.Background(), peerReciever)
	require.NoError(err)
	addrReciever := cfgReciever.Chain.ProducerAddress().String()
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		_, ok := dmSender.GetNodeInfo(addrReciever)
		return ok, nil
	}))
}
