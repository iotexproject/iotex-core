package actpool

import (
	"context"
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"path"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestBlobStore(t *testing.T) {
	r := require.New(t)
	cfg := blobStoreConfig{
		Datadir: t.TempDir(),
		Datacap: 10 * 1024 * 1024,
	}
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
	store, err := newBlobStore(cfg, encode, decode)
	r.NoError(err)
	r.NoError(store.Open(func(selp *action.SealedEnvelope) error {
		r.FailNow("should not be called")
		return nil
	}))
	act, err := action.SignedExecution("", identityset.PrivateKey(1), 1, big.NewInt(1), 100, big.NewInt(100), nil)
	r.NoError(err)
	r.NoError(store.Put(act))
	r.NoError(store.Close())

	store, err = newBlobStore(cfg, encode, decode)
	r.NoError(err)
	acts := []*action.SealedEnvelope{}
	r.NoError(store.Open(func(selp *action.SealedEnvelope) error {
		acts = append(acts, selp)
		return nil
	}))
	acts[0].Hash()
	r.Len(acts, 1)
	r.Equal(act, acts[0])
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
		cfg := blobStoreConfig{
			Datadir: b.TempDir(),
			Datacap: 1024,
		}
		store, err := newBlobStore(cfg, nil, nil)
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
