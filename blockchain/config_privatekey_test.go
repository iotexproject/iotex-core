// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

const (
	hashiCorpVaultTestCfg = `
method: hashiCorpVault
hashiCorpVault:
    address: http://127.0.0.1:8200
    token: secret/data/test
    path: secret/data/test
    key: my key
`

	vaultTestKey   = "my key"
	vaultTestValue = "my value"
)

type (
	mockVaultSuccess          struct{}
	mockVaultNoSecret         struct{}
	mockVaultInvalidDataType  struct{}
	mockVaultNoValue          struct{}
	mockVaultInvalidValueType struct{}
)

func (m *mockVaultSuccess) Read(path string) (*api.Secret, error) {
	return &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				vaultTestKey: vaultTestValue,
			},
		},
	}, nil
}

func (m *mockVaultNoSecret) Read(path string) (*api.Secret, error) {
	return nil, nil
}

func (m *mockVaultInvalidDataType) Read(path string) (*api.Secret, error) {
	return &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]string{
				vaultTestKey: vaultTestValue,
			},
		},
	}, nil
}

func (m *mockVaultNoValue) Read(path string) (*api.Secret, error) {
	return &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{},
		},
	}, nil
}

func (m *mockVaultInvalidValueType) Read(path string) (*api.Secret, error) {
	return &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				vaultTestKey: 123,
			},
		},
	}, nil
}

func newMockVaultClientSuccess() *vaultClient {
	return &vaultClient{&mockVaultSuccess{}}
}

func newMockVaultClientNoSecret() *vaultClient {
	return &vaultClient{&mockVaultNoSecret{}}
}

func newMockVaultClientInvalidDataType() *vaultClient {
	return &vaultClient{&mockVaultInvalidDataType{}}
}

func newMockVaultClientNoValue() *vaultClient {
	return &vaultClient{&mockVaultNoValue{}}
}

func newMockVaultClientInvalidValueType() *vaultClient {
	return &vaultClient{&mockVaultInvalidValueType{}}
}

func TestVault(t *testing.T) {
	r := require.New(t)
	cfg := &hashiCorpVault{
		Address: "http://127.0.0.1:8200",
		Token:   "hello iotex",
		Path:    "secret/data/test",
		Key:     vaultTestKey,
	}
	t.Run("new vault client", func(t *testing.T) {
		_, err := newVaultClient(cfg)
		r.NoError(err)
	})
	t.Run("vault success", func(t *testing.T) {
		cli := newMockVaultClientSuccess()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		res, err := loader.load()
		r.NoError(err)
		r.Equal(vaultTestValue, res)
	})
	t.Run("vault no secret", func(t *testing.T) {
		cli := newMockVaultClientNoSecret()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		_, err := loader.load()
		r.Contains(err.Error(), "secret does not exist")
	})
	t.Run("vault invalid data type", func(t *testing.T) {
		cli := newMockVaultClientInvalidDataType()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		_, err := loader.load()
		r.Contains(err.Error(), "invalid data type")
	})
	t.Run("vault no value", func(t *testing.T) {
		cli := newMockVaultClientNoValue()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		_, err := loader.load()
		r.Contains(err.Error(), "secret value does not exist")
	})
	t.Run("vault invalid secret value type", func(t *testing.T) {
		cli := newMockVaultClientInvalidValueType()
		loader := &vaultPrivKeyLoader{
			cfg:         cfg,
			vaultClient: cli,
		}
		_, err := loader.load()
		r.Contains(err.Error(), "invalid secret value type")
	})
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
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.Contains(err.Error(), "invalid private key method")
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
		r.Contains(err.Error(), "dial tcp 127.0.0.1:8200: connect: connection refused")
	})
}
