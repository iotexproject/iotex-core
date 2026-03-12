// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"os"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// ShardRange describes the stored height range of a chain db shard file.
type ShardRange struct {
	FilePath    string
	Version     string
	StartHeight uint64
	EndHeight   uint64
}

// InspectShardRanges opens a filedao database and reports each shard file and its stored height range.
func InspectShardRanges(cfg db.Config, deser *block.Deserializer) ([]ShardRange, error) {
	dao, err := NewFileDAO(cfg, deser)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if err := dao.Start(ctx); err != nil {
		return nil, err
	}
	defer dao.Stop(ctx)

	fd, ok := dao.(*fileDAO)
	if !ok {
		return nil, errors.New("unexpected filedao type")
	}

	var ranges []ShardRange
	if fd.legacyFd != nil {
		legacy, ok := fd.legacyFd.(*fileDAOLegacy)
		if !ok {
			return nil, errors.New("unexpected legacy filedao type")
		}
		legacyRanges, err := inspectLegacyShardRanges(cfg.DbPath, legacy)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, legacyRanges...)
	}
	if fd.v2Fd != nil {
		for _, file := range fd.v2Fd.Indices {
			ranges = append(ranges, ShardRange{
				FilePath:    file.fd.filename,
				Version:     FileV2,
				StartHeight: file.start,
				EndHeight:   file.end,
			})
		}
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartHeight == ranges[j].StartHeight {
			return ranges[i].FilePath < ranges[j].FilePath
		}
		return ranges[i].StartHeight < ranges[j].StartHeight
	})
	return ranges, nil
}

func inspectLegacyShardRanges(base string, fd *fileDAOLegacy) ([]ShardRange, error) {
	tip, err := fd.Height()
	if err != nil {
		return nil, err
	}
	type auxFile struct {
		index uint64
		path  string
	}
	auxFiles := make([]auxFile, 0)
	topIndex, _ := fd.topIndex.Load().(uint64)
	for index := uint64(1); index <= topIndex; index++ {
		file := kthAuxFileName(base, index)
		if _, err := os.Stat(file); err != nil {
			continue
		}
		auxFiles = append(auxFiles, auxFile{index: index, path: file})
	}
	sort.Slice(auxFiles, func(i, j int) bool { return auxFiles[i].index < auxFiles[j].index })

	if len(auxFiles) == 0 {
		return []ShardRange{{
			FilePath:    base,
			Version:     FileLegacyMaster,
			StartHeight: 1,
			EndHeight:   tip,
		}}, nil
	}

	starts := make([]uint64, len(auxFiles))
	low := fd.cfg.SplitDBHeight + 1
	if low == 0 {
		low = 1
	}
	for i, file := range auxFiles {
		start, err := firstLegacyHeightForIndex(fd, file.index, low, tip)
		if err != nil {
			return nil, err
		}
		starts[i] = start
		low = start + 1
	}

	ranges := make([]ShardRange, 0, len(auxFiles)+1)
	ranges = append(ranges, ShardRange{
		FilePath:    base,
		Version:     FileLegacyMaster,
		StartHeight: 1,
		EndHeight:   starts[0] - 1,
	})
	for i, file := range auxFiles {
		end := tip
		if i+1 < len(starts) {
			end = starts[i+1] - 1
		}
		ranges = append(ranges, ShardRange{
			FilePath:    file.path,
			Version:     FileLegacyAuxiliary,
			StartHeight: starts[i],
			EndHeight:   end,
		})
	}
	return ranges, nil
}

func firstLegacyHeightForIndex(fd *fileDAOLegacy, index, low, high uint64) (uint64, error) {
	var found uint64
	for low <= high {
		mid := low + (high-low)/2
		value, err := fd.getFileIndex(mid)
		if err != nil {
			return 0, err
		}
		current := byteutil.BytesToUint64BigEndian(value)
		if current >= index {
			if current == index {
				found = mid
			}
			if mid == 0 {
				break
			}
			high = mid - 1
			continue
		}
		low = mid + 1
	}
	if found == 0 {
		return 0, errors.Errorf("failed to determine start height for legacy shard %d", index)
	}
	return found, nil
}
