// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestInspectShardRangesV2(t *testing.T) {
	require := require.New(t)
	testPath, err := testutil.PathOfTempFile("inspect-shard-ranges-v2")
	require.NoError(err)
	defer testutil.CleanupPath(testPath)

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	cfg.V2BlocksToSplitDB = 10

	deser := block.NewDeserializer(_defaultEVMNetworkID)
	fd, err := NewFileDAO(cfg, deser)
	require.NoError(err)

	ctx := context.Background()
	require.NoError(fd.Start(ctx))
	require.NoError(testCommitBlocks(t, fd, 1, 25, hash.ZeroHash256))
	require.NoError(fd.Stop(ctx))

	ranges, err := InspectShardRanges(cfg, deser)
	require.NoError(err)
	require.Equal([]ShardRange{
		{FilePath: testPath, Version: FileV2, StartHeight: 1, EndHeight: 10},
		{FilePath: kthAuxFileName(testPath, 1), Version: FileV2, StartHeight: 11, EndHeight: 20},
		{FilePath: kthAuxFileName(testPath, 2), Version: FileV2, StartHeight: 21, EndHeight: 25},
	}, ranges)
}

func TestInspectShardRangesLegacyAndV2(t *testing.T) {
	require := require.New(t)
	testPath, err := testutil.PathOfTempFile("inspect-shard-ranges-hybrid")
	require.NoError(err)
	defer testutil.CleanupPath(testPath)
	defer testutil.CleanupPath(kthAuxFileName(testPath, 1))
	defer testutil.CleanupPath(kthAuxFileName(testPath, 2))
	defer testutil.CleanupPath(kthAuxFileName(testPath, 3))
	defer testutil.CleanupPath(kthAuxFileName(testPath, 4))

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	cfg.SplitDBHeight = 5
	cfg.SplitDBSizeMB = 20

	deser := block.NewDeserializer(_defaultEVMNetworkID)
	fd, err := newFileDAOLegacy(cfg, deser)
	require.NoError(err)

	ctx := context.Background()
	require.NoError(fd.Start(ctx))
	require.NoError(testCommitBlocks(t, fd, 1, 10, hash.ZeroHash256))
	require.NoError(fd.Stop(ctx))

	cfg.V2BlocksToSplitDB = 15
	fd2, err := NewFileDAO(cfg, deser)
	require.NoError(err)
	require.NoError(fd2.Start(ctx))
	require.NoError(testCommitBlocks(t, fd2, 11, 55, hash.ZeroHash256))
	require.NoError(fd2.Stop(ctx))

	ranges, err := InspectShardRanges(cfg, deser)
	require.NoError(err)
	require.Equal([]ShardRange{
		{FilePath: testPath, Version: FileLegacyMaster, StartHeight: 1, EndHeight: 5},
		{FilePath: kthAuxFileName(testPath, 1), Version: FileLegacyAuxiliary, StartHeight: 6, EndHeight: 15},
		{FilePath: kthAuxFileName(testPath, 2), Version: FileV2, StartHeight: 16, EndHeight: 30},
		{FilePath: kthAuxFileName(testPath, 3), Version: FileV2, StartHeight: 31, EndHeight: 45},
		{FilePath: kthAuxFileName(testPath, 4), Version: FileV2, StartHeight: 46, EndHeight: 55},
	}, ranges)
}
