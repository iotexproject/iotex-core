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

func TestBuildStorageWitness(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	acct := &state.Account{}
	cntr, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, false)
	require.NoError(err)

	require.NoError(cntr.SetState(_k1b, _v1b[:]))
	require.NoError(cntr.SetState(_k2b, _v2b[:]))
	require.NoError(cntr.Commit())

	preRoot := cntr.SelfState().Root

	_, err = cntr.GetState(_k1b)
	require.NoError(err)
	require.NoError(cntr.SetState(_k1b, _v3b[:]))
	require.NoError(cntr.SetState(_k4b, _v4b[:]))

	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{
		Reads:  []common.Hash{common.BytesToHash(_k1b[:]), common.BytesToHash(_k4b[:])},
		Writes: []common.Hash{common.BytesToHash(_k4b[:]), common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)
	require.Equal(preRoot, witness.StorageRoot)
	require.Len(witness.Entries, 2)
	require.Equal(_k1b, witness.Entries[0].Key)
	require.Equal(_v1b[:], witness.Entries[0].Value)
	require.Equal(_k4b, witness.Entries[1].Key)
	require.Nil(witness.Entries[1].Value)
	require.NotEmpty(witness.ProofNodes)
}

func TestBuildStorageWitnessRequiresCommittedPrestate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	acct := &state.Account{}
	cntr, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, false)
	require.NoError(err)

	_, err = cntr.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.Error(err)
	require.Contains(err.Error(), "missing committed pre-state")
}

func TestContractGetStateMissingSlot(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	sm, err := initMockStateManager(ctrl)
	require.NoError(err)

	acct := &state.Account{}
	cntr, err := newContract(hash.BytesToHash160(_c1[:]), acct, sm, false)
	require.NoError(err)

	value, err := cntr.GetState(_k4b)
	require.Nil(value)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k4b[:])},
	})
	require.NoError(err)
	require.Len(witness.Entries, 1)
	require.Equal(_k4b, witness.Entries[0].Key)
	require.Nil(witness.Entries[0].Value)
}
