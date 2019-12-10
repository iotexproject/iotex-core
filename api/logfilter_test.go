package api

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
)

var (
	topic1 = hash.Hash256b([]byte("topic1"))
	topic2 = hash.Hash256b([]byte("topic2"))
	topicA = hash.Hash256b([]byte("topicA"))
	topicB = hash.Hash256b([]byte("topicB"))

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

	data1 = hash.Hash256b([]byte("topic1"))
	data2 = hash.Hash256b([]byte("topic2"))
	dataA = hash.Hash256b([]byte("topicA"))
	dataB = hash.Hash256b([]byte("topicB"))
	dataN = hash.Hash256b([]byte("topicNotExist"))

	testData = []struct {
		log   *action.Log
		match [5]bool
	}{
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{dataN}, // both address and topic not exist
			},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{data1, data2, dataA}, // topic longer than log's topic list
			},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{data1, dataN}, // topic not match
			},
			[5]bool{true, false, false, false, false},
		},
		{
			&action.Log{
				Address: "topic1",
				Topics:  []hash.Hash256{dataN}, // topic not exist
			},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "topic2",
				Topics:  []hash.Hash256{dataA, dataB}, // topic not match
			},
			[5]bool{true, true, false, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{data1, dataB},
			},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "topicN",
				Topics:  []hash.Hash256{data2, dataA},
			},
			[5]bool{true, false, true, false, false},
		},
		{
			&action.Log{
				Address: "topic1",
				Topics:  []hash.Hash256{data1, dataN},
			},
			[5]bool{true, true, false, true, false},
		},
		{
			&action.Log{
				Address: "topicB",
				Topics:  []hash.Hash256{dataN, dataA},
			},
			[5]bool{true, true, false, false, true},
		},
	}
)

func TestLogFilter_MatchBlock(t *testing.T) {
	require := require.New(t)

	for i, q := range testFilter {
		f, ok := NewLogFilter(q, nil, nil).(*LogFilter)
		require.True(ok)
		for _, v := range testData {
			log := v.log.ConvertToLogPb()
			require.Equal(f.match(log), v.match[i])
		}
	}
}
