package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
)

type stubContract struct {
	getStateValue     []byte
	getStateErr       error
	getCommittedValue []byte
	getCommittedErr   error
	witness           *ContractStorageWitness
	witnessErr        error
	setStateErr       error
	setStateCalls     int
	getStateCalls     int
	getCommittedCalls int
	commitCalls       int
}

func (s *stubContract) GetCommittedState(hash.Hash256) ([]byte, error) {
	s.getCommittedCalls++
	return s.getCommittedValue, s.getCommittedErr
}

func (s *stubContract) GetState(hash.Hash256) ([]byte, error) {
	s.getStateCalls++
	return s.getStateValue, s.getStateErr
}

func (s *stubContract) SetState(hash.Hash256, []byte) error {
	s.setStateCalls++
	return s.setStateErr
}

func (s *stubContract) BuildStorageWitness(ContractStorageAccess) (*ContractStorageWitness, error) {
	return s.witness, s.witnessErr
}

func (s *stubContract) GetCode() ([]byte, error)     { return nil, nil }
func (s *stubContract) SetCode(hash.Hash256, []byte) {}
func (s *stubContract) SelfState() *state.Account    { return &state.Account{} }
func (s *stubContract) Commit() error {
	s.commitCalls++
	return nil
}
func (s *stubContract) LoadRoot() error                  { return nil }
func (s *stubContract) Iterator() (trie.Iterator, error) { return nil, nil }
func (s *stubContract) Snapshot() Contract               { return s }

func TestContractDryrunHybridRoutesExecutionAndWitnessSeparately(t *testing.T) {
	require := require.New(t)

	proof := &stubContract{
		witness: &ContractStorageWitness{
			Entries:    []ContractStorageWitnessEntry{{Key: _k1b, Value: _v1b[:]}},
			ProofNodes: [][]byte{[]byte("proof")},
		},
	}
	erigon := &stubContract{
		getStateValue:     _v2b[:],
		getCommittedValue: _v3b[:],
	}
	hybrid := &contractDryrunHybrid{
		proof:  proof,
		erigon: erigon,
	}

	value, err := hybrid.GetState(_k1b)
	require.NoError(err)
	require.Equal(_v2b[:], value)
	require.Equal(1, proof.getStateCalls)
	require.Equal(1, erigon.getStateCalls)

	committed, err := hybrid.GetCommittedState(_k1b)
	require.NoError(err)
	require.Equal(_v3b[:], committed)
	require.Equal(1, proof.getCommittedCalls)
	require.Equal(1, erigon.getCommittedCalls)

	require.NoError(hybrid.SetState(_k1b, _v4b[:]))
	require.Equal(1, proof.setStateCalls)
	require.Equal(1, erigon.setStateCalls)

	witness, err := hybrid.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.NoError(err)
	require.NotNil(witness)
	require.Len(witness.Entries, 1)

	require.NoError(hybrid.Commit())
	require.Equal(0, proof.commitCalls)
	require.Equal(1, erigon.commitCalls)
}

func TestContractDryrunHybridReturnsStoredWitnessError(t *testing.T) {
	require := require.New(t)

	proof := &stubContract{getStateErr: trie.ErrInvalidTrie}
	erigon := &stubContract{getStateValue: _v1b[:]}
	hybrid := &contractDryrunHybrid{
		proof:  proof,
		erigon: erigon,
	}

	_, err := hybrid.GetState(_k1b)
	require.NoError(err)

	witness, err := hybrid.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.Nil(witness)
	require.ErrorIs(err, trie.ErrInvalidTrie)
}

func TestContractDryrunHybridSurfacesSetStateWitnessError(t *testing.T) {
	require := require.New(t)

	proof := &stubContract{setStateErr: trie.ErrInvalidTrie}
	erigon := &stubContract{}
	hybrid := &contractDryrunHybrid{
		proof:  proof,
		erigon: erigon,
	}

	require.NoError(hybrid.SetState(_k1b, _v1b[:]))
	require.Equal(1, proof.setStateCalls)
	require.Equal(1, erigon.setStateCalls)

	witness, err := hybrid.BuildStorageWitness(ContractStorageAccess{
		Reads: []common.Hash{common.BytesToHash(_k1b[:])},
	})
	require.Nil(witness)
	require.ErrorIs(err, trie.ErrInvalidTrie)
}
