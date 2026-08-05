package contractstaking

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"

	"github.com/iotexproject/iotex-address/address"
)

// ContractStakingStateReader wraps a state reader to provide staking contract-specific reads.
type ContractStakingStateReader struct {
	sr         protocol.StateReader
	globalOpts []protocol.StateOption
}

// NewStateReader creates a new ContractStakingStateReader.
func NewStateReader(sr protocol.StateReader, opts ...protocol.StateOption) *ContractStakingStateReader {
	return &ContractStakingStateReader{
		sr:         sr,
		globalOpts: opts,
	}
}

func contractNamespaceOption(contractAddr address.Address) protocol.StateOption {
	return protocol.NamespaceOption(BucketNamespace(contractAddr))
}

// BucketNamespace is the state namespace holding one staking contract's
// buckets. Exported so readers outside this package (the IIP-59 era
// copy-on-write resolver, which needs the live location of a covered key) can
// name it without duplicating the format string.
func BucketNamespace(contractAddr address.Address) string {
	return fmt.Sprintf("%s%x", state.ContractStakingBucketNamespacePrefix, contractAddr.Bytes())
}

// BucketKey is the state key of one contract-staking bucket inside
// BucketNamespace. Little-endian, matching bucketIDKeyOption; the encoding is
// fixed by existing state and must not be "corrected".
func BucketKey(bucketID uint64) []byte {
	return byteutil.Uint64ToBytes(bucketID)
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
		r.makeOpts(
			metaNamespaceOption(),
			contractKeyOption(contractAddr),
		)...,
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
	var bktType BucketType
	if _, err := r.sr.State(
		&bktType,
		r.makeOpts(
			bucketTypeNamespaceOption(contractAddr),
			bucketIDKeyOption(tID),
		)...,
	); err != nil {
		return nil, errors.Wrapf(err, "failed to get bucket type %d for contract %s", tID, contractAddr.String())
	}
	return &bktType, nil
}

// Bucket returns the Bucket for a given contract and bucket id.
func (r *ContractStakingStateReader) Bucket(contractAddr address.Address, bucketID uint64) (*Bucket, error) {
	var ssb Bucket
	if _, err := r.sr.State(
		&ssb,
		r.makeOpts(
			contractNamespaceOption(contractAddr),
			bucketIDKeyOption(bucketID),
		)...,
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
	_, iter, err := r.sr.States(r.makeOpts(
		bucketTypeNamespaceOption(contractAddr),
		protocol.ObjectOption(&BucketType{}),
	)...)
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil, nil, nil
	default:
		return nil, nil, errors.Wrapf(err, "failed to get bucket types for contract %s", contractAddr.String())
	}
	ids := make([]uint64, 0, iter.Size())
	types := make([]*BucketType, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		var bktType BucketType
		switch key, err := iter.Next(&bktType); err {
		case nil:
			ids = append(ids, byteutil.BytesToUint64(key))
			types = append(types, &bktType)
		case state.ErrNilValue:
		default:
			return nil, nil, errors.Wrapf(err, "failed to read bucket type %x for contract %s", key, contractAddr.String())
		}
	}
	return ids, types, nil
}

// Buckets returns all BucketInfo for a given contract.
func (r *ContractStakingStateReader) Buckets(contractAddr address.Address) ([]uint64, []*Bucket, error) {
	_, iter, err := r.sr.States(r.makeOpts(
		contractNamespaceOption(contractAddr),
		protocol.ObjectOption(&Bucket{}),
	)...)
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil, nil, nil
	default:
		return nil, nil, errors.Wrapf(err, "failed to get buckets for contract %s", contractAddr.String())
	}
	ids := make([]uint64, 0, iter.Size())
	buckets := make([]*Bucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		var ssb Bucket
		switch key, err := iter.Next(&ssb); err {
		case nil:
			ids = append(ids, byteutil.BytesToUint64(key))
			buckets = append(buckets, &ssb)
		case state.ErrNilValue:
		default:
			return nil, nil, errors.Wrapf(err, "failed to read bucket %d for contract %s", byteutil.BytesToUint64(key), contractAddr.String())
		}
	}
	return ids, buckets, nil
}

// rawKeyOnly is a Deserializer that throws the value away.
//
// States() insists on decoding every value it returns; MaxBucketIDInState only
// wants the keys, and decoding tens of thousands of buckets to read their ids
// off the keys they are already stored under is pure waste.
type rawKeyOnly struct{}

func (rawKeyOnly) Deserialize([]byte) error { return nil }

// MaxBucketIDInState returns the highest bucket id this contract currently has
// a record for, and whether it has any at all.
//
// This is the seed for the IIP-59 owner-index backfill and for the contract's
// bucket high-water mark, for contracts that have neither yet — i.e. every
// contract but V1, whose indexer wrote the mark all along. It is a full key
// scan of one contract's bucket namespace, so it is a seed-time operation, not
// a per-block one; the same CreatePreStates already enumerates every native
// bucket once per epoch (handleStakingIndexer), so a one-off scan at activation
// is within the established budget. The values are not decoded.
//
// "Highest id currently in state" can be below "highest id ever minted" when
// the top bucket has since been burned. That is still a sound freeze bound: it
// covers every bucket that exists, and any id above it can only belong to a
// bucket minted later, which is exactly what the bound is there to exclude.
func (r *ContractStakingStateReader) MaxBucketIDInState(contractAddr address.Address) (uint64, bool, error) {
	_, iter, err := r.sr.States(r.makeOpts(
		contractNamespaceOption(contractAddr),
		protocol.ObjectOption(rawKeyOnly{}),
	)...)
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return 0, false, nil
	default:
		return 0, false, errors.Wrapf(err, "failed to scan buckets of contract %s", contractAddr.String())
	}
	var (
		max   uint64
		found bool
	)
	for i := 0; i < iter.Size(); i++ {
		key, err := iter.Next(rawKeyOnly{})
		if err != nil && !errors.Is(err, state.ErrNilValue) {
			return 0, false, errors.Wrapf(err, "failed to scan buckets of contract %s", contractAddr.String())
		}
		if errors.Is(err, state.ErrNilValue) {
			continue
		}
		if len(key) != 8 {
			// Not a bucket key. The namespace is single-purpose, but a stray
			// key must not be read as a little-endian id.
			continue
		}
		// Little-endian, per BucketKey. Key order is therefore NOT id order,
		// which is why this has to look at every key rather than the last one.
		if id := byteutil.BytesToUint64(key); !found || id > max {
			max, found = id, true
		}
	}
	return max, found, nil
}

func (cs *ContractStakingStateReader) makeOpts(opts ...protocol.StateOption) []protocol.StateOption {
	return append(cs.globalOpts, opts...)
}
