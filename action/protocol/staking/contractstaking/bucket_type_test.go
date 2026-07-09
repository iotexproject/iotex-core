// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

func TestBucketType_SerializeDeserializeRoundTrip(t *testing.T) {
	r := require.New(t)
	bt := &BucketType{
		Amount:      big.NewInt(1_000_000),
		Duration:    720,
		ActivatedAt: 12345,
	}
	b, err := bt.Serialize()
	r.NoError(err)
	r.NotEmpty(b)

	got := &BucketType{}
	r.NoError(got.Deserialize(b))
	r.Equal(0, got.Amount.Cmp(bt.Amount))
	r.Equal(bt.Duration, got.Duration)
	r.Equal(bt.ActivatedAt, got.ActivatedAt)
}

func TestBucketType_DeserializeInvalidBytes(t *testing.T) {
	r := require.New(t)
	bt := &BucketType{}
	// not a valid protobuf wire encoding
	err := bt.Deserialize([]byte{0xff, 0xff, 0xff, 0xff})
	r.Error(err)
}

func TestBucketType_DeserializePreservesTargetOnError(t *testing.T) {
	r := require.New(t)
	// a well-formed proto whose amount cannot be parsed as a base-10 int must
	// surface a wrap error and not mutate the receiver in place.
	pb := &stakingpb.BucketType{Amount: "not-a-number", Duration: 1, ActivatedAt: 2}
	raw, err := proto.Marshal(pb)
	r.NoError(err)

	bt := &BucketType{Amount: big.NewInt(7), Duration: 99, ActivatedAt: 88}
	err = bt.Deserialize(raw)
	r.ErrorContains(err, "failed to load bucket type from proto")
	// receiver untouched on failure
	r.Equal(int64(7), bt.Amount.Int64())
	r.EqualValues(99, bt.Duration)
}

func TestLoadBucketTypeFromProto(t *testing.T) {
	r := require.New(t)
	bt, err := LoadBucketTypeFromProto(&stakingpb.BucketType{Amount: "42", Duration: 3, ActivatedAt: 4})
	r.NoError(err)
	r.Equal(int64(42), bt.Amount.Int64())
	r.EqualValues(3, bt.Duration)
	r.EqualValues(4, bt.ActivatedAt)

	_, err = LoadBucketTypeFromProto(&stakingpb.BucketType{Amount: "", Duration: 1})
	r.ErrorContains(err, "failed to parse amount")
}

func TestBucketType_Clone(t *testing.T) {
	r := require.New(t)
	bt := &BucketType{Amount: big.NewInt(100), Duration: 10, ActivatedAt: 20}
	clone := bt.Clone()
	r.Equal(0, clone.Amount.Cmp(bt.Amount))
	r.Equal(bt.Duration, clone.Duration)
	r.Equal(bt.ActivatedAt, clone.ActivatedAt)

	// mutating the clone's amount must not affect the original (deep copy)
	clone.Amount.Add(clone.Amount, big.NewInt(1))
	r.Equal(int64(100), bt.Amount.Int64())
	r.Equal(int64(101), clone.Amount.Int64())
}

func TestBucketType_EncodeDecodeRoundTrip(t *testing.T) {
	r := require.New(t)
	bt := &BucketType{Amount: big.NewInt(555), Duration: 30, ActivatedAt: 40}
	gv, err := bt.Encode()
	r.NoError(err)
	r.NotEmpty(gv.PrimaryData)

	got := &BucketType{}
	r.NoError(got.Decode(gv))
	r.Equal(int64(555), got.Amount.Int64())
	r.EqualValues(30, got.Duration)
	r.EqualValues(40, got.ActivatedAt)
}

func TestBucketType_DecodeInvalid(t *testing.T) {
	r := require.New(t)
	bt := &BucketType{}
	err := bt.Decode(systemcontracts.GenericValue{PrimaryData: []byte{0xff, 0xff}})
	r.Error(err)
}
