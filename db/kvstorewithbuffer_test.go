package db

import (
	"bytes"
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/test/mock/mock_batch"
)

func TestFlusher(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Run("create failed with nil kvStore", func(t *testing.T) {
		f, err := NewKVStoreFlusher(nil, nil)
		require.Nil(t, f)
		require.Error(t, err)
		require.Contains(t, err.Error(), "store cannot be nil")
	})
	t.Run("fail to create with nil buffer", func(t *testing.T) {
		store := NewMockKVStore(ctrl)
		f, err := NewKVStoreFlusher(store, nil)
		require.Nil(t, f)
		require.Error(t, err)
		require.Contains(t, err.Error(), "buffer cannot be nil")
	})
	t.Run("create flusher successfully", func(t *testing.T) {
		store := NewMockKVStore(ctrl)
		buffer := mock_batch.NewMockCachedBatch(ctrl)
		f, err := NewKVStoreFlusher(store, buffer)
		require.NoError(t, err)
		kvb := f.KVStoreWithBuffer()
		expectedError := errors.New("failed to start")
		ns := "namespace"
		key := []byte("key")
		value := []byte("value")
		t.Run("fail to start kvStore with buffer", func(t *testing.T) {
			store.EXPECT().Start(gomock.Any()).Return(err).Times(1)
			require.Equal(t, err, kvb.Start(context.Background()))
		})
		t.Run("fail to stop kvStore with buffer", func(t *testing.T) {
			err = errors.New("failed to stop")
			store.EXPECT().Stop(gomock.Any()).Return(err).Times(1)
			require.Equal(t, err, kvb.Stop(context.Background()))
		})
		t.Run("start kv store successfully", func(t *testing.T) {
			store.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
			store.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
			require.NoError(t, kvb.Start(context.Background()))
			require.NoError(t, kvb.Stop(context.Background()))
		})
		t.Run("fail to flush", func(t *testing.T) {
			buffer.EXPECT().Translate(gomock.Any()).Return(buffer).Times(1)
			store.EXPECT().WriteBatch(gomock.Any()).Return(expectedError).Times(1)
			require.Equal(t, expectedError, f.Flush())
		})
		t.Run("flush successfully", func(t *testing.T) {
			buffer.EXPECT().Translate(gomock.Any()).Return(buffer).Times(1)
			store.EXPECT().WriteBatch(gomock.Any()).Return(nil).Times(1)
			buffer.EXPECT().Lock().Times(1)
			buffer.EXPECT().ClearAndUnlock().Times(1)
			require.NoError(t, f.Flush())
		})
		t.Run("Get", func(t *testing.T) {
			buffer.EXPECT().Get(ns, key).Return(value, nil).Times(1)
			v, err := kvb.Get(ns, key)
			require.True(t, bytes.Equal(value, v))
			require.NoError(t, err)
			buffer.EXPECT().Get(ns, key).Return(nil, batch.ErrNotExist).Times(1)
			store.EXPECT().Get(ns, key).Return(value, nil)
			v, err = kvb.Get(ns, key)
			require.True(t, bytes.Equal(value, v))
			require.NoError(t, err)
			buffer.EXPECT().Get(ns, key).Return(nil, batch.ErrAlreadyDeleted).Times(1)
			v, err = kvb.Get(ns, key)
			require.Nil(t, v)
			require.Equal(t, errors.Cause(err), ErrNotExist)
		})
		t.Run("Snapshot", func(t *testing.T) {
			buffer.EXPECT().Snapshot().Return(1).Times(1)
			require.Equal(t, 1, kvb.Snapshot())
		})
		t.Run("Revert", func(t *testing.T) {
			buffer.EXPECT().RevertSnapshot(gomock.Any()).Return(expectedError).Times(1)
			require.Equal(t, expectedError, kvb.RevertSnapshot(1))
			buffer.EXPECT().RevertSnapshot(gomock.Any()).Return(nil).Times(1)
			require.NoError(t, kvb.RevertSnapshot(1))
		})
		t.Run("Size", func(t *testing.T) {
			buffer.EXPECT().Size().Return(5).Times(1)
			require.Equal(t, 5, kvb.Size())
		})
		t.Run("SerializeQueue", func(t *testing.T) {
			buffer.EXPECT().SerializeQueue(gomock.Any(), gomock.Any()).Return(value).Times(1)
			require.Equal(t, value, f.SerializeQueue())
		})
		t.Run("MustPut", func(t *testing.T) {
			buffer.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			kvb.MustPut(ns, key, value)
		})
		t.Run("MustDelete", func(t *testing.T) {
			buffer.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			kvb.MustDelete(ns, key)
		})
	})
}
