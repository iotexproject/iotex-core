package contractstaking

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"

	"github.com/iotexproject/iotex-address/address"
)

// ContractStakingStateReader wraps a state reader to provide staking contract-specific reads.
type ContractStakingStateReader struct {
	sr protocol.StateReader
}

// NewStateReader creates a new ContractStakingStateReader.
func NewStateReader(sr protocol.StateReader) *ContractStakingStateReader {
	return &ContractStakingStateReader{
		sr: sr,
	}
}

func contractNamespaceOption(contractAddr address.Address) protocol.StateOption {
	return protocol.NamespaceOption(fmt.Sprintf("%s%x", state.ContractStakingBucketNamespacePrefix, contractAddr.Bytes()))
}

func bucketTypeNamespaceOption(contractAddr address.Address) protocol.StateOption {
	return protocol.NamespaceOption(fmt.Sprintf("%s%x", state.ContractStakingBucketTypeNamespacePrefix, contractAddr.Bytes()))
}

func contractKeyOption(contractAddr address.Address) protocol.StateOption {
	return protocol.KeyOption(contractAddr.Bytes())
}

func bucketIDKeyOption(bucketID uint64) protocol.StateOption {
	return protocol.KeyOption(byteutil.Uint64ToBytes(bucketID))
}

// metaNamespaceOption is the namespace for meta information (e.g., total number of buckets).
func metaNamespaceOption() protocol.StateOption {
	return protocol.NamespaceOption(state.StakingContractMetaNamespace)
}

func (r *ContractStakingStateReader) contract(contractAddr address.Address) (*StakingContract, error) {
	var contract StakingContract
	_, err := r.sr.State(
		&contract,
		metaNamespaceOption(),
		contractKeyOption(contractAddr),
	)
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// NumOfBuckets returns the total number of buckets for a contract.
func (r *ContractStakingStateReader) NumOfBuckets(contractAddr address.Address) (uint64, error) {
	contract, err := r.contract(contractAddr)
	if err != nil {
		return 0, err
	}
	return contract.NumOfBuckets, nil
}

// BucketType returns the BucketType for a given contract and bucket id.
func (r *ContractStakingStateReader) BucketType(contractAddr address.Address, tID uint64) (*BucketType, error) {
	var bktType stakingpb.BucketType
	if _, err := r.sr.State(
		&bktType,
		bucketTypeNamespaceOption(contractAddr),
		bucketIDKeyOption(tID),
	); err != nil {
		return nil, fmt.Errorf("failed to get bucket type %d for contract %s: %w", tID, contractAddr.String(), err)
	}
	return LoadBucketTypeFromProto(&bktType)
}

// Bucket returns the Bucket for a given contract and bucket id.
func (r *ContractStakingStateReader) Bucket(contractAddr address.Address, bucketID uint64) (*Bucket, error) {
	var ssb Bucket
	if _, err := r.sr.State(
		&ssb,
		contractNamespaceOption(contractAddr),
		bucketIDKeyOption(bucketID),
	); err != nil {
		switch errors.Cause(err) {
		case state.ErrStateNotExist:
			return nil, errors.Wrapf(ErrBucketNotExist, "bucket %d for contract %s", bucketID, contractAddr.String())
		}
		return nil, err
	}

	return &ssb, nil
}

// BucketTypes returns all BucketType for a given contract and bucket id.
func (r *ContractStakingStateReader) BucketTypes(contractAddr address.Address) ([]uint64, []*BucketType, error) {
	_, iter, err := r.sr.States(bucketTypeNamespaceOption(contractAddr), protocol.ObjectOption(&BucketType{}))
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("failed to get bucket types for contract %s: %w", contractAddr.String(), err)
	}
	ids := make([]uint64, 0, iter.Size())
	types := make([]*BucketType, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		var bktType stakingpb.BucketType
		switch key, err := iter.Next(&bktType); err {
		case nil:
			bt, err := LoadBucketTypeFromProto(&bktType)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to load bucket type from proto")
			}
			ids = append(ids, byteutil.BytesToUint64(key))
			types = append(types, bt)
		case state.ErrNilValue:
		default:
			return nil, nil, fmt.Errorf("failed to read bucket type %d for contract %s: %w", byteutil.BytesToUint64(key), contractAddr.String(), err)
		}
	}
	return ids, types, nil
}

// Buckets returns all BucketInfo for a given contract.
func (r *ContractStakingStateReader) Buckets(contractAddr address.Address) ([]uint64, []*Bucket, error) {
	_, iter, err := r.sr.States(contractNamespaceOption(contractAddr), protocol.ObjectOption(&Bucket{}))
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("failed to get buckets for contract %s: %w", contractAddr.String(), err)
	}
	ids := make([]uint64, 0, iter.Size())
	buckets := make([]*Bucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		var ssb stakingpb.SystemStakingBucket
		switch key, err := iter.Next(&ssb); err {
		case nil:
			bucket, err := LoadBucketFromProto(&ssb)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to load bucket from proto")
			}
			if bucket != nil {
				ids = append(ids, byteutil.BytesToUint64(key))
				buckets = append(buckets, bucket)
			}
		case state.ErrNilValue:
		default:
			return nil, nil, fmt.Errorf("failed to read bucket %d for contract %s: %w", byteutil.BytesToUint64(key), contractAddr.String(), err)
		}
	}
	return ids, buckets, nil
}
