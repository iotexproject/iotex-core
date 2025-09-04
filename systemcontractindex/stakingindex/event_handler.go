package stakingindex

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	eventHandler struct {
		stakingBucketNS string
		dirty           indexerCache       // dirty cache, a view for current block
		delta           batch.KVStoreBatch // delta for db to store buckets of current block
	}
)

func newEventHandler(bucketNS string, dirty indexerCache) *eventHandler {
	return &eventHandler{
		stakingBucketNS: bucketNS,
		dirty:           dirty,
		delta:           batch.NewBatch(),
	}
}

func (eh *eventHandler) PutBucketType(_ address.Address, bt *contractstaking.BucketType) error {
	return errors.New("not implemented")
}

func (eh *eventHandler) DeductBucket(_ address.Address, id uint64) (*Bucket, error) {
	bkt := eh.dirty.Bucket(id)
	if bkt == nil {
		return nil, errors.Wrapf(contractstaking.ErrBucketNotExist, "token id %d", id)
	}
	return bkt, nil
}

func (eh *eventHandler) Finalize() (batch.KVStoreBatch, indexerCache) {
	delta, dirty := eh.delta, eh.dirty
	eh.delta, eh.dirty = nil, nil
	return delta, dirty
}

func (eh *eventHandler) PutBucket(_ address.Address, id uint64, bkt *Bucket) error {
	data, err := bkt.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize bucket")
	}
	eh.dirty.PutBucket(id, bkt)
	eh.delta.Put(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket")
	return nil
}

func (eh *eventHandler) DeleteBucket(_ address.Address, id uint64) error {
	eh.dirty.DeleteBucket(id)
	eh.delta.Delete(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket")
	return nil
}
