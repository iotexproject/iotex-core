package rolldpos

import (
	"context"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestEnableDardanellesSubEpoch(t *testing.T) {
	require := require.New(t)
	numSubEpochs := 1
	options := EnableDardanellesSubEpoch(uint64(0), uint64(numSubEpochs))
	p := NewProtocol(23, 4, 3)
	require.Nil(options(p))
	require.NotNil(options)
}

func TestEpochBlockNumber(t *testing.T) {
	require := require.New(t)
	g := genesis.Default
	dardanellesBlockHeight := g.NumDelegates*g.NumSubEpochs + 1
	wakeBlockHeight := dardanellesBlockHeight + g.NumDelegates*g.DardanellesNumSubEpochs
	p := NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
		EnableDardanellesSubEpoch(dardanellesBlockHeight, g.DardanellesNumSubEpochs),
		EnableWakeSubEpoch(wakeBlockHeight, g.WakeNumSubEpochs),
	)
	require.NotNil(p)
	require.Equal(dardanellesBlockHeight, p.dardanellesHeight)
	require.Equal(g.DardanellesNumSubEpochs, p.numSubEpochsDardanelles)
	require.Equal(wakeBlockHeight, p.wakeHeight)
	require.Equal(g.WakeNumSubEpochs, p.numSubEpochsWake)
	dardanellesEpoch := p.GetEpochNum(dardanellesBlockHeight)
	wakeEpoch := p.GetEpochNum(wakeBlockHeight)
	require.Equal(g.NumSubEpochs*g.NumDelegates, p.NumBlocksByEpoch(0))
	require.Equal(g.DardanellesNumSubEpochs*g.NumDelegates, p.NumBlocksByEpoch(dardanellesEpoch))
	require.Equal(g.WakeNumSubEpochs*g.NumDelegates, p.NumBlocksByEpoch(wakeEpoch))
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

	ctrl := gomock.NewController(t)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	arg1Num, err := strconv.ParseUint(string(arg1), 10, 64)
	require.NoError(err)

	for i, method := range methods {

		if i != 0 && i != 1 {
			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1, arg2)
			require.Nil(result)
			require.Error(err)
		}

		switch method {

		case "NumCandidateDelegates":
			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.numCandidateDelegates, 10), string(result))
			require.NoError(err)

		case "NumDelegates":
			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.numDelegates, 10), string(result))
			require.NoError(err)

		case "NumSubEpochs":
			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.NumSubEpochs(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochNumber":

			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochNum(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochHeight":

			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochHeight(arg1Num), 10), string(result))
			require.NoError(err)

		case "EpochLastHeight":

			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetEpochLastBlockHeight(arg1Num), 10), string(result))
			require.NoError(err)

		case "SubEpochNumber":

			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
			require.Equal(strconv.FormatUint(p.GetSubEpochNum(arg1Num), 10), string(result))
			require.NoError(err)

		default:
			result, _, err := p.ReadState(ctx, sm, []byte(method), arg1)
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

	t.Run("wakeUpgrade", func(t *testing.T) {
		p := NewProtocol(23, 4, 3,
			EnableDardanellesSubEpoch(13, 5),
			EnableWakeSubEpoch(33, 7),
		)
		require.Equal(uint64(3), p.NumSubEpochs(0))
		require.Equal(uint64(3*4), p.NumBlocksByEpoch(0))
		require.Equal(uint64(3), p.NumSubEpochs(12))
		require.Equal(uint64(3*4), p.NumBlocksByEpoch(p.GetEpochNum(12)))
		require.Equal(uint64(5), p.NumSubEpochs(13))
		require.Equal(uint64(5*4), p.NumBlocksByEpoch(p.GetEpochNum(13)))
		require.Equal(uint64(5), p.NumSubEpochs(14))
		require.Equal(uint64(5*4), p.NumBlocksByEpoch(p.GetEpochNum(14)))
		require.Equal(uint64(7), p.NumSubEpochs(33))
		require.Equal(uint64(7*4), p.NumBlocksByEpoch(p.GetEpochNum(33)))
		require.Equal(uint64(7), p.NumSubEpochs(34))
		require.Equal(uint64(7*4), p.NumBlocksByEpoch(p.GetEpochNum(34)))
	})
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

	t.Run("wakeUpgrade", func(t *testing.T) {
		p := NewProtocol(23, 4, 5,
			EnableDardanellesSubEpoch(41, 10),
			EnableWakeSubEpoch(201, 20),
		)
		require.Equal(uint64(0), p.GetEpochNum(0))
		require.Equal(uint64(1), p.GetEpochNum(1))
		require.Equal(uint64(1), p.GetEpochNum(19))
		require.Equal(uint64(1), p.GetEpochNum(20))
		require.Equal(uint64(2), p.GetEpochNum(21))
		require.Equal(uint64(2), p.GetEpochNum(40))
		require.Equal(uint64(3), p.GetEpochNum(41))
		require.Equal(uint64(3), p.GetEpochNum(80))
		require.Equal(uint64(4), p.GetEpochNum(81))
		require.Equal(uint64(4), p.GetEpochNum(120))
		require.Equal(uint64(5), p.GetEpochNum(121))
		require.Equal(uint64(6), p.GetEpochNum(200))
		require.Equal(uint64(7), p.GetEpochNum(201))
		require.Equal(uint64(7), p.GetEpochNum(280))
		require.Equal(uint64(8), p.GetEpochNum(281))
	})
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

	t.Run("wakeUpgrade", func(t *testing.T) {
		p := NewProtocol(23, 4, 5,
			EnableDardanellesSubEpoch(41, 10),
			EnableWakeSubEpoch(201, 20),
		)
		cases := []struct {
			epochNum uint64
			height   uint64
		}{
			{0, 0}, {1, 1}, {2, 21}, {3, 41}, {4, 81}, {5, 121}, {6, 161}, {7, 201}, {8, 281}, {9, 361},
		}
		for i := range cases {
			c := &cases[i]
			require.Equal(c.height, p.GetEpochHeight(c.epochNum), "epochNum %d", c.epochNum)
		}
	})
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

	t.Run("wakeUpgrade", func(t *testing.T) {
		p := NewProtocol(23, 4, 5,
			EnableDardanellesSubEpoch(41, 10),
			EnableWakeSubEpoch(201, 20),
		)
		cases := []struct {
			epochNum uint64
			height   uint64
		}{
			{0, 0}, {1, 20}, {2, 40}, {3, 80}, {4, 120}, {5, 160}, {6, 200}, {7, 280}, {8, 360}, {9, 440},
		}
		for i := range cases {
			c := &cases[i]
			require.Equal(c.height, p.GetEpochLastBlockHeight(c.epochNum), "epochNum %d", c.epochNum)
		}
	})
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
	if epochStartHeight == 0 || epochEndHeight == 0 {
		return nil, errors.New("productivity error")
	}
	return map[string]uint64{"ret": 0}, nil
}

