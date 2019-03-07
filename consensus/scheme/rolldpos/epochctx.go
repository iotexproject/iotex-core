// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/state"
)

// epochCtx keeps the context data for the current epoch
type epochCtx struct {
	// num is the ordinal number of an epoch
	num uint64
	// height is the first block in this epoch
	height uint64
	// subEpochNum is the ordinal number of sub-epoch within the current epoch
	subEpochNum uint64
	delegates   []string
}

func newEpochCtx(
	rp *rolldpos.Protocol,
	blockHeight uint64,
	candidatesByHeight func(uint64) ([]*state.Candidate, error),
) (*epochCtx, error) {
	epochNum := rp.GetEpochNum(blockHeight)
	epochHeight := rp.GetEpochHeight(epochNum)
	numDelegates := rp.NumDelegates()
	candidates, err := candidatesByHeight(epochHeight)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to get candidates on height %d",
			epochHeight,
		)
	}
	if len(candidates) < int(numDelegates) {
		return nil, errors.Errorf(
			"# of candidates %d is less than from required number %d",
			len(candidates),
			numDelegates,
		)
	}
	addrs := []string{}
	for i, candidate := range candidates {
		if uint64(i) >= rp.NumCandidateDelegates() {
			break
		}
		addrs = append(addrs, candidate.Address)
	}
	crypto.SortCandidates(addrs, epochNum, crypto.CryptoSeed)

	return &epochCtx{
		num:         epochNum,
		delegates:   addrs[:numDelegates],
		subEpochNum: rp.GetSubEpochNum(blockHeight),
		height:      epochHeight,
	}, nil
}
