package contractstaking

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"go.uber.org/zap"

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
	return protocol.NamespaceOption(bucketNamespace(contractAddr))
}

// bucketNamespace is the state namespace holding one staking contract's
// buckets.
//
// Unexported on purpose: no code outside this package may address a
// contract-staking bucket, because doing so would drop the reader's global
// options. Use BucketStateOpts instead.
func bucketNamespace(contractAddr address.Address) string {
	return fmt.Sprintf("%s%x", state.ContractStakingBucketNamespacePrefix, contractAddr.Bytes())
}

// BucketStateOpts addresses one contract-staking bucket.
//
// This is the single expression for that address in the repository. Every live
// read and write of a bucket goes through it, and so does the IIP-59 era
// copy-on-write resolver's live-value fallback (staking.FrozenContractBucket),
// which would otherwise re-derive the same address by hand and drift from this
// one — silently, since a frozen read that misses is skipped, not failed.
func (r *ContractStakingStateReader) BucketStateOpts(contractAddr address.Address, bucketID uint64) []protocol.StateOption {
	return r.makeOpts(
		contractNamespaceOption(contractAddr),
		bucketIDKeyOption(bucketID),
	)
}

func bucketTypeNamespaceOption(contractAddr address.Address) protocol.StateOption {
	return protocol.NamespaceOption(fmt.Sprintf("%s%x", state.ContractStakingBucketTypeNamespacePrefix, contractAddr.Bytes()))
}

func contractKeyOption(contractAddr address.Address) protocol.StateOption {
	return protocol.KeyOption(contractAddr.Bytes())
}

// bucketIDKeyOption is the state key of one contract-staking bucket inside
// bucketNamespace. Little-endian; the encoding is fixed by existing state and
// must not be "corrected".
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
		r.BucketStateOpts(contractAddr, bucketID)...,
	); err != nil {
		switch errors.Cause(err) {
		case state.ErrStateNotExist:
			return nil, errors.Wrapf(ErrBucketNotExist, "bucket %d for contract %s", bucketID, contractAddr.String())
		}
		return nil, err
	}

	return &ssb, nil
}

// FrozenBucket reads a contract-staking bucket as of the era freeze height.
func (r *ContractStakingStateReader) FrozenBucket(
	window eracow.Window,
	contractAddr address.Address,
	bucketID uint64,
) (*Bucket, error) {
	if !window.Open() {
		return nil, errors.New("contractstaking: no era window open")
	}
	if !window.ContractBucketExisted(contractAddr.Bytes(), bucketID) {
		if !window.ContractKnown(contractAddr.Bytes()) {
			log.L().Error("IIP-59: contract-staking contract has no frozen bucket high-water mark; "+
				"all of its buckets are excluded from this era's voter weights",
				zap.String("contract", contractAddr.String()),
				zap.Uint64("bucketID", bucketID),
				zap.Uint64("freezeHeight", window.FreezeHeight),
			)
		}
		return nil, errors.Wrapf(eracow.ErrBucketPostFreeze, "contract bucket %d of %s", bucketID, contractAddr.String())
	}
	bucket := &Bucket{}
	err := eracow.Resolve(
		r.sr, window.FreezeHeight,
		eracow.KindLSDBucket, eracow.LSDBucketSubkey(contractAddr.Bytes(), bucketID),
		bucket,
		r.BucketStateOpts(contractAddr, bucketID)...,
	)
	switch {
	case err == nil:
		return bucket, nil
	case errors.Is(err, eracow.ErrNotFrozen):
		return nil, errors.Wrapf(eracow.ErrBucketPostFreeze, "contract bucket %d of %s", bucketID, contractAddr.String())
	default:
		return nil, err
	}
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

func (cs *ContractStakingStateReader) makeOpts(opts ...protocol.StateOption) []protocol.StateOption {
	return append(cs.globalOpts, opts...)
}
