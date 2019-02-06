// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
)

func BenchmarkBoltDB_Get(b *testing.B) {
	runBenchmark := func(b *testing.B, size int) {
		path, err := ioutil.TempFile("", "boltdb")
		require.NoError(b, err)
		db := boltDB{
			path:   path.Name(),
			config: config.Default.DB,
		}
		db.Start(context.Background())
		defer db.Stop(context.Background())

		key := []byte("key")
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(rand.Int())
		}
		require.NoError(b, db.Put("ns", key, data))

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			b.StartTimer()
			_, err := db.Get("ns", key)
			b.StopTimer()
			require.NoError(b, err)
		}
	}

	b.Run("100", func(b *testing.B) {
		runBenchmark(b, 100)
	})
	b.Run("10000", func(b *testing.B) {
		runBenchmark(b, 100)
	})
	b.Run("1000000", func(b *testing.B) {
		runBenchmark(b, 100)
	})
}
