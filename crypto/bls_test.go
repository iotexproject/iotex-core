// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTBLS(t *testing.T) {
	require := require.New(t)
	var err error
	idList := make([][]uint8, numnodes)
	skList := make([][]uint32, numnodes)
	askList := make([][]uint32, numnodes)
	coeffsList := make([][][]uint32, numnodes)
	sharesList := make([][][]uint32, numnodes)
	shares := make([][]uint32, numnodes)
	qsList := make([][]byte, numnodes)
	pkList := make([][]byte, numnodes)
	sigList := make([][]byte, numnodes)
	witnessesList := make([][][]byte, numnodes)
	sharestatusmatrix := make([][numnodes]bool, numnodes)

	// Generate 21 identifiers for the delegates
	for i := 0; i < numnodes; i++ {
		idList[i] = RndGenerate()
	}

	// Initialize DKG and generate secret shares
	for i := 0; i < numnodes; i++ {
		skList[i] = DKG.SkGeneration()
		coeffsList[i], sharesList[i], witnessesList[i], err = DKG.Init(skList[i], idList)
		require.NoError(err)
	}

	// Verify all the received secret shares
	for i := 0; i < numnodes; i++ {
		for j := 0; j < numnodes; j++ {
			shares[j] = sharesList[j][i]
		}
		sharestatusmatrix[i], err = DKG.SharesCollect(idList[i], shares, witnessesList)
		require.NoError(err)
		for _, b := range sharestatusmatrix[i] {
			require.True(b)
		}
	}

	// Generate private and public key shares of a group key
	for i := 0; i < numnodes; i++ {
		for j := 0; j < numnodes; j++ {
			shares[j] = sharesList[j][i]
		}
		qsList[i], pkList[i], askList[i], err = DKG.KeyPairGeneration(shares, sharestatusmatrix)
		require.NoError(err)
	}

	// Generate signature shares
	message := []byte("hello iotex message")
	for i := 0; i < numnodes; i++ {
		var ok bool
		ok, sigList[i], err = BLS.SignShare(askList[i], message)
		require.NoError(err)
		require.True(ok)
	}

	// Randomly select BLS signature shares for aggregation
	selected := make([]int, numnodes)
	for c := 0; c < 20; c++ {
		check := make(map[int]bool)
		for len(check) < degree+1 {
			r := rand.Intn(numnodes)
			if _, ok := check[r]; !ok {
				selected[len(check)] = r
				check[r] = true
			}
		}
		selectedID := make([][]uint8, degree+1)
		selectedSig := make([][]byte, degree+1)
		selectedPK := make([][]byte, degree+1)
		for j := 0; j < degree+1; j++ {
			selectedID[j] = idList[selected[j]]
			selectedSig[j] = sigList[selected[j]]
			selectedPK[j] = pkList[selected[j]]
		}
		aggsig, err := BLS.SignAggregate(selectedID, selectedSig)
		require.NoError(err)
		err = BLS.VerifyAggregate(selectedID, selectedPK, message, aggsig)
		require.NoError(err)
	}
}
