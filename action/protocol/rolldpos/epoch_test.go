package rolldpos

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnableDardanellesSubEpoch(t *testing.T) {
	require := require.New(t)
	numSubEpochs := 1
	options := EnableDardanellesSubEpoch(uint64(0), uint64(numSubEpochs))
	p := NewProtocol(23, 4, 3)
	require.Nil(options(p))
	require.NotNil(options)
}

func TestNewProtocol(t *testing.T) {
	require := require.New(t)
	numCandidateDelegates := uint64(23)
	numDelegates := uint64(24)
	numSubEpochs := uint64(3)
	height := 0
	options := EnableDardanellesSubEpoch(uint64(height), numSubEpochs)
	require.NotNil(NewProtocol(numCandidateDelegates, numDelegates, numSubEpochs, options))
}

func TestProtocol_Handle(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	ctx := context.Background()
	receipt, error := p.Handle(ctx, nil, nil)
	require.Nil(receipt)
	require.NoError(error)
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

	arg1 := []byte("10")
	arg2 := []byte("20")

	arg1Num, err := strconv.ParseUint(string(arg1), 10, 64)
	require.NoError(err)

	for i, method := range methods {

		if i != 0 && i != 1 {
			result, err := p.ReadState(ctx, nil, []byte(method), arg1, arg2)
			require.Nil(result)
			require.Error(err)
		}

		switch method {

		case "NumCandidateDelegates":
			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.numCandidateDelegates, 10), string(result))
			require.NoError(err)

		case "NumDelegates":
			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.numDelegates, 10), string(result))
			require.NoError(err)

		case "NumSubEpochs":
			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.NumSubEpochs(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochNumber":

			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochNum(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochHeight":

			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochHeight(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochLastHeight":

			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochLastBlockHeight(arg1Num), 10), string(result))
			require.NoError(err)

		case "SubEpochNumber":

			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetSubEpochNum(arg1Num), 10), string(result))
			require.NoError(err)

		default:
			result, err := p.ReadState(ctx, nil, []byte(method), arg1)
			require.Nil(result)
			require.Error(err)

		}

	}

}

func TestProtocol_NumSubEpochs(t *testing.T) {

	require := require.New(t)

	height := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}

	expectedP := []uint64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	for i := 0; i < len(height); i++ {
		p1 := NewProtocol(23, 4, 3)
		p2 := NewProtocol(23, 4, 3)
		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = height[i]
		p2.dardanellesOn = true
		numSubEpochs := p1.NumSubEpochs(height[i])
		require.Equal(expectedP[i], numSubEpochs)
		numSubEpochs = p2.NumSubEpochs(height[i])
		require.Equal(expectedP[i], numSubEpochs)
	}

}

func TestGetEpochNum(t *testing.T) {
	require := require.New(t)

	height := []uint64{0, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}

	expectedP1 := []uint64{0, 1, 1, 3, 4, 5, 5, 7, 8, 8, 10}
	expectedP2 := []uint64{0, 1, 1, 3, 4, 5, 5, 7, 8, 8, 10}
	expectedP3 := []uint64{0, 0, 1, 2, 3, 4, 4, 6, 7, 7, 10}

	//If only the modification of function EnableDardanellesSubEpoch to Protocol，
	//then function GetEpochNum won't jump out of the second if block
	//If dardanellesOn =true, and height <= p3.dardanellesHeight，
	//then p3.numSubEpochsDardanelles can't be 0.
	//Assume that in addition of function EnableDardanellesSubEpoch,
	//there are other function that can assign values to numSubEpochsDardanelles
	for i := 0; i < len(height); i++ {
		p1 := NewProtocol(23, 4, 3)

		p2 := NewProtocol(23, 4, 3)
		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = height[i]
		p2.dardanellesOn = true

		p3 := NewProtocol(23, 4, 3)
		p3.dardanellesOn = true
		p3.numSubEpochsDardanelles = 3

		epochNum := p1.GetEpochNum(height[i])
		require.Equal(expectedP1[i], epochNum)

		epochNum = p2.GetEpochNum(height[i])
		require.Equal(expectedP2[i], epochNum)

		epochNum = p3.GetEpochNum(height[i])
		require.Equal(expectedP3[i], epochNum)
	}

}

func TestGetEpochHeight(t *testing.T) {

	require := require.New(t)
	epochNum := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	expectedP1 := []uint64{0, 1, 13, 25, 37, 49, 61, 73, 85, 97, 109}
	expectedP2 := []uint64{0, 12, 24, 36, 48, 60, 72, 84, 96, 108, 120}

	for i := 0; i < len(epochNum); i++ {

		p1 := NewProtocol(23, 4, 3)

		p2 := NewProtocol(23, 4, 3)
		p2.dardanellesOn = true
		p2.numSubEpochsDardanelles = p2.numSubEpochs
		p2.dardanellesHeight = 0 //Consider tha p2 doesn't meet the condition fo the second if block, ie height = 0

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

func productivity(epochStartHeight uint64, epochEndHeight uint64) (map[string]uint64, error) {
	return map[string]uint64{"ret": epochEndHeight - epochStartHeight}, nil
}

func TestProductivityByEpoch(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)
	epochNum := []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tipHeight := []uint64{1, 1, 12, 25, 38, 53, 59, 80, 90, 93, 120}

	expectedHeights := []uint64{1, 1, 0, 1, 2, 5, 0, 8, 6, 0, 12}
	var nilMap = map[string]uint64{}
	nilMap = nil
	expectedProduces := []map[string]uint64{
		{"ret": 0},
		{"ret": 0},
		nilMap,
		{"ret": 0},
		{"ret": 1},
		{"ret": 4},
		nilMap,
		{"ret": 7},
		{"ret": 5},
		nilMap,
		{"ret": 11},
	}
	expectedErrors := []error{
		nil,
		nil,
		errors.New("epoch number 2 is larger than current epoch number 1"),
		nil,
		nil,
		nil,
		errors.New("epoch number 6 is larger than current epoch number 5"),
		nil,
		nil,
		errors.New("epoch number 9 is larger than current epoch number 8"),
		nil,
	}

	for i := 0; i < len(epochNum); i++ {
		retHeight, retProduce, retError := p.ProductivityByEpoch(epochNum[i], tipHeight[i], productivity)
		require.Equal(retHeight, expectedHeights[i])
		require.Equal(retProduce, expectedProduces[i])
		if i == 2 || i == 6 || i == 9 {
			require.EqualError(retError, expectedErrors[i].Error())
		} else {
			require.NoError(retError)
		}
	}
}
