// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_privatekey"
)

const (
	hashiCorpVaultTestCfg = `
address: http://127.0.0.1:8200
token: secret/data/test
path: secret/data/test
key: my key
`

	vaultTestKey   = "my key"
	vaultTestValue = "my value"
)

func TestVault(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	reader := mock_privatekey.NewMockvaultSecretReader(ctrl)
	cfg := &hashiCorpVault{
		Address: "http://127.0.0.1:8200",
		Token:   "hello iotex",
		Path:    "secret/data/test",
		Key:     vaultTestKey,
	}
	loader := &vaultPrivKeyLoader{
		cfg:         cfg,
		vaultClient: &vaultClient{reader},
	}

	t.Run("NewVaultPrivKeyLoaderSuccess", func(t *testing.T) {
		_, err := newVaultPrivKeyLoader(cfg)
		r.NoError(err)
	})
	t.Run("VaultSuccess", func(t *testing.T) {
		reader.EXPECT().Read(gomock.Any()).Return(&api.Secret{
			Data: map[string]interface{}{
				"data": map[string]interface{}{
					vaultTestKey: vaultTestValue,
				},
			},
		}, nil)
		res, err := loader.load()
		r.NoError(err)
		r.Equal(vaultTestValue, res)
	})
	t.Run("VaultNoSecret", func(t *testing.T) {
		reader.EXPECT().Read(gomock.Any()).Return(nil, nil)
		_, err := loader.load()
		r.Contains(err.Error(), "secret does not exist")
	})
	t.Run("VaultInvalidDataType", func(t *testing.T) {
		reader.EXPECT().Read(gomock.Any()).Return(&api.Secret{
			Data: map[string]interface{}{
				"data": map[string]string{
					vaultTestKey: vaultTestValue,
				},
			},
		}, nil)
		_, err := loader.load()
		r.Contains(err.Error(), "invalid data type")
	})
	t.Run("VaultNoValue", func(t *testing.T) {
		reader.EXPECT().Read(gomock.Any()).Return(&api.Secret{
			Data: map[string]interface{}{
				"data": map[string]interface{}{},
			},
		}, nil)
		_, err := loader.load()
		r.Contains(err.Error(), "secret value does not exist")
	})
	t.Run("VaultInvalidSecretValueType", func(t *testing.T) {
		reader.EXPECT().Read(gomock.Any()).Return(&api.Secret{
			Data: map[string]interface{}{
				"data": map[string]interface{}{
					vaultTestKey: 123,
				},
			},
		}, nil)
		_, err := loader.load()
		r.Contains(err.Error(), "invalid secret value type")
	})
}

func TestSetProducerPrivKey(t *testing.T) {
	r := require.New(t)
	testfile := "private_key.*.yaml"
	t.Run("PrivateConfigFileDoesNotExist", func(t *testing.T) {
		cfg := DefaultConfig
		key := DefaultConfig.ProducerPrivKey
		err := cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal(key, cfg.ProducerPrivKey)
	})
	t.Run("PrivateConfigUnknownSchema", func(t *testing.T) {
		cfg := DefaultConfig
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())
		cfg.ProducerPrivKey = tmp.Name()
		cfg.ProducerPrivKeySchema = "unknown"
		err = cfg.SetProducerPrivKey()
		r.Contains(err.Error(), "invalid private key schema")
	})
	t.Run("PrivateConfigFileHasHashiCorpVault", func(t *testing.T) {
		cfg := DefaultConfig
		tmp, err := os.CreateTemp("", testfile)
		r.NoError(err)
		defer os.Remove(tmp.Name())

		_, err = tmp.WriteString(hashiCorpVaultTestCfg)
		r.NoError(err)
		err = tmp.Close()
		r.NoError(err)
		cfg.ProducerPrivKey = tmp.Name()
		cfg.ProducerPrivKeySchema = "hashiCorpVault"
		err = cfg.SetProducerPrivKey()
		r.Contains(err.Error(), "dial tcp 127.0.0.1:8200: connect: connection refused")
	})
}
