// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package batch

import (
	"bytes"
	"math/rand"
	"strconv"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var (
	_bucket1 = "test_ns1"
	_testK1  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	_testV1  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
	_testK2  = [3][]byte{[]byte("key_4"), []byte("key_5"), []byte("key_6")}
	_testV2  = [3][]byte{[]byte("value_4"), []byte("value_5"), []byte("value_6")}
)

func TestBaseKVStoreBatch(t *testing.T) {
	require := require.New(t)

	b := NewBatch()
	require.Equal(0, b.Size())
	b.Put("ns", []byte{}, []byte{}, "")
	require.Equal(1, b.Size())
	_, err := b.Entry(1)
	require.Error(err)
	b.Delete("ns", []byte{}, "")
	require.Equal(2, b.Size())
	wi, err := b.Entry(1)
	require.NoError(err)
	require.Equal(Delete, wi.WriteType())
	b.AddFillPercent("test", 0.5)
	p, ok := b.CheckFillPercent("ns")
	require.False(ok)
	require.Equal(1.0*0, p)
	p, ok = b.CheckFillPercent("test")
	require.True(ok)
	require.Equal(0.5, p)

	// test serialize/translate
	require.True(bytes.Equal([]byte{0, 110, 115, 1, 110, 115}, b.SerializeQueue(nil, nil)))
	require.True(bytes.Equal([]byte{}, b.SerializeQueue(nil, func(wi *WriteInfo) bool {
		return wi.Namespace() == "ns"
	})))
	require.True(bytes.Equal([]byte{110, 115, 110, 115}, b.SerializeQueue(func(wi *WriteInfo) []byte {
		return wi.SerializeWithoutWriteType()
	}, nil)))
	newb := b.Translate(func(wi *WriteInfo) *WriteInfo {
		if wi.WriteType() == Delete {
			return NewWriteInfo(
				Put,
				"to_delete_ns",
				wi.Key(),
				wi.Value(),
				"",
			)
		}
		return wi
	})
	newEntry1, err := newb.Entry(1)
	require.NoError(err)
	require.Equal("to_delete_ns", newEntry1.Namespace())
	require.Equal(Put, newEntry1.WriteType())
	b.Clear()
	require.Equal(0, b.Size())
}

func TestCachedBatch(t *testing.T) {
	require := require.New(t)

	cb := NewCachedBatch()
	cb.Put(_bucket1, _testK1[0], _testV1[0], "")
	v, err := cb.Get(_bucket1, _testK1[0])
	require.NoError(err)
	require.Equal(_testV1[0], v)
	v, err = cb.Get(_bucket1, _testK2[0])
	require.Equal(ErrNotExist, err)
	require.Equal([]byte(nil), v)
	si := cb.Snapshot()
	require.Equal(0, si)

	cb.Delete(_bucket1, _testK2[0], "")
	cb.Delete(_bucket1, _testK1[0], "")
	_, err = cb.Get(_bucket1, _testK1[0])
	require.Equal(ErrAlreadyDeleted, errors.Cause(err))

	w, err := cb.Entry(1)
	require.NoError(err)
	require.Equal(_bucket1, w.namespace)
	require.Equal(_testK2[0], w.key)
	require.Equal([]byte(nil), w.value)
	require.Equal(Delete, w.writeType)

	w, err = cb.Entry(2)
	require.NoError(err)
	require.Equal(_bucket1, w.namespace)
	require.Equal(_testK1[0], w.key)
	require.Equal([]byte(nil), w.value)
	require.Equal(Delete, w.writeType)
	require.True(bytes.Equal(
		[]byte{116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49, 118, 97, 108, 117, 101, 95, 49, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 52, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49},
		cb.SerializeQueue(func(wi *WriteInfo) []byte {
			return wi.SerializeWithoutWriteType()
		}, nil),
	))
	require.True(bytes.Equal([]byte{116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49, 118, 97, 108, 117, 101, 95, 49}, cb.SerializeQueue(func(wi *WriteInfo) []byte {
		return wi.SerializeWithoutWriteType()
	}, func(wi *WriteInfo) bool {
		return wi.WriteType() == Delete
	})))
	require.True(bytes.Equal(
		[]byte{0, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49, 118, 97, 108, 117, 101, 95, 49, 1, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 52, 1, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49},
		cb.SerializeQueue(nil, nil),
	))
	require.True(bytes.Equal([]byte{0, 116, 101, 115, 116, 95, 110, 115, 49, 107, 101, 121, 95, 49, 118, 97, 108, 117, 101, 95, 49}, cb.SerializeQueue(nil, func(wi *WriteInfo) bool {
		return wi.WriteType() == Delete
	})))
	require.Equal(3, cb.Size())
	require.Error(cb.RevertSnapshot(-1))
	require.Error(cb.RevertSnapshot(si + 1))
	require.NoError(cb.RevertSnapshot(si))
	require.Equal(1, cb.Size())
	require.True(bytes.Equal([]byte{}, cb.Translate(func(wi *WriteInfo) *WriteInfo {
		if wi.WriteType() != Delete {
			return nil
		}
		return wi
	}).SerializeQueue(nil, nil)))
	cb.Clear()
	require.Equal(0, cb.Size())
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)

	cb := NewCachedBatch()
	cb.Clear()
	cb.Put(_bucket1, _testK1[0], _testV1[0], "")
	cb.Put(_bucket1, _testK1[1], _testV1[1], "")
	s0 := cb.Snapshot()
	require.Equal(0, s0)
	require.Equal(2, cb.Size())

	cb.Put(_bucket1, _testK2[0], _testV2[0], "")
	cb.Put(_bucket1, _testK2[1], _testV2[1], "")
	cb.Delete(_bucket1, _testK1[0], "")
	v, err := cb.Get(_bucket1, _testK1[0])
	require.Equal(ErrAlreadyDeleted, err)
	require.Nil(v)
	s1 := cb.Snapshot()
	require.Equal(1, s1)
	require.Equal(5, cb.Size())

	cb.Put(_bucket1, _testK1[2], _testV1[2], "")
	cb.Put(_bucket1, _testK2[2], _testV2[2], "")
	cb.Delete(_bucket1, _testK2[0], "")
	_, err = cb.Get(_bucket1, _testK2[0])
	require.Equal(ErrAlreadyDeleted, err)
	s2 := cb.Snapshot()
	require.Equal(2, s2)
	require.Equal(8, cb.Size())

	// snapshot 2
	require.Error(cb.RevertSnapshot(3))
	require.Error(cb.RevertSnapshot(-1))
	require.NoError(cb.RevertSnapshot(2))
	_, err = cb.Get(_bucket1, _testK2[0])
	require.Equal(ErrAlreadyDeleted, err)
	v, err = cb.Get(_bucket1, _testK1[1])
	require.NoError(err)
	require.Equal(_testV1[1], v)
	v, err = cb.Get(_bucket1, _testK2[1])
	require.NoError(err)
	require.Equal(_testV2[1], v)
	v, err = cb.Get(_bucket1, _testK1[2])
	require.NoError(err)
	require.Equal(_testV1[2], v)
	v, err = cb.Get(_bucket1, _testK2[2])
	require.NoError(err)
	require.Equal(_testV2[2], v)
	cb.Put(_bucket1, _testK2[2], _testV2[1], "")
	v, err = cb.Get(_bucket1, _testK2[2])
	require.NoError(err)
	require.Equal(_testV2[1], v)

	// snapshot 1
	require.NoError(cb.RevertSnapshot(2))
	v, err = cb.Get(_bucket1, _testK2[2])
	require.NoError(err)
	require.Equal(_testV2[2], v)
	require.NoError(cb.RevertSnapshot(1))
	_, err = cb.Get(_bucket1, _testK1[0])
	require.Equal(ErrAlreadyDeleted, err)
	v, err = cb.Get(_bucket1, _testK1[1])
	require.NoError(err)
	require.Equal(_testV1[1], v)
	v, err = cb.Get(_bucket1, _testK2[0])
	require.NoError(err)
	require.Equal(_testV2[0], v)
	v, err = cb.Get(_bucket1, _testK2[1])
	require.NoError(err)
	require.Equal(_testV2[1], v)
	_, err = cb.Get(_bucket1, _testK2[2])
	require.Equal(ErrNotExist, err)

	// snapshot 0
	require.Error(cb.RevertSnapshot(2))
	require.NoError(cb.RevertSnapshot(0))
	v, err = cb.Get(_bucket1, _testK1[0])
	require.NoError(err)
	require.Equal(_testV1[0], v)
	v, err = cb.Get(_bucket1, _testK1[1])
	require.NoError(err)
	require.Equal(_testV1[1], v)
	_, err = cb.Get(_bucket1, _testK2[0])
	require.Equal(ErrNotExist, err)
	_, err = cb.Get(_bucket1, _testK1[2])
	require.Equal(ErrNotExist, err)
}

