// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

// GetEpochNum returns the epoch number
func GetEpochNum(
	height uint64,
	numDelegates uint64,
	numSubEpochs uint64,
) uint64 {
	if height == 0 {
		return 0
	}
	return (height-1)/numDelegates/numSubEpochs + 1
}

// GetSubEpochNum returns the sub epoch number of a block height
func GetSubEpochNum(
	height uint64,
	numDelegates uint64,
	numSubEpochs uint64,
) uint64 {
	if height == 0 {
		return 0
	}
	return (height - 1) % (numDelegates * numSubEpochs) / numDelegates
}

// GetEpochHeight returns the epoch start block height
func GetEpochHeight(
	epochNum uint64,
	numDelegates uint64,
	numSubEpochs uint64,
) uint64 {
	if epochNum == 0 {
		return 0
	}
	return (epochNum-1)*numDelegates*numSubEpochs + 1
}

// GetEpochLastBlockHeight returns the epoch last block height
func GetEpochLastBlockHeight(
	epochNum uint64,
	numDelegates uint64,
	numSubEpochs uint64,
) uint64 {
	if epochNum == 0 {
		return 0
	}
	return epochNum * numDelegates * numSubEpochs
}
