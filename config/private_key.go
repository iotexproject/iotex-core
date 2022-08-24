// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"net/http"
	"os"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/pkg/errors"
	uconfig "go.uber.org/config"
)

const defaultHTTPTimeout = 10 * time.Second

type (
	privKeyConfig struct {
		ProducerPrivKey string `yaml:"producerPrivKey"`
		HashiCorpVault  *struct {
			Address string `yaml:"address"`
			Token   string `yaml:"token"`
			Path    string `yaml:"path"`
			Key     string `yaml:"key"`
		} `yaml:"hashiCorpVault"`
	}

	privKeyLoader interface {
		load() (string, error)
	}

	localPrivKeyLoader struct {
		cfg *privKeyConfig
	}

	vaultPrivKeyLoader struct {
		cfg *privKeyConfig
	}
)

func (l *localPrivKeyLoader) load() (string, error) {
	return l.cfg.ProducerPrivKey, nil
}

func (l *vaultPrivKeyLoader) load() (string, error) {
	vc := l.cfg.HashiCorpVault
	client, err := vault.NewClient(&vault.Config{
		Address: vc.Address,
		HttpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to init vault client")
	}
	client.SetToken(vc.Token)

	secret, err := client.Logical().Read(vc.Path)
	if err != nil {
		return "", errors.Wrap(err, "failed to read vault secret")
	}
	if secret == nil {
		return "", errors.New("vault secret not exist")
	}
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("vault data type invalid")
	}
	value, ok := data[vc.Key]
	if !ok {
		return "", errors.New("vault secret value not exist")
	}
	v, ok := value.(string)
	if !ok {
		return "", errors.New("vault secret value type invalid")
	}

	return v, nil
}

func setProducerPrivKey(cfg *blockchain.Config, privKeyPath string) error {
	if privKeyPath == "" {
		return nil
	}

	yaml, err := uconfig.NewYAML(uconfig.Expand(os.LookupEnv), uconfig.File(privKeyPath))
	if err != nil {
		return errors.Wrap(err, "failed to init private key config")
	}
	pc := &privKeyConfig{}
	if err := yaml.Get(uconfig.Root).Populate(pc); err != nil {
		return errors.Wrap(err, "failed to unmarshal YAML config to privKeyConfig struct")
	}

	var loader privKeyLoader
	switch {
	case pc.ProducerPrivKey != "":
		loader = &localPrivKeyLoader{pc}
	case pc.HashiCorpVault != nil:
		loader = &vaultPrivKeyLoader{pc}
	default:
		return nil
	}

	key, err := loader.load()
	if err != nil {
		return errors.Wrap(err, "failed to load producer private key")
	}
	cfg.ProducerPrivKey = key
	return nil
}
