package rolldpos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)


func TestEnableDardanellesSubEpoch(t *testing.T) {
	require := require.New(t)

	height := 0
	numSubEpochs := 1

	options := EnableDardanellesSubEpoch(uint64(height), uint64(numSubEpochs))
	p := NewProtocol(23, 4, 3)
	require.Nil(options(p))
	require.NotNil(options)
}

func TestNewProtocol(t *testing.T) {
	require := require.New(t)
	numCandidateDelegates := uint64(23)
	numDelegates := uint64(24)
	numSubEpochs := uint64(3)
	height := -1
	options := EnableDardanellesSubEpoch(uint64(height), numSubEpochs)
	p := NewProtocol(numCandidateDelegates, numDelegates, numSubEpochs, options)
	require.NotNil(p)

}

func TestProtocol_Handle(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	ctx := context.Background()
	require.Nil(p.Handle(ctx, nil, nil))

}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	ctx := context.Background()
	require.Nil(p.Validate(ctx, nil))
}

func TestProtocol_NumCandidateDelegates(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	require.Equal(uint64(23), p.NumCandidateDelegates())
}

func TestProtocol_NumDelegates(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	require.Equal(uint64(4), p.NumDelegates())
}

func TestProtocol_ReadState(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)

	ctx := context.Background()
	methods := [8]string{
		"NumCandidateDelegates",
		"NumDelegates",
		"NumSubEpochs",
		"EpochNumber",
		"EpochHeight",
		"EpochLastHeight",
		"SubEpochNumber",
		"trick",
	}
	for _, method := range methods {
		_, err:=p.ReadState(ctx, nil, []byte(method), []byte("trick_arg_1"), []byte("trick_arg_2"))
		require.Error(err)
	}
	for _, method := range methods {
		_, err:=p.ReadState(ctx, nil, []byte(method), []byte("good_arg_1"))
		require.NoError(err)
	}

}

func TestProtocol_NumSubEpochs(t *testing.T) {

	require := require.New(t)

	height := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}

	expectedP1 := []uint64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
	expectedP2 := []uint64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	//分别声明两个结构体实例，分别代表已调用和未调用的EnableDardanellesSubEpoch
	for i := 1; i < len(height); i++ {
		p1 := NewProtocol(23, 4, 3)

		p2 := NewProtocol(23, 4, 3)

		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = height[i]
		p2.dardanellesOn = true

		numSubEpochs := p1.NumSubEpochs(height[i])
		require.Equal(expectedP1[i], numSubEpochs)

		numSubEpochs = p2.NumSubEpochs(height[i])
		require.Equal(expectedP2[i], numSubEpochs)

	}

}

func TestGetEpochNum(t *testing.T) {
	require := require.New(t)

	height := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}

	expectedP1 := []uint64{0, 1, 1, 3, 4, 5, 5, 7, 8, 8, 10}
	expectedP2 := []uint64{0, 1, 1, 3, 4, 5, 5, 7, 8, 8, 10}
	expectedP3 := []uint64{0, 0, 1, 2, 3, 4, 4, 6, 7, 7, 10}

	//如果只考量EnableDardanellesSubEpoch对Protocal实例的操作，那Protocal就一直不会跳出GetEpochNum的第二个if
	//如果满足dardanellesOn=true,且height <= p3.dardanellesHeight，
	//则p3.numSubEpochsDardanelles不能是0，
	//假设除了，EnableDardanellesSubEpoch，函数外，还有函数能给numSubEpochsDardanelles赋值
	for i := 1; i < len(height); i++ {
		p1 := NewProtocol(23, 4, 3)

		p2 := NewProtocol(23, 4, 3)
		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = height[i]
		p2.dardanellesOn = true

		p3 := NewProtocol(23, 4, 3)
		p3.dardanellesOn = true
		p3.numSubEpochsDardanelles = 3

		epocNum := p1.GetEpochNum(height[i])
		require.Equal(expectedP1[i], epocNum)

		epocNum = p2.GetEpochNum(height[i])
		require.Equal(expectedP2[i], epocNum)

		epocNum = p3.GetEpochNum(height[i])
		require.Equal(expectedP3[i], epocNum)

	}

}

func TestGetEpochHeight(t *testing.T) {

	require := require.New(t)
	epochNum := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	expectedP1 := []uint64{0, 1, 13, 25, 37, 49, 61, 73, 85, 97, 109}
	expectedP2 := []uint64{0, 12, 24, 36, 48, 60, 72, 84, 96, 108, 120}

	for i := 1; i < len(epochNum); i++ {

		p1 := NewProtocol(23, 4, 3)

		p2 := NewProtocol(23, 4, 3)
		p2.dardanellesOn = true
		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = 0 //考虑p2不符合第二个if块的情况，即此时，height=0

		epochHeight := p1.GetEpochHeight(epochNum[i])
		require.Equal(expectedP1[i], epochHeight)

		epochHeight = p2.GetEpochHeight(epochNum[i])
		require.Equal(expectedP2[i], epochHeight)
	}

}

func TestGetEpochLastBlockHeight(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)

	epochNums := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedHeights := []uint64{0, 12, 24, 36, 48, 60, 72, 84, 96, 108, 120}
	for i, epochNum := range epochNums {
		height := p.GetEpochLastBlockHeight(epochNum)
		require.Equal(expectedHeights[i], height)
	}
}

func TestGetSubEpochNum(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	epochHeights := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}
	expectedSubEpochNums := []uint64{0, 0, 2, 0, 0, 1, 2, 1, 1, 2, 2}
	for i, epochHeight := range epochHeights {
		subEpochNum := p.GetSubEpochNum(epochHeight)
		require.Equal(expectedSubEpochNums[i], subEpochNum)
	}
}
