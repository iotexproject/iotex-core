// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockExplorerApi(t *testing.T) {
	require := require.New(t)

	svc := MockExplorer{}

	_, err := svc.GetBlockchainHeight()
	require.Nil(err)

	_, err = svc.GetAddressBalance("")
	require.Nil(err)

	_, err = svc.GetAddressDetails("")
	require.Nil(err)

	_, err = svc.GetLastTransfersByRange(0, 0, 10, true)
	require.Nil(err)

	_, err = svc.GetTransferByID("")
	require.Nil(err)

	_, err = svc.GetTransfersByAddress("", 0, 10)
	require.Nil(err)

	_, err = svc.GetTransfersByBlockID("", 0, 10)
	require.Nil(err)

	_, err = svc.GetLastVotesByRange(0, 0, 10)
	require.Nil(err)

	_, err = svc.GetVoteByID("")
	require.Nil(err)

	_, err = svc.GetVotesByAddress("", 0, 10)
	require.Nil(err)

	_, err = svc.GetVotesByBlockID("", 0, 10)
	require.Nil(err)

	_, err = svc.GetLastBlocksByRange(0, 10)
	require.Nil(err)

	_, err = svc.GetBlockByID("")
	require.Nil(err)

	_, err = svc.GetCoinStatistic()
	require.Nil(err)

	_, err = svc.GetConsensusMetrics()
	require.Nil(err)

	_, err = svc.GetPeers()
	require.Nil(err)

	randInt64 := randInt64()
	require.NotNil(randInt64)

	randString := randString()
	require.NotNil(randString)

	randTransaction := randTransaction()
	require.NotNil(randTransaction)

	randVote := randVote()
	require.NotNil(randVote)

	randBlock := randBlock()
	require.NotNil(randBlock)
}
