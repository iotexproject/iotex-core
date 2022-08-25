// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	"github.com/stretchr/testify/require"
)

const hashiCorpVaultTestCfg = `
producerPrivKey: my private key
hashiCorpVault:
    address: %s
    token: %s
    path: %s
    key: %s
`

func TestSetProducerPrivKey(t *testing.T) {
	r := require.New(t)
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
		tmp, err := os.CreateTemp("", "private_key.*.yaml")
		r.NoError(err)
		defer os.Remove(tmp.Name())
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal(key, cfg.ProducerPrivKey)
	})
	t.Run("private config file has producerPrivKey", func(t *testing.T) {
		cfg := DefaultConfig
		tmp, err := os.CreateTemp("", "private_key.*.yaml")
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
		tmp, err := os.CreateTemp("", "private_key.*.yaml")
		r.NoError(err)
		defer os.Remove(tmp.Name())

		core, _, rootToken := vault.TestCoreUnsealed(t)
		ln, addr := http.TestServer(t, core)
		defer ln.Close()

		path := "secret/data/test"
		key := "my key"
		value := "my value"

		conf := api.DefaultConfig()
		conf.Address = addr
		client, err := api.NewClient(conf)
		r.NoError(err)
		client.SetToken(rootToken)
		_, err = client.Logical().Write(path, map[string]interface{}{
			"data": map[string]interface{}{
				key: value,
			},
		})
		r.NoError(err)

		_, err = tmp.WriteString(fmt.Sprintf(hashiCorpVaultTestCfg, addr, rootToken, path, key))
		r.NoError(err)
		err = tmp.Close()
		r.NoError(err)
		cfg.PrivKeyConfigFile = tmp.Name()
		err = cfg.SetProducerPrivKey()
		r.NoError(err)
		r.Equal(value, cfg.ProducerPrivKey)
	})
}
