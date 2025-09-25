// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package stakingindex

import (
	"context"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestCandBucketIndexer(t *testing.T) {
	req := require.New(t)

	cbIndexPath := MustNoErrorV(testutil.PathOfTempFile("cbindex"))
	defer testutil.CleanupPath(cbIndexPath)
	cfg := db.DefaultConfig
	cfg.DbPath = cbIndexPath
	kv := db.NewKVStoreWithVersion(cfg, db.VersionedNamespaceOption(
		db.Namespace{StakingCandidatesNamespace, 20}, db.Namespace{StakingBucketsNamespace, 8}))

	ci := MustNoErrorV(NewCandBucketsIndexer(kv))
	ctx := context.Background()
	req.NoError(ci.Start(ctx))
	defer func() { req.NoError(ci.Stop(ctx)) }()
	req.Zero(ci.currentHeight)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(ctx, genesis.Default), protocol.BlockCtx{}))
	req.NoError(ci.PutBlock(ctx, block.NewBlockDeprecated(0, 1, hash.ZeroHash256, testutil.TimestampNow(), nil, nil)))
	req.NoError(ci.PutBlock(ctx, block.NewBlockDeprecated(0, 2, hash.ZeroHash256, testutil.TimestampNow(), nil, nil)))
	req.NoError(ci.PutBlock(ctx, block.NewBlockDeprecated(0, 3, hash.ZeroHash256, testutil.TimestampNow(), nil, nil)))
	b, _, err := ci.GetBuckets(3, 0, 8)
	req.NoError(err)
	req.Equal(6, len(b.Buckets))
	for i, index := range []uint64{0, 2, 5, 6, 7, 8} {
		req.Equal(index, b.Buckets[i].Index)
	}
	req.NoError(ci.Stop(ctx))
	req.NoError(ci.Start(ctx))
	req.Equal(&bucketList{maxBucket: 8, deleted: []uint64{1, 3, 4}}, ci.deleteList)
	csr := MustNoErrorV(staking.ConstructBaseView(ci.stateReader))
	cList := MustNoErrorV(staking.ToIoTeXTypesCandidateListV2(csr, c, protocol.MustGetFeatureCtx(ctx)))
	for i := range 3 {
		cand, _, err := ci.GetCandidates(uint64(i+1), 0, 4)
		req.NoError(err)
		req.Equal(i+1, len(cand.Candidates))
		for j := range i + 1 {
			req.Equal(cList.Candidates[j].String(), cand.Candidates[j].String())
		}
	}
}
