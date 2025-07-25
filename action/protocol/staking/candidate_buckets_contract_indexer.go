package staking

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	CandidateListWrapper struct {
		Candidates *iotextypes.CandidateListV2
	}

	VoteBucketListWrapper struct {
		Buckets *iotextypes.VoteBucketList
	}

	CandidatesBucketsContractIndexer struct{}
)

func (w CandidateListWrapper) StorageAddr(ns string, key []byte) hash.Hash160 {
	return hash.Hash160(systemcontracts.SystemContracts[systemcontracts.CandidateListV2Storage].Address.Bytes())
}

func (w CandidateListWrapper) ErigonOnly() {}

func (w *CandidateListWrapper) StoreToContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	if ns != StakingCandidatesNamespace {
		return errors.Errorf("invalid namespace %s, expected %s", ns, StakingCandidatesNamespace)
	}
	candidateListContract, err := systemcontracts.NewCandidateListV2StorageContract(
		common.Address(w.StorageAddr(ns, key)),
		backend,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create candidate list v2 storage contract")
	}
	if err := candidateListContract.PutCandidates(w.Candidates.Candidates); err != nil {
		return errors.Wrap(err, "failed to put candidates to contract")
	}
	return nil
}

// LoadFromContract loads probation list from the contract storage
func (w *CandidateListWrapper) LoadFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	if ns != StakingCandidatesNamespace {
		return errors.Errorf("invalid namespace %s, expected %s", ns, StakingCandidatesNamespace)
	}
	candidateListContract, err := systemcontracts.NewCandidateListV2StorageContract(
		common.Address(w.StorageAddr(ns, key)),
		backend,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create candidate list v2 storage contract")
	}
	// recover offset and limit from key
	offset := byteutil.BytesToUint64(key[:8])
	limit := byteutil.BytesToUint64(key[8:])
	candidates, err := candidateListContract.GetCandidates(offset, limit)
	if err != nil {
		return errors.Wrap(err, "failed to get candidates from contract")
	}
	w.Candidates = &iotextypes.CandidateListV2{
		Candidates: candidates.Candidates,
	}
	return nil
}

// DeleteFromContract deletes probation list from the contract storage
func (w *CandidateListWrapper) DeleteFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	return errors.New("not implemented")
}

// ListFromContract lists all probation data from the contract storage
func (w *CandidateListWrapper) ListFromContract(_ string, _ systemcontracts.ContractBackend) ([][]byte, []any, error) {
	return nil, nil, errors.New("not implemented")
}

// BatchFromContract retrieves multiple probation lists from the contract storage
func (w *CandidateListWrapper) BatchFromContract(_ string, _ [][]byte, _ systemcontracts.ContractBackend) ([]any, error) {
	return nil, errors.New("not implemented")
}

func (w VoteBucketListWrapper) StorageAddr(ns string, key []byte) hash.Hash160 {
	return hash.Hash160(systemcontracts.SystemContracts[systemcontracts.VoteBucketStorage].Address.Bytes())
}

func (w VoteBucketListWrapper) ErigonOnly() {}

func (w *VoteBucketListWrapper) StoreToContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	if ns != StakingBucketsNamespace {
		return errors.Errorf("invalid namespace %s, expected %s", ns, StakingBucketsNamespace)
	}
	voteBucketListContract, err := systemcontracts.NewVoteBucketStorageContract(
		common.Address(w.StorageAddr(ns, key)),
		backend,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create vote bucket list storage contract")
	}
	if err := voteBucketListContract.PutBuckets(w.Buckets.Buckets); err != nil {
		return errors.Wrap(err, "failed to put vote buckets to contract")
	}
	return nil
}

// LoadFromContract loads probation list from the contract storage
func (w *VoteBucketListWrapper) LoadFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	if ns != StakingBucketsNamespace {
		return errors.Errorf("invalid namespace %s, expected %s", ns, StakingBucketsNamespace)
	}
	voteBucketListContract, err := systemcontracts.NewVoteBucketStorageContract(
		common.Address(w.StorageAddr(ns, key)),
		backend,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create vote bucket storage contract")
	}
	// recover offset and limit from key
	offset := byteutil.BytesToUint64(key[:8])
	limit := byteutil.BytesToUint64(key[8:])
	buckets, err := voteBucketListContract.GetBuckets(offset, limit)
	if err != nil {
		return errors.Wrap(err, "failed to get buckets from contract")
	}
	w.Buckets = &iotextypes.VoteBucketList{
		Buckets: buckets.Buckets,
	}
	return nil
}

// DeleteFromContract deletes probation list from the contract storage
func (w *VoteBucketListWrapper) DeleteFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	return errors.New("not implemented")
}

// ListFromContract lists all probation data from the contract storage
func (w *VoteBucketListWrapper) ListFromContract(_ string, _ systemcontracts.ContractBackend) ([][]byte, []any, error) {
	return nil, nil, errors.New("not implemented")
}

// BatchFromContract retrieves multiple probation lists from the contract storage
func (w *VoteBucketListWrapper) BatchFromContract(_ string, _ [][]byte, _ systemcontracts.ContractBackend) ([]any, error) {
	return nil, errors.New("not implemented")
}

func newCandidatesBucketsContractIndexer() *CandidatesBucketsContractIndexer {
	return &CandidatesBucketsContractIndexer{}
}

func (cbi *CandidatesBucketsContractIndexer) PutCandidates(sm protocol.StateManager, candidates *iotextypes.CandidateListV2) error {
	_, err := sm.PutState(&CandidateListWrapper{Candidates: candidates}, protocol.NamespaceOption(StakingCandidatesNamespace))
	return err
}

func (cbi *CandidatesBucketsContractIndexer) GetCandidates(sr protocol.StateReader, offset, limit uint32) (*iotextypes.CandidateListV2, uint64, error) {
	var wrapper CandidateListWrapper
	// serialize offset and limit into a key, which is able to recover the offset and limit
	// offset and limit are uint32, so we can use the first 4 bytes for offset and the second 4 bytes for limit
	key := make([]byte, 16)
	copy(key[:8], byteutil.Uint64ToBytes(uint64(offset)))
	copy(key[8:], byteutil.Uint64ToBytes(uint64(limit)))
	height, err := sr.State(&wrapper, protocol.NamespaceOption(StakingCandidatesNamespace), protocol.KeyOption(key))
	if err != nil {
		return nil, 0, err
	}
	return wrapper.Candidates, height, nil
}

func (cbi *CandidatesBucketsContractIndexer) PutBuckets(sm protocol.StateManager, buckets *iotextypes.VoteBucketList) error {
	_, err := sm.PutState(&VoteBucketListWrapper{Buckets: buckets}, protocol.NamespaceOption(StakingBucketsNamespace))
	return err
}

func (cbi *CandidatesBucketsContractIndexer) GetBuckets(sr protocol.StateReader, offset, limit uint32) (*iotextypes.VoteBucketList, uint64, error) {
	var wrapper VoteBucketListWrapper
	// serialize offset and limit into a key, which is able to recover the offset and limit
	key := make([]byte, 16)
	copy(key[:8], byteutil.Uint64ToBytes(uint64(offset)))
	copy(key[8:], byteutil.Uint64ToBytes(uint64(limit)))
	height, err := sr.State(&wrapper, protocol.NamespaceOption(StakingBucketsNamespace), protocol.KeyOption(key))
	if err != nil {
		return nil, 0, err
	}
	return wrapper.Buckets, height, nil
}
