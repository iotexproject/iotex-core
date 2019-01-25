// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package election

import (
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestVoteCarrier(t *testing.T) {
	require := require.New(t)
	carrier := NewEthereumVoteCarrier(
		"https://kovan.infura.io",
		common.HexToAddress("0xe010d3b0cb5f0685aaf8e3b044f891fc327a0eb6"),
	)

	// 10201573: deploy
	// 10201596: first staking
	lastIndex, votes, err := carrier.Votes(uint64(10201596), big.NewInt(-1), uint8(3))
	require.NoError(err)
	require.Equal(0, big.NewInt(1).Cmp(lastIndex))
	require.Equal(1, len(votes))
	require.Equal(int64(1548582024), votes[0].startTime.Unix())
	require.Equal(168*time.Hour, votes[0].duration)
	require.Equal(0, big.NewInt(100).Cmp(votes[0].amount))
	require.Equal(common.BytesToAddress([]byte{76, 217, 222, 70, 254, 208, 201, 31, 236, 193, 93, 131, 146, 70, 143, 126, 254, 227, 78, 37}), votes[0].voter)
	require.Equal("", votes[0].candidate)

	lastIndex, votes, err = carrier.Votes(uint64(10201632), big.NewInt(-1), uint8(3))
	require.NoError(err)
	require.Equal(0, big.NewInt(4).Cmp(lastIndex))
	require.Equal(3, len(votes))
	lastIndex, votes, err = carrier.Votes(uint64(10201632), lastIndex, uint8(3))
	require.NoError(err)
	require.Equal(0, big.NewInt(7).Cmp(lastIndex))
	require.Equal(3, len(votes))
	lastIndex, votes, err = carrier.Votes(uint64(10201632), lastIndex, uint8(3))
	require.NoError(err)
	require.Equal(0, big.NewInt(9).Cmp(lastIndex))
	require.Equal(2, len(votes))
}
