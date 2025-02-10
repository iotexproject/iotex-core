package actpool

import (
	"context"
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestBlobStore(t *testing.T) {
	r := require.New(t)
	encode := func(selp *action.SealedEnvelope) ([]byte, error) {
		return proto.Marshal(selp.Proto())
	}
	decode := func(blob []byte) (*action.SealedEnvelope, error) {
		d := &action.Deserializer{}
		d.SetEvmNetworkID(4689)

		a := &iotextypes.Action{}
		if err := proto.Unmarshal(blob, a); err != nil {
			return nil, err
		}
		se, err := d.ActionToSealedEnvelope(a)
		if err != nil {
			return nil, err
		}
		return se, nil
	}
	t.Run("open", func(t *testing.T) {
		cfg := actionStoreConfig{
			Datadir: t.TempDir(),
		}
		store, err := newActionStore(cfg, encode, decode)
		r.NoError(err)
		r.NoError(store.Open(func(selp *action.SealedEnvelope) error {
			r.FailNow("should not be called")
			return nil
		}))
		act, err := action.SignedExecution("", identityset.PrivateKey(1), 1, big.NewInt(1), 100, big.NewInt(100), nil)
		r.NoError(err)
		r.NoError(store.Put(act))
		r.NoError(store.Close())

		store, err = newActionStore(cfg, encode, decode)
		r.NoError(err)
		acts := []*action.SealedEnvelope{}
		r.NoError(store.Open(func(selp *action.SealedEnvelope) error {
			acts = append(acts, selp)
			return nil
		}))
		r.Len(acts, 1)
		r.Equal(act, acts[0])
	})
	t.Run("put", func(t *testing.T) {
		cfg := actionStoreConfig{
			Datadir: t.TempDir(),
		}
		store, err := newActionStore(cfg, encode, decode)
		r.NoError(err)
		r.NoError(store.Open(func(selp *action.SealedEnvelope) error { return nil }))
		// put action 1
		body := make([]byte, 1024)
		act, err := action.SignedExecution("", identityset.PrivateKey(1), 1, big.NewInt(1), 100, big.NewInt(100), body)
		r.NoError(err)
		r.NoError(store.Put(act))
		r.Equal(uint64(4096), store.stored)
		r.Equal(1, len(store.lookup))
		assertions.MustNoErrorV(store.Get(assertions.MustNoErrorV(act.Hash())))
		// put action 2
		act, err = action.SignedExecution("", identityset.PrivateKey(1), 2, big.NewInt(1), 100, big.NewInt(100), body)
		r.NoError(err)
		r.NoError(store.Put(act))
		r.Equal(uint64(8192), store.stored)
		r.Equal(2, len(store.lookup))
		assertions.MustNoErrorV(store.Get(assertions.MustNoErrorV(act.Hash())))
		// put action 3 and drop
		act, err = action.SignedExecution("", identityset.PrivateKey(1), 3, big.NewInt(1), 100, big.NewInt(100), body)
		r.NoError(err)
		r.NoError(store.Put(act))
		r.Equal(uint64(12288), store.stored)
		r.Equal(3, len(store.lookup))
		assertions.MustNoErrorV(store.Get(assertions.MustNoErrorV(act.Hash())))
	})
}

func BenchmarkDatabase(b *testing.B) {
	r := require.New(b)
	blobs := make([][]byte, 100)
	for i := range blobs {
		blob := make([]byte, 4096*32)
		_, err := rand.Read(blob)
		r.NoError(err)
		blobs[i] = blob
	}
	keys := make([]hash.Hash256, 100)
	for i := range keys {
		key := make([]byte, 32)
		_, err := rand.Read(key)
		r.NoError(err)
		copy(keys[i][:], key)
	}

	b.Run("billy-put", func(b *testing.B) {
		cfg := actionStoreConfig{
			Datadir: b.TempDir(),
		}
		store, err := newActionStore(cfg, nil, nil)
		r.NoError(err)
		r.NoError(store.Open(func(selp *action.SealedEnvelope) error {
			r.FailNow("should not be called")
			return nil
		}))

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.store.Put(blobs[mrand.Intn(len(blobs))])
			r.NoError(err)
		}
		b.StopTimer()
		r.NoError(store.store.Close())
	})
	b.Run("pebbledb-put", func(b *testing.B) {
		cfg := db.DefaultConfig
		cfg.DbPath = path.Join(b.TempDir(), "pebbledb")
		store := db.NewPebbleDB(cfg)
		r.NoError(store.Start(context.Background()))
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			idx := mrand.Intn(len(blobs))
			err := store.Put("ns", keys[idx][:], blobs[idx])
			r.NoError(err)
		}
		b.StopTimer()
		r.NoError(store.Stop(context.Background()))
	})
	b.Run("boltdb-put", func(b *testing.B) {
		cfg := db.DefaultConfig
		cfg.DbPath = path.Join(b.TempDir(), "boltdb")
		store := db.NewBoltDB(cfg)
		r.NoError(store.Start(context.Background()))
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			idx := mrand.Intn(len(blobs))
			err := store.Put("ns", keys[idx][:], blobs[idx])
			r.NoError(err)
		}
		b.StopTimer()
		r.NoError(store.Stop(context.Background()))
	})
}
