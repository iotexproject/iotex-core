// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package ioctl

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

type (
	// Client defines the interface of an ioctl client
	Client interface {
		// Start starts the client
		Start(context.Context) error
		// Stop stops the client
		Stop(context.Context) error
		// Config returns the config of the client
		Config() config.Config
		// APIServiceClient returns an API service client
		APIServiceClient(APIServiceConfig) (iotexapi.APIServiceClient, error)
		// SelectTranslation select a translation based on UILanguage
		SelectTranslation(map[config.Language]string) (string, config.Language)
		// AskToConfirm asks user to confirm from terminal, true to continue
		AskToConfirm(string) bool
		// ReadSecret reads password from terminal
		ReadSecret() (string, error)
		// Execute a bash command
		Execute(string) error
		// AddressWithDefaultIfNotExist returns default address if input empty
		AddressWithDefaultIfNotExist(in string) (string, error)
		// Address returns address if input address|alias
		Address(in string) (string, error)
		// NewKeyStore creates a keystore by default walletdir
		NewKeyStore() *keystore.KeyStore
		// DecryptPrivateKey returns privateKey from a json blob
		DecryptPrivateKey(string, string) (*ecdsa.PrivateKey, error)
		// AliasMap returns the alias map: accountAddr-aliasName
		AliasMap() map[string]string
		// SetAliasUnwritten updates aliasname and account address and not write them into the default config file
		SetAliasUnwritten(string, string)
		// SetAlias updates aliasname and account address and write them into the default config file
		SetAlias(string, string) error
		// DeleteAlias delete alias from the default config file
		DeleteAlias(string) error
		// WriteConfig write config datas to the default config file
		WriteConfig() error
		// IsCryptoSm2 return true if use sm2 cryptographic algorithm, false if not use
		IsCryptoSm2() bool
		// QueryAnalyser sends request to Analyser endpoint
		QueryAnalyser(interface{}) (*http.Response, error)
	}

	// APIServiceConfig defines a config of APIServiceClient
	APIServiceConfig struct {
		Endpoint string
		Insecure bool
	}

	client struct {
		cfg            config.Config
		conn           *grpc.ClientConn
		cryptoSm2      bool
		configFilePath string
	}

	// Option sets client construction parameter
	Option func(*client)

	// ConfirmationMessage is the struct of an Confirmation output
	ConfirmationMessage struct {
		Info    string   `json:"info"`
		Options []string `json:"options"`
	}
)

// EnableCryptoSm2 enables to use sm2 cryptographic algorithm
func EnableCryptoSm2() Option {
	return func(c *client) {
		c.cryptoSm2 = true
	}
}

// NewClient creates a new ioctl client
func NewClient(cfg config.Config, configFilePath string, opts ...Option) Client {
	c := &client{
		cfg:            cfg,
		configFilePath: configFilePath,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *client) Start(context.Context) error {
	return nil
}

func (c *client) Stop(context.Context) error {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
		c.conn = nil
	}
	return nil
}

func (c *client) Config() config.Config {
	return c.cfg
}

func (c *client) AskToConfirm(info string) bool {
	message := ConfirmationMessage{Info: info, Options: []string{"yes"}}
	fmt.Println(message.String())
	var confirm string
	fmt.Scanf("%s", &confirm)
	return strings.EqualFold(confirm, "yes")
}

func (c *client) SelectTranslation(trls map[config.Language]string) (string, config.Language) {
	trl, ok := trls[c.cfg.Lang()]
	if ok {
		return trl, c.cfg.Lang()
	}

	trl, ok = trls[config.English]
	if !ok {
		panic("failed to pick a translation")
	}
	return trl, config.English
}

func (c *client) ReadSecret() (string, error) {
	// TODO: delete util.ReadSecretFromStdin, and move code to here
	return util.ReadSecretFromStdin()
}