func BenchmarkCachedBatch_Digest(b *testing.B) {
	cb := NewCachedBatch()

	for i := 0; i < 10000; i++ {
		k := hash.Hash256b([]byte(strconv.Itoa(i)))
		var v [1024]byte
		for i := range v {
			v[i] = byte(rand.Intn(8))
		}
		cb.Put(_bucket1, k[:], v[:], "")
	}
	require.Equal(b, 10000, cb.Size())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		b.StartTimer()
		h := cb.SerializeQueue(nil, nil)
		b.StopTimer()
		require.NotEqual(b, hash.ZeroHash256, h)
	}
}

func BenchmarkCachedBatch_Snapshot(b *testing.B) {
	cb := NewCachedBatch()
	k := hash.Hash256b([]byte("test"))
	var v [1024]byte
	for i := range v {
		v[i] = byte(rand.Intn(8))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Put(_bucket1, k[:], v[:], "")
		_, _ = cb.Get(_bucket1, k[:])
		sn := cb.Snapshot()
		cb.Delete(_bucket1, k[:], "")
		_, _ = cb.Get(_bucket1, k[:])
		cb.RevertSnapshot(sn)
		_, _ = cb.Get(_bucket1, k[:])
		cb.Delete(_bucket1, k[:], "")
	}
}
