// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/api/logfilter"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// buildBlockWithLogs builds a block whose single receipt carries the given logs.
func buildBlockWithLogs(t *testing.T, logs ...*action.Log) *block.Block {
	receipt := &action.Receipt{BlockHeight: 1}
	receipt.AddLogs(logs...)
	builder := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetTimeStamp(time.Now()).
		SetReceipts([]*action.Receipt{receipt})
	blk, err := builder.SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}

// matchAllFilter returns a filter with empty address+topics, which matches every log.
func matchAllFilter() *logfilter.LogFilter {
	return logfilter.NewLogFilter(&iotexapi.LogsFilter{Address: []string{}, Topics: []*iotexapi.Topics{}})
}

// nonMatchingFilter restricts to an address that is not present in the test logs.
func nonMatchingFilter() *logfilter.LogFilter {
	return logfilter.NewLogFilter(&iotexapi.LogsFilter{Address: []string{"io1nomatchaddressxxxxxxxxxxxxxxxxxxxxxxxx"}})
}

func TestGRPCLogListener(t *testing.T) {
	r := require.New(t)
	logObj := &action.Log{
		Address:     identityset.Address(1).String(),
		Topics:      action.Topics{hash.Hash256b([]byte("topic"))},
		BlockHeight: 1,
	}
	blk := buildBlockWithLogs(t, logObj)

	t.Run("match and stream", func(t *testing.T) {
		errChan := make(chan error, 10)
		var got []*iotexapi.StreamLogsResponse
		handler := func(in interface{}) (int, error) {
			got = append(got, in.(*iotexapi.StreamLogsResponse))
			return 0, nil
		}
		ll := NewGRPCLogListener(matchAllFilter(), handler, errChan)
		r.NoError(ll.Respond("", blk))
		r.Len(got, 1)
		// the block hash is injected into the streamed log
		blkHash := blk.HashBlock()
		r.Equal(blkHash[:], got[0].Log.BlkHash)
	})

	t.Run("no match short-circuits without streaming", func(t *testing.T) {
		errChan := make(chan error, 10)
		called := false
		handler := func(in interface{}) (int, error) {
			called = true
			return 0, nil
		}
		ll := NewGRPCLogListener(nonMatchingFilter(), handler, errChan)
		r.NoError(ll.Respond("", blk))
		r.False(called)
	})

	t.Run("stream error is propagated to errChan", func(t *testing.T) {
		errChan := make(chan error, 10)
		sendErr := errorSend
		handler := func(in interface{}) (int, error) {
			return 0, sendErr
		}
		ll := NewGRPCLogListener(matchAllFilter(), handler, errChan)
		r.Equal(sendErr, ll.Respond("", blk))
		r.Equal(sendErr, <-errChan)
	})

	t.Run("exit sends nil to errChan", func(t *testing.T) {
		errChan := make(chan error, 10)
		ll := NewGRPCLogListener(matchAllFilter(), nil, errChan)
		ll.Exit()
		r.NoError(<-errChan)
	})
}

func TestWeb3LogListener(t *testing.T) {
	r := require.New(t)
	logObj := &action.Log{
		Address:     identityset.Address(1).String(),
		Topics:      action.Topics{hash.Hash256b([]byte("topic"))},
		BlockHeight: 1,
	}
	blk := buildBlockWithLogs(t, logObj)

	t.Run("match and stream", func(t *testing.T) {
		var got []interface{}
		handler := func(in interface{}) (int, error) {
			got = append(got, in)
			return 0, nil
		}
		ll := NewWeb3LogListener(matchAllFilter(), handler)
		r.NoError(ll.Respond("id", blk))
		r.Len(got, 1)
		resp, ok := got[0].(*streamResponse)
		r.True(ok)
		r.Equal("id", resp.id)
	})

	t.Run("no match short-circuits", func(t *testing.T) {
		called := false
		handler := func(in interface{}) (int, error) {
			called = true
			return 0, nil
		}
		ll := NewWeb3LogListener(nonMatchingFilter(), handler)
		r.NoError(ll.Respond("id", blk))
		r.False(called)
	})

	t.Run("stream error is returned", func(t *testing.T) {
		handler := func(in interface{}) (int, error) {
			return 0, errorSend
		}
		ll := NewWeb3LogListener(matchAllFilter(), handler)
		r.Equal(errorSend, ll.Respond("id", blk))
	})

	t.Run("exit is a no-op", func(t *testing.T) {
		ll := NewWeb3LogListener(matchAllFilter(), nil)
		r.NotPanics(func() { ll.Exit() })
	})
}