func (c *client) APIServiceClient(cfg APIServiceConfig) (iotexapi.APIServiceClient, error) {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return nil, err
		}
	}
	if cfg.Endpoint == "" {
		return nil, errors.New(`use "ioctl config set endpoint" to config endpoint first`)
	}

	var err error
	if cfg.Insecure {
		c.conn, err = grpc.Dial(cfg.Endpoint, grpc.WithInsecure())
	} else {
		c.conn, err = grpc.Dial(cfg.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	if err != nil {
		return nil, err
	}
	return iotexapi.NewAPIServiceClient(c.conn), nil
}

func (c *client) Execute(cmd string) error {
	return exec.Command("bash", "-c", cmd).Run()
}

func (c *client) AddressWithDefaultIfNotExist(in string) (string, error) {
	var address string
	if !strings.EqualFold(in, "") {
		address = in
	} else {
		if strings.EqualFold(c.cfg.DefaultAccount.AddressOrAlias, "") {
			return "", errors.New(`use "ioctl config set defaultacc ADDRESS|ALIAS" to config default account first`)
		}
		address = c.cfg.DefaultAccount.AddressOrAlias
	}
	return c.Address(address)
}

func (c *client) Address(in string) (string, error) {
	if len(in) >= validator.IoAddrLen {
		if err := validator.ValidateAddress(in); err != nil {
			return "", err
		}
		return in, nil
	}
	addr, ok := c.cfg.Aliases[in]
	if ok {
		return addr, nil
	}
	return "", errors.New("cannot find address from " + in)
}

func (c *client) NewKeyStore() *keystore.KeyStore {
	return keystore.NewKeyStore(c.cfg.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
}

func (c *client) DecryptPrivateKey(passwordOfKeyStore, keyStorePath string) (*ecdsa.PrivateKey, error) {
	keyJSON, err := os.ReadFile(keyStorePath)
	if err != nil {
		return nil, fmt.Errorf("keystore file \"%s\" read error", keyStorePath)
	}

	key, err := keystore.DecryptKey(keyJSON, passwordOfKeyStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt key")
	}
	if key != nil && key.PrivateKey != nil {
		// clear private key in memory prevent from attack
		defer func(k *ecdsa.PrivateKey) {
			b := k.D.Bits()
			for i := range b {
				b[i] = 0
			}
		}(key.PrivateKey)
	}
	return key.PrivateKey, nil
}

func (c *client) AliasMap() map[string]string {
	aliases := make(map[string]string)
	for name, addr := range c.cfg.Aliases {
		aliases[addr] = name
	}
	return aliases
}

func (c *client) SetAliasUnwritten(aliasName string, addr string) {
	for k, v := range c.cfg.Aliases {
		if v == addr {
			delete(c.cfg.Aliases, k)
		}
	}
	c.cfg.Aliases[aliasName] = addr
}

func (c *client) SetAlias(aliasName string, addr string) error {
	c.SetAliasUnwritten(aliasName, addr)
	return c.WriteConfig()
}

func (c *client) DeleteAlias(aliasName string) error {
	delete(c.cfg.Aliases, aliasName)
	return c.WriteConfig()
}

func (c *client) WriteConfig() error {
	out, err := yaml.Marshal(&c.cfg)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal config to config file %s", c.configFilePath)
	}
	if err = os.WriteFile(c.configFilePath, out, 0600); err != nil {
		return errors.Wrapf(err, "failed to write to config file %s", c.configFilePath)
	}
	return nil
}

func (c *client) IsCryptoSm2() bool {
	return c.cryptoSm2
}

func (c *client) QueryAnalyser(reqData interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack in json")
	}
	resp, err := http.Post(c.cfg.AnalyserEndpoint+"/api.ActionsService.GetActionsByAddress", "application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}
	return resp, nil
}

func (m *ConfirmationMessage) String() string {
	line := fmt.Sprintf("%s\nOptions:", m.Info)
	for _, option := range m.Options {
		line += " " + option
	}
	line += "\nQuit for anything else."
	return line
}
