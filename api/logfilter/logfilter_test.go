package logfilter

import (
	"testing"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	topic1 = hash.Hash256b([]byte("topic1"))
	topic2 = hash.Hash256b([]byte("topic2"))
	topicA = hash.Hash256b([]byte("topicA"))
	topicB = hash.Hash256b([]byte("topicB"))
	topicN = hash.Hash256b([]byte("topicNotExist"))

	testFilter = []*iotexapi.LogsFilter{
		{
			Address: []string{},
			Topics:  []*iotexapi.Topics{},
		},
		{
			Address: []string{"topic1", "topic2", "topicA", "topicB"},
			Topics:  nil,
		},
		{
			Address: nil,
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						topic1[:],
						topic2[:],
					},
				},
				{
					Topic: [][]byte{
						topicA[:],
						topicB[:],
					},
				},
			},
		},
		{
			Address: []string{"topic1", "topic2"},
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						topic1[:],
						topic2[:],
					},
				},
				nil,
			},
		},
		{
			Address: []string{"topicA", "topicB"},
			Topics: []*iotexapi.Topics{
				nil,
				{
					Topic: [][]byte{
						topicA[:],
						topicB[:],
					},
				},
			},
		},
	}

	testData = []struct {
		log    *action.Log
		match  [5]bool
		exist  [5]bool
		exist2 [5]bool
	}{
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{topicN}, // both address and topic not exist
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, false, false, false},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{topic1, topic2, topicA}, // topic longer than log's topic list
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{topic1, topicN}, // topic not match
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, true, true, false},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topic1",
				Topics:  []hash.Hash256{topicN}, // topic not exist
			},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "topic2",
				Topics:  []hash.Hash256{topicA, topicB}, // topic not match
			},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, true, false, true},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{topic1, topicB},
			},
			[5]bool{true, false, true, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{topic2, topicA},
			},
			[5]bool{true, false, true, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "topic1",
				Topics:  []hash.Hash256{topic1, topicN},
			},
			[5]bool{true, true, false, true, false},
			[5]bool{true, true, true, true, false},
			[5]bool{true, true, false, true, false}, // should be false
		},
		{
			&action.Log{
				Address: "topicB",
				Topics:  []hash.Hash256{topicN, topicA},
			},
			[5]bool{true, true, false, false, true},
			[5]bool{true, true, true, false, true},
			[5]bool{true, true, false, false, true},
		},
	}
)

func TestLogFilter_MatchBlock(t *testing.T) {
	require := require.New(t)

	f := NewLogFilter(testFilter[0], nil, nil)
	require.True(f.ExistInBloomFilter(nil))

	for i, q := range testFilter {
		f = NewLogFilter(q, nil, nil)
		for _, v := range testData {
			bloom, err := bloom.NewBloomFilter(2048, 3)
			require.NoError(err)
			for _, topic := range v.log.Topics {
				bloom.Add(topic[:])
			}
			require.Equal(f.ExistInBloomFilter(bloom), v.exist[i])
			log := v.log.ConvertToLogPb()
			require.Equal(f.match(log), v.match[i])
		}
	}
}

func TestLogFilter_ExistInBloomFilterv2(t *testing.T) {
	require := require.New(t)

	f := NewLogFilter(testFilter[0], nil, nil)
	require.True(f.ExistInBloomFilterv2(nil))

	for i, q := range testFilter {
		f = NewLogFilter(q, nil, nil)
		for _, v := range testData {
			bloom, err := bloom.NewBloomFilter(2048, 3)
			require.NoError(err)
			bloom.Add([]byte(v.log.Address))
			for i, topic := range v.log.Topics {
				bloom.Add(append(byteutil.Uint64ToBytes(uint64(i)), topic[:]...))
			}
			require.Equal(v.exist2[i], f.ExistInBloomFilterv2(bloom))
			log := v.log.ConvertToLogPb()
			require.Equal(f.match(log), v.match[i])
		}
	}
}
