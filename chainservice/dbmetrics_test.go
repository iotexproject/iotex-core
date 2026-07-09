// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"os"
	"path/filepath"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/config"
)

func TestDBFileSizeBytes_File(t *testing.T) {
	r := require.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.db")
	payload := []byte("hello-iotex")
	r.NoError(os.WriteFile(path, payload, 0o600))

	size, err := dbFileSizeBytes(path)
	r.NoError(err)
	r.Equal(int64(len(payload)), size)
}

func TestDBFileSizeBytes_Directory(t *testing.T) {
	r := require.New(t)
	dir := t.TempDir()
	// pebbledb-style directory: sum of all contained files, recursively.
	r.NoError(os.WriteFile(filepath.Join(dir, "a"), []byte("12345"), 0o600))
	sub := filepath.Join(dir, "sub")
	r.NoError(os.Mkdir(sub, 0o700))
	r.NoError(os.WriteFile(filepath.Join(sub, "b"), []byte("6789"), 0o600))

	size, err := dbFileSizeBytes(dir)
	r.NoError(err)
	r.Equal(int64(9), size)
}

func TestDBFileSizeBytes_Missing(t *testing.T) {
	r := require.New(t)
	_, err := dbFileSizeBytes(filepath.Join(t.TempDir(), "does-not-exist.db"))
	r.Error(err)
}

func TestConfiguredDBFiles_SkipsEmpty(t *testing.T) {
	r := require.New(t)
	cfg := config.Config{}
	cfg.Chain.ChainDBPath = "/data/chain.db"
	cfg.Chain.TrieDBPath = "/data/trie.db"
	// leave the rest empty -> they must not produce entries

	files := configuredDBFiles(cfg)
	r.Len(files, 2)
	names := map[string]string{}
	for _, f := range files {
		names[f.name] = f.path
	}
	r.Equal("/data/chain.db", names["chain"])
	r.Equal("/data/trie.db", names["trie"])
}

func TestLocalChainDBPath(t *testing.T) {
	r := require.New(t)
	r.Equal("", localChainDBPath(""))
	r.Equal("/var/data/chain.db", localChainDBPath("/var/data/chain.db"))
	r.Equal("/var/data/chain.db", localChainDBPath("file:///var/data/chain.db"))
	// grpc (remote block DAO) has no local file to stat
	r.Equal("", localChainDBPath("grpc://host:1234?insecure=true"))
}

func TestConfiguredDBFiles_SkipsRemoteChainDB(t *testing.T) {
	r := require.New(t)
	cfg := config.Config{}
	cfg.Chain.ChainDBPath = "grpc://host:1234"
	cfg.Chain.TrieDBPath = "/data/trie.db"

	files := configuredDBFiles(cfg)
	for _, f := range files {
		r.NotEqual("chain", f.name, "remote chain db must not produce a series")
	}
}

func TestNewDBFileSizeCollector_NilWhenNoFiles(t *testing.T) {
	r := require.New(t)
	r.Nil(newDBFileSizeCollector(config.Config{}))
}

func TestNewDBFileSizeCollector_PopulatesGauge(t *testing.T) {
	r := require.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.db")
	r.NoError(os.WriteFile(path, []byte("0123456789"), 0o600))

	cfg := config.Config{}
	cfg.Chain.ChainDBPath = path
	collector := newDBFileSizeCollector(cfg)
	r.NotNil(collector)

	// collector samples once at construction time
	g, err := dbFileSizeMtc.GetMetricWithLabelValues("chain", path)
	r.NoError(err)
	var m dto.Metric
	r.NoError(g.Write(&m))
	r.Equal(float64(10), m.GetGauge().GetValue())
}