func TestProductivityByEpoch(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(23, 4, 3)

	t.Run("normal call", func(t *testing.T) {
		epochNum := uint64(1)
		tipHeight := uint64(1)
		expectedHeights := uint64(1)
		expectedProduces := map[string]uint64{"ret": 0}
		retHeight, retProduce, retError := p.ProductivityByEpoch(epochNum, tipHeight, productivity)
		require.Equal(retHeight, expectedHeights)
		require.Equal(retProduce, expectedProduces)
		require.NoError(retError)
	})

	t.Run("tipHeight param error", func(t *testing.T) {
		epochNum := uint64(0)
		tipHeight := uint64(0)
		expectedHeights := uint64(0)
		expectedProduces := map[string]uint64{}
		retHeight, retProduce, retError := p.ProductivityByEpoch(epochNum, tipHeight, productivity)
		require.Equal(retHeight, expectedHeights)
		require.Equal(retProduce, expectedProduces)
		require.NoError(retError)
	})

	t.Run("epochNum param error", func(t *testing.T) {
		epochNum := uint64(2)
		tipHeight := uint64(12)
		expectedHeights := uint64(0)
		var expectedProduces = map[string]uint64{}
		expectedProduces = nil
		expectedErrors := errors.New("epoch number 2 is larger than current epoch number 1")
		retHeight, retProduce, retError := p.ProductivityByEpoch(epochNum, tipHeight, productivity)
		require.Equal(retHeight, expectedHeights)
		require.Equal(retProduce, expectedProduces)
		require.EqualError(retError, expectedErrors.Error())
	})

	t.Run("productivity param error", func(t *testing.T) {
		epochNum := uint64(0)
		tipHeight := uint64(1)
		expectedHeights := uint64(1)
		var expectedProduces = map[string]uint64{}
		expectedProduces = nil
		expectedErrors := errors.New("productivity error")
		retHeight, retProduce, retError := p.ProductivityByEpoch(epochNum, tipHeight, productivity)
		require.Equal(retHeight, expectedHeights)
		require.Equal(retProduce, expectedProduces)
		require.EqualError(retError, expectedErrors.Error())
	})
}
