// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

const hashiCorpVaultTestCfg = `
producerPrivKey: my private key
hashiCorpVault:
    address: http://127.0.0.1:8200
    token: secret/data/test
    path: secret/data/test
    key: my key
`

type mockVault struct{}

func (m *mockVault) Read(path string) (*api.Secret, error) {
	return &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				"my key": "my value",
			},
		},
	}, nil
}

func newMockVaultClient() *vaultClient {
	return &vaultClient{
		cli: &mockVault{},
	}
}

func TestSetProducerPrivKey(t *testing.T) {
	r := require.New(t)
	testfile := "private_key.*.yaml"
	t.Run("private config file does not exist", func(t *testing.T) {
		cfg := DefaultConfig
		key := DefaultConfig.ProducerPrivKey
		err := cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal(key, cfg.ProducerPrivKey)
	})
	t.Run("private config file is empty", func(t *testing.T) {
		cfg := DefaultConfig
		key := DefaultConfig.ProducerPrivKey
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal(key, cfg.ProducerPrivKey)
	})
	t.Run("private config file has producerPrivKey", func(t *testing.T) {
		cfg := DefaultConfig
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())
		_, err = tmp.WriteString("producerPrivKey: my private key")
		r.NoError(err)
		err = tmp.Close()
		r.NoError(err)
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal("my private key", cfg.ProducerPrivKey)
	})

	t.Run("private config file has hashiCorpVault", func(t *testing.T) {
		cfg := DefaultConfig
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())

		_, err = tmp.WriteString(hashiCorpVaultTestCfg)
		r.NoError(err)
		err = tmp.Close()
		r.NoError(err)
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.True(strings.Contains(err.Error(), "dial tcp 127.0.0.1:8200: connect: connection refused"))
	})
}

func TestVault(t *testing.T) {
	r := require.New(t)
	cfg := &hashiCorpVault{
		Address: "http://127.0.0.1:8200",
		Token:   "hello iotex",
		Path:    "secret/data/test",
		Key:     "my key",
	}
	t.Run("new vault client", func(t *testing.T) {
		_, err := newVaultClient(cfg)
		r.NoError(err)
	})
	t.Run("vault read", func(t *testing.T) {
		cli := newMockVaultClient()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		res, err := loader.load()
		r.NoError(err)
		r.Equal("my value", res)
	})
}
