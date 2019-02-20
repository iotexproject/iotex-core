package rolldpos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEpochNum(t *testing.T) {
	require := require.New(t)
	numDelegates := uint64(4)
	numSubEpochs := uint64(3)
	epochHeights := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}
	expectedNums := []uint64{0, 1, 1, 3, 4, 5, 5, 7, 8, 8, 10}
	for i, epochHeight := range epochHeights {
		num := GetEpochNum(epochHeight, numDelegates, numSubEpochs)
		require.Equal(expectedNums[i], num)
	}
}

func TestGetEpochHeight(t *testing.T) {
	require := require.New(t)
	numDelegates := uint64(4)
	numSubEpochs := uint64(3)
	epochNums := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedHeights := []uint64{0, 1, 13, 25, 37, 49, 61, 73, 85, 97, 109}
	for i, epochNum := range epochNums {
		height := GetEpochHeight(epochNum, numDelegates, numSubEpochs)
		require.Equal(expectedHeights[i], height)
	}
}

func TestGetEpochLastBlockHeight(t *testing.T) {
	require := require.New(t)
	numDelegates := uint64(4)
	numSubEpochs := uint64(3)
	epochNums := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedHeights := []uint64{0, 12, 24, 36, 48, 60, 72, 84, 96, 108, 120}
	for i, epochNum := range epochNums {
		height := GetEpochLastBlockHeight(epochNum, numDelegates, numSubEpochs)
		require.Equal(expectedHeights[i], height)
	}
}

func TestGetSubEpochNum(t *testing.T) {
	require := require.New(t)
	numDelegates := uint64(4)
	numSubEpochs := uint64(3)
	epochHeights := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}
	expectedSubEpochNums := []uint64{0, 0, 2, 0, 0, 1, 2, 1, 1, 2, 2}
	for i, epochHeight := range epochHeights {
		subEpochNum := GetSubEpochNum(epochHeight, numDelegates, numSubEpochs)
		require.Equal(expectedSubEpochNums[i], subEpochNum)
	}
}
