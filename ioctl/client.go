// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package ioctl

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

const (
	_urlPattern = `[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
)

var (
	//ErrInvalidEndpointOrInsecure represents that endpoint or insecure is invalid
	ErrInvalidEndpointOrInsecure = errors.New("check endpoint or secureConnect in ~/.config/ioctl/default/config.default or cmd flag value if has")
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
		// ConfigFilePath returns the file path of the config
		ConfigFilePath() string
		// SetEndpointWithFlag receives input flag value
		SetEndpointWithFlag(func(*string, string, string, string))
		// SetInsecureWithFlag receives input flag value
		SetInsecureWithFlag(func(*bool, string, bool, string))
		// APIServiceClient returns an API service client
		APIServiceClient() (iotexapi.APIServiceClient, error)
		// SelectTranslation select a translation based on UILanguage
		SelectTranslation(map[config.Language]string) (string, config.Language)
		// ReadCustomLink scans a custom link from terminal and validates it.
		ReadCustomLink() (string, error)
		// AskToConfirm asks user to confirm from terminal, true to continue
		AskToConfirm(string) (bool, error)
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
		// Alias returns the alias corresponding to address
		Alias(string) (string, error)
		// SetAlias updates aliasname and account address and not write them into the default config file
		SetAlias(string, string)
		// SetAliasAndSave updates aliasname and account address and write them into the default config file
		SetAliasAndSave(string, string) error
		// DeleteAlias delete alias from the default config file
		DeleteAlias(string) error
		// WriteConfig write config datas to the default config file
		WriteConfig() error
		// IsCryptoSm2 return true if use sm2 cryptographic algorithm, false if not use
		IsCryptoSm2() bool
		// QueryAnalyser sends request to Analyser endpoint
		QueryAnalyser(interface{}) (*http.Response, error)
		// ReadInput reads the input from stdin
		ReadInput() (string, error)
		// HdwalletMnemonic returns the mnemonic of hdwallet
		HdwalletMnemonic(string) (string, error)
		// WriteHdWalletConfigFile writes encrypting mnemonic into config file
		WriteHdWalletConfigFile(string, string) error
		// RemoveHdWalletConfigFile removes hdwalletConfigFile
		RemoveHdWalletConfigFile() error
		// IsHdWalletConfigFileExist return true if config file is existed, false if not existed
		IsHdWalletConfigFileExist() bool
		// Insecure returns the insecure connect option of grpc dial, default is false
		Insecure() bool
	}

	client struct {
		cfg                config.Config
		conn               *grpc.ClientConn
		cryptoSm2          bool
		configFilePath     string
		endpoint           string
		insecure           bool
		hdWalletConfigFile string
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
		cfg:                cfg,
		configFilePath:     configFilePath,
		hdWalletConfigFile: cfg.Wallet + "/hdwallet",
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

// ConfigFilePath returns the file path for the config.
func (c *client) ConfigFilePath() string {
	return c.configFilePath
}

func (c *client) SetEndpointWithFlag(cb func(*string, string, string, string)) {
	usage, _ := c.SelectTranslation(map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	})
	cb(&c.endpoint, "endpoint", c.cfg.Endpoint, usage)
}

func (c *client) SetInsecureWithFlag(cb func(*bool, string, bool, string)) {
	usage, _ := c.SelectTranslation(map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接",
	})
	cb(&c.insecure, "insecure", !c.cfg.SecureConnect, usage)
}

func (c *client) AskToConfirm(info string) (bool, error) {
	message := ConfirmationMessage{Info: info, Options: []string{"yes"}}
	fmt.Println(message.String())
	var confirm string
	if _, err := fmt.Scanf("%s", &confirm); err != nil {
		return false, err
	}
	return strings.EqualFold(confirm, "yes"), nil
}

func (c *client) ReadCustomLink() (string, error) { // notest
	var link string
	if _, err := fmt.Scanln(&link); err != nil {
		return "", err
	}

	match, err := regexp.MatchString(_urlPattern, link)
	if err != nil {
		return "", errors.Wrapf(err, "failed to validate link %s", link)
	}
	if match {
		return link, nil
	}
	return "", errors.Errorf("link is not a valid url %s", link)
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

func (c *client) APIServiceClient() (iotexapi.APIServiceClient, error) {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return nil, err
		}
	}

	if c.endpoint == "" {
		return nil, errors.New(`use "ioctl config set endpoint" to config endpoint first`)
	}

	var err error
	if c.insecure {
		c.conn, err = grpc.Dial(c.endpoint, grpc.WithInsecure())
	} else {
		c.conn, err = grpc.Dial(c.endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
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
	keyJSON, err := os.ReadFile(filepath.Clean(keyStorePath))
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

func (c *client) Alias(address string) (string, error) {
	if err := validator.ValidateAddress(address); err != nil {
		return "", err
	}
	for aliasName, addr := range c.cfg.Aliases {
		if addr == address {
			return aliasName, nil
		}
	}
	return "", errors.New("no alias is found")
}

func (c *client) SetAlias(aliasName string, addr string) {
	for k, v := range c.cfg.Aliases {
		if v == addr {
			delete(c.cfg.Aliases, k)
		}
	}
	c.cfg.Aliases[aliasName] = addr
}

func (c *client) SetAliasAndSave(aliasName string, addr string) error {
	c.SetAlias(aliasName, addr)
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

func (c *client) ReadInput() (string, error) { // notest
	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return line, nil
}

func (c *client) HdwalletMnemonic(password string) (string, error) {
	// derive key as "m/44'/304'/account'/change/index"
	if !c.IsHdWalletConfigFileExist() {
		return "", errors.New("run 'ioctl hdwallet create' to create your HDWallet first")
	}
	enctxt, err := os.ReadFile(c.hdWalletConfigFile)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read config file %s", c.hdWalletConfigFile)
	}

	enckey := util.HashSHA256([]byte(password))
	dectxt, err := util.Decrypt(enctxt, enckey)
	if err != nil {
		return "", errors.Wrap(err, "failed to decrypt")
	}

	dectxtLen := len(dectxt)
	if dectxtLen <= 32 {
		return "", errors.Errorf("incorrect data dectxtLen %d", dectxtLen)
	}
	mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]
	if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
		return "", errors.New("password error")
	}
	return string(mnemonic), nil
}

func (c *client) WriteHdWalletConfigFile(mnemonic string, password string) error {
	enctxt := append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
	enckey := util.HashSHA256([]byte(password))
	out, err := util.Encrypt(enctxt, enckey)
	if err != nil {
		return errors.Wrap(err, "failed to encrypting mnemonic")
	}
	if err := os.WriteFile(c.hdWalletConfigFile, out, 0600); err != nil {
		return errors.Wrapf(err, "failed to write to config file %s", c.hdWalletConfigFile)
	}
	return nil
}

func (c *client) RemoveHdWalletConfigFile() error {
	return os.Remove(c.hdWalletConfigFile)
}

func (c *client) IsHdWalletConfigFileExist() bool { // notest
	return fileutil.FileExists(c.hdWalletConfigFile)
}

// Insecure returns the insecure connect option of grpc dial, default is false
func (c *client) Insecure() bool {
	return c.insecure
}

func (m *ConfirmationMessage) String() string {
	line := fmt.Sprintf("%s\nOptions:", m.Info)
	for _, option := range m.Options {
		line += " " + option
	}
	line += "\nQuit for anything else."
	return line
}
