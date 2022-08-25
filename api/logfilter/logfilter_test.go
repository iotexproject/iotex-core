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
	_topic1 = hash.Hash256b([]byte("_topic1"))
	_topic2 = hash.Hash256b([]byte("_topic2"))
	_topicA = hash.Hash256b([]byte("_topicA"))
	_topicB = hash.Hash256b([]byte("_topicB"))
	_topicN = hash.Hash256b([]byte("topicNotExist"))

	_testFilter = []*iotexapi.LogsFilter{
		{
			Address: []string{},
			Topics:  []*iotexapi.Topics{},
		},
		{
			Address: []string{"_topic1", "_topic2", "_topicA", "_topicB"},
			Topics:  nil,
		},
		{
			Address: nil,
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						_topic1[:],
						_topic2[:],
					},
				},
				{
					Topic: [][]byte{
						_topicA[:],
						_topicB[:],
					},
				},
			},
		},
		{
			Address: []string{"_topic1", "_topic2"},
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						_topic1[:],
						_topic2[:],
					},
				},
				nil,
			},
		},
		{
			Address: []string{"_topicA", "_topicB"},
			Topics: []*iotexapi.Topics{
				nil,
				{
					Topic: [][]byte{
						_topicA[:],
						_topicB[:],
					},
				},
			},
		},
	}

	_testData = []struct {
		log    *action.Log
		match  [5]bool
		exist  [5]bool
		exist2 [5]bool
	}{
		{
			&action.Log{
				Address: "_topicN",
				Topics:  []hash.Hash256{_topicN}, // both address and topic not exist
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, false, false, false},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "_topicN",
				Topics:  []hash.Hash256{_topic1, _topic2, _topicA}, // topic longer than log's topic list
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "_topicN",
				Topics:  []hash.Hash256{_topic1, _topicN}, // topic not match
			},
			[5]bool{true, false, false, false, false},
			[5]bool{true, true, true, true, false},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "_topic1",
				Topics:  []hash.Hash256{_topicN}, // topic not exist
			},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "_topic2",
				Topics:  []hash.Hash256{_topicA, _topicB}, // topic not match
			},
			[5]bool{true, true, false, false, false},
			[5]bool{true, true, true, false, true},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "_topicN",
				Topics:  []hash.Hash256{_topic1, _topicB},
			},
			[5]bool{true, false, true, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "_topicN",
				Topics:  []hash.Hash256{_topic2, _topicA},
			},
			[5]bool{true, false, true, false, false},
			[5]bool{true, true, true, true, true},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "_topic1",
				Topics:  []hash.Hash256{_topic1, _topicN},
			},
			[5]bool{true, true, false, true, false},
			[5]bool{true, true, true, true, false},
			[5]bool{true, true, false, true, false}, // should be false
		},
		{
			&action.Log{
				Address: "_topicB",
				Topics:  []hash.Hash256{_topicN, _topicA},
			},
			[5]bool{true, true, false, false, true},
			[5]bool{true, true, true, false, true},
			[5]bool{true, true, false, false, true},
		},
	}
)

func TestLogFilter_MatchBlock(t *testing.T) {
	require := require.New(t)

	f := NewLogFilter(_testFilter[0])
	require.True(f.ExistInBloomFilter(nil))

	for i, q := range _testFilter {
		f = NewLogFilter(q)
		for _, v := range _testData {
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

	f := NewLogFilter(_testFilter[0])
	require.True(f.ExistInBloomFilterv2(nil))

	for i, q := range _testFilter {
		f = NewLogFilter(q)
		for _, v := range _testData {
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
