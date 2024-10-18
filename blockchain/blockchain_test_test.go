package blockchain_test

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
)

func TestProductivity(t *testing.T) {
	r := require.New(t)
	c := gomock.NewController(t)
	defer c.Finish()

	bc := mock_blockchain.NewMockBlockchain(c)

	t.Run("FailedToGetBlockHeader", func(t *testing.T) {
		bc.EXPECT().BlockHeaderByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
		_, err := blockchain.Productivity(bc, 1, 1)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		p.Reset()

		times := 2
		producer := "any"

		bc.EXPECT().BlockHeaderByHeight(gomock.Any()).Return(&block.Header{}, nil).Times(times)
		p = p.ApplyMethodReturn(&block.Header{}, "ProducerAddress", producer)

		stats, err := blockchain.Productivity(bc, 1, 2)
		r.NoError(err)
		r.Len(stats, 1)
		r.Equal(stats[producer], uint64(2))
	})
}
