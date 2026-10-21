package staking

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestBucketGuardKey(t *testing.T) {
	r := require.New(t)
	key := bucketGuardKey(0x0102030405060708)
	r.Equal(byte(6), key[0])
	r.Len(key, 9)
	r.Equal([]byte{1, 2, 3, 4, 5, 6, 7, 8}, key[1:])
}

func TestBucketGuardStateManager(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	bsm := NewBucketGuardStateManager(sm)
	bsr := NewBucketGuardStateReader(sm)

	got, err := bsr.Get(1)
	r.NoError(err)
	r.Nil(got)

	pub := bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize)
	g := &BucketGuard{
		AlgID:       bucketguard.AlgSLHDSA192s,
		PublicKey:   pub,
		Generation:  1,
		Nonce:       2,
		BoundHeight: 3,
	}
	r.NoError(bsm.Put(1, g))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.NotNil(got)
	r.True(got.Guarded)
	r.Equal(bucketguard.AlgSLHDSA192s, got.AlgID)
	r.Equal(pub, got.PublicKey)
	r.EqualValues(1, got.Generation)
	r.EqualValues(2, got.Nonce)
	r.EqualValues(3, got.BoundHeight)

	tombstone := &BucketGuard{Generation: 1, Nonce: 3}
	r.NoError(bsm.Put(1, tombstone))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.NotNil(got)
	r.False(got.Guarded)
	r.EqualValues(1, got.Generation)
	r.EqualValues(3, got.Nonce)
	r.EqualValues(0, got.BoundHeight)

	r.NoError(bsm.Delete(1))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.Nil(got)

	r.Error(bsm.Put(2, nil))
}
