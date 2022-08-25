// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = 10 * time.Second

var ErrVault = errors.New("vault error")

type (
	hashiCorpVault struct {
		Address string `yaml:"address"`
		Token   string `yaml:"token"`
		Path    string `yaml:"path"`
		Key     string `yaml:"key"`
	}

	privKeyConfig struct {
		ProducerPrivKey string          `yaml:"producerPrivKey"`
		VaultConfig     *hashiCorpVault `yaml:"hashiCorpVault"`
	}

	privKeyLoader interface {
		load() (string, error)
	}

	localPrivKeyLoader struct {
		privKey string
	}

	vaultPrivKeyLoader struct {
		cfg *hashiCorpVault
	}
)

func (l *localPrivKeyLoader) load() (string, error) {
	return l.privKey, nil
}

func (l *vaultPrivKeyLoader) load() (string, error) {
	conf := api.DefaultConfig()
	conf.Address = l.cfg.Address
	conf.Timeout = defaultHTTPTimeout
	cli, err := api.NewClient(conf)
	if err != nil {
		return "", errors.Wrap(err, "failed to init vault client")
	}
	cli.SetToken(l.cfg.Token)

	secret, err := cli.Logical().Read(l.cfg.Path)
	if err != nil {
		return "", errors.Wrap(err, "failed to read vault secret")
	}
	if secret == nil {
		return "", errors.Wrap(ErrVault, "secret does not exist")
	}
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", errors.Wrap(ErrVault, "invalid data type")
	}
	value, ok := data[l.cfg.Key]
	if !ok {
		return "", errors.Wrap(ErrVault, "secret value does not exist")
	}
	v, ok := value.(string)
	if !ok {
		return "", errors.Wrap(ErrVault, "invalid secret value type")
	}

	return v, nil
}
