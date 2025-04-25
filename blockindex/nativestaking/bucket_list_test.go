// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package stakingindex

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
)

func TestBucketList(t *testing.T) {
	req := require.New(t)
	l0 := bucketList{0, nil}
	b := MustNoErrorV(l0.serialize())
	l1 := MustNoErrorV(deserializeBucketList(b))
	req.Equal(&l0, l1)
}

func TestBucketListSize(t *testing.T) {
	req := require.New(t)
	d, check := []uint64{}, []uint64{}
	for i := range 50001 {
		d = append(d, rand.Uint64N(50001))
		if i%5000 == 0 {
			check = append(check, d[i])
		}
	}
	l0 := bucketList{d[1234], d}
	b := MustNoErrorV(l0.serialize())
	l1 := MustNoErrorV(deserializeBucketList(b))
	req.Equal(&l0, l1)
	req.Equal(d[1234], l1.maxBucket)
	for i := 0; i < 50001; i += 5000 {
		req.Equal(check[i/5000], l1.deleted[i])
	}
}
