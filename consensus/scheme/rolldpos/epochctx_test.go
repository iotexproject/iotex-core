// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/state"
)

func TestNewEpochCtx(t *testing.T) {
	require := require.New(t)
	numDelegates := uint(24)
	numSubEpochs := uint(360)
	candidates := []*state.Candidate{}
	addrs := []string{}
	for i := 0; i < 24; i++ {
		addrs = append(addrs, strconv.Itoa(i))
		candidates = append(candidates, &state.Candidate{Address: strconv.Itoa(i)})
	}
	f := func(uint64) ([]*state.Candidate, error) {
		return candidates, errors.New("some error")
	}
	epoch, err := newEpochCtx(numDelegates, numSubEpochs, 1, f)
	require.Error(err)
	require.Nil(epoch)
	f = func(uint64) ([]*state.Candidate, error) {
		return candidates[:20], nil
	}
	epoch, err = newEpochCtx(numDelegates, numSubEpochs, 1, f)
	require.Error(err)
	require.Nil(epoch)
	f = func(uint64) ([]*state.Candidate, error) {
		return candidates[:24], nil
	}
	epoch, err = newEpochCtx(numDelegates, numSubEpochs, 1, f)
	require.NoError(err)
	require.NotNil(epoch)
	require.Equal(uint64(1), epoch.num)
	require.Equal(uint64(1), epoch.height)
	require.Equal(uint64(0), epoch.subEpochNum)
	crypto.SortCandidates(addrs, epoch.num, crypto.CryptoSeed)
	require.Equal(addrs, epoch.delegates)
}
