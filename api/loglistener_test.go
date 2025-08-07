package api

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/api/logfilter"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestGRPCLogListener_Respond(t *testing.T) {
	require := require.New(t)

	addr := "io13zt8szr73t3z44qg2gzfl6y5mprn76u2w4w234"
	filter := logfilter.NewLogFilter(&iotexapi.LogsFilter{
		Address: []string{addr},
	})
	stream := &testStream{}
	errChan := make(chan error)

	gl := NewGRPCLogListener(filter, stream.send, errChan).(*gRPCLogListener)
	require.NotNil(gl)

	t.Run("create block with logs", func(t *testing.T) {
		log := &action.Log{Address: addr}
		receipt := &action.Receipt{Status: 1}
		receipt.AddLogs(log)
		receipts := []*action.Receipt{receipt}
		blk, err := block.NewTestingBuilder().
			SetReceipts(receipts).
			SignAndBuild(identityset.PrivateKey(0))

		require.NoError(err)
		hash := blk.HashBlock()
		err = gl.Respond(hex.EncodeToString(hash[:]), &blk)
		require.NoError(err)
		require.Equal(1, stream.count)
	})
}

func TestGRPCLogListener(t *testing.T) {
	require := require.New(t)
	stream := &testStream{}
	errChan := make(chan error, 1)
	blk := &block.Block{}
	hash := blk.HashBlock()

	t.Run("exit without error", func(t *testing.T) {
		ctx := context.Background()
		filter := &logfilter.LogFilter{}

		gl := NewGRPCLogListener(filter, stream.send, errChan).(*gRPCLogListener)
		require.NotNil(gl)

		err := gl.Respond(hex.EncodeToString(hash[:]), blk)
		require.NoError(err)
		go func() {
			gl.Exit()
		}()
		err = <-errChan
		require.NoError(err)
		require.NotNil(ctx)
	})

	t.Run("stream error", func(t *testing.T) {
		addr := "io13zt8szr73t3z44qg2gzfl6y5mprn76u2w4w234"
		filter := logfilter.NewLogFilter(&iotexapi.LogsFilter{
			Address: []string{addr},
		})
		stream.err = errors.New("test error")
		gl := NewGRPCLogListener(filter, stream.send, errChan).(*gRPCLogListener)
		require.NotNil(gl)
		log := &action.Log{Address: addr}
		receipt := &action.Receipt{Status: 1}
		receipt.AddLogs(log)
		receipts := []*action.Receipt{receipt}

		_blk, err := block.NewTestingBuilder().
			SetReceipts(receipts).
			SignAndBuild(identityset.PrivateKey(0))
		require.NoError(err)

		blk = &_blk
		hash = blk.HashBlock()
		err = gl.Respond(hex.EncodeToString(hash[:]), blk)
		require.Error(err)
		require.Equal(stream.err, err)

		err = <-errChan
		require.Error(err)
		require.Equal(stream.err, err)
	})
}

func TestWeb3LogListener(t *testing.T) {
	require := require.New(t)
	stream := &testStream{}
	blk := &block.Block{}

	t.Run("exit without error", func(t *testing.T) {
		filter := &logfilter.LogFilter{}

		wl := NewWeb3LogListener(filter, stream.send).(*web3LogListener)
		require.NotNil(wl)

		err := wl.Respond("1", blk)
		require.NoError(err)
		wl.Exit()
	})

	t.Run("stream error", func(t *testing.T) {
		addr := "io13zt8szr73t3z44qg2gzfl6y5mprn76u2w4w234"
		filter := logfilter.NewLogFilter(&iotexapi.LogsFilter{
			Address: []string{addr},
		})
		stream.err = errors.New("test error")
		wl := NewWeb3LogListener(filter, stream.send).(*web3LogListener)

		require.NotNil(wl)

		log := &action.Log{Address: addr}
		receipt := &action.Receipt{Status: 1}
		receipt.AddLogs(log)
		receipts := []*action.Receipt{receipt}

		_blk, err := block.NewTestingBuilder().
			SetReceipts(receipts).
			SignAndBuild(identityset.PrivateKey(0))
		require.NoError(err)

		blk = &_blk
		err = wl.Respond("1", blk)
		require.Error(err)
		require.Equal(stream.err, err)
	})
}

type testStream struct {
	err   error
	count int
}

func (s *testStream) send(interface{}) (int, error) {
	s.count++
	return 0, s.err
}
