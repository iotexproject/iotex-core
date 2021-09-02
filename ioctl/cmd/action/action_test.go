package action

import (
	"testing"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/stretchr/testify/require"
)

func TestChainID(t *testing.T) {
	r := require.New(t)

	testCases := []struct {
		endPoint string
		chainID  uint32
	}{
		{
			"",
			0,
		},
		{
			"apiiotex.one",
			0,
		},
		{
			"api.iotex.one",
			1,
		},
		{
			"api.mainnet.iotex.one",
			1,
		},
		{
			"api.testnet.iotex.one",
			2,
		},
		{
			"api.nightly-cluster-2.iotex.one",
			3,
		},
		{
			"api.nightly-cluster-1.iotex.one",
			0,
		},
	}
	for _, c := range testCases {
		config.ReadConfig.Endpoint = c.endPoint
		r.Equal(getChainID(), c.chainID)
	}

}
