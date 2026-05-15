package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
)

func TestVerifyContractStorageWitness(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	cntr, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)

	require.NoError(cntr.SetState(_k1b, _v1b[:]))
	require.NoError(cntr.SetState(_k2b, _v2b[:]))
	require.NoError(cntr.Commit())
	_, err = cntr.GetState(_k1b)
	require.NoError(err)
	_, err = cntr.GetState(_k4b)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	require.NoError(cntr.SetState(_k1b, _v3b[:]))
	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{
			common.BytesToHash(_k1b[:]),
			common.BytesToHash(_k4b[:]),
		},
	})
	require.NoError(err)

	err = VerifyContractStorageWitness(common.BytesToAddress(addr[:]), witness)
	require.NoError(err)
}

func TestVerifyContractStorageWitnessRejectsTamperedValue(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	cntr, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)

	require.NoError(cntr.SetState(_k1b, _v1b[:]))
	require.NoError(cntr.Commit())
	_, err = cntr.GetState(_k1b)
	require.NoError(err)

	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)
	witness.Entries[0].Value = _v2b[:]

	err = VerifyContractStorageWitness(common.BytesToAddress(addr[:]), witness)
	require.Error(err)
	require.Contains(err.Error(), "value mismatch")
}

func TestVerifyContractStorageWitnessRejectsTamperedProof(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	addr := hash.BytesToHash160(_c1[:])
	cntr, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(err)

	require.NoError(cntr.SetState(_k1b, _v1b[:]))
	require.NoError(cntr.Commit())
	_, err = cntr.GetState(_k1b)
	require.NoError(err)

	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)
	require.NotEmpty(witness.ProofNodes)
	witness.ProofNodes[0] = append([]byte(nil), witness.ProofNodes[0]...)
	witness.ProofNodes[0][len(witness.ProofNodes[0])-1] ^= 0x01

	err = VerifyContractStorageWitness(common.BytesToAddress(addr[:]), witness)
	require.Error(err)
	require.ErrorIs(err, trie.ErrInvalidTrie)
}

func TestVerifyContractStorageWitnessAcceptsEmptyRootAbsence(t *testing.T) {
	require := require.New(t)

	witness := &ContractStorageWitness{
		StorageRoot: hash.ZeroHash256,
		Entries: []ContractStorageWitnessEntry{
			{
				Key: _k1b,
			},
		},
		ProofNodes: [][]byte{{0x12, 0x00}},
	}

	err := VerifyContractStorageWitness(common.BytesToAddress(_c1[:]), witness)
	require.NoError(err)
}
