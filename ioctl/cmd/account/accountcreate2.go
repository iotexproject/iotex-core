// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/tyler-smith/go-bip39"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"

	ecrypto "github.com/ethereum/go-ethereum/crypto"
)

// Multi-language support
var (
	createByMnemonicCmdShorts = map[config.Language]string{
		config.English: "Create new account for ioctl by mnemonic",
		config.Chinese: "为ioctl创建新账户通过助记词",
	}
	createByMnemonicCmdUses = map[config.Language]string{
		config.English: "create2",
		config.Chinese: "create2 创建",
	}

	mnemonicConfigFilePath = filepath.Join(config.ReadConfig.Wallet, "mnemonic.yml")
)

// accountCreateCmd represents the account create command
var accountCreateByMnemonicCmd = &cobra.Command{
	Use:   config.TranslateInLang(createByMnemonicCmdUses, config.UILanguage),
	Short: config.TranslateInLang(createByMnemonicCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountCreateByMnemonic()
		return output.PrintError(err)
	},
}

type mnemonicConfig struct {
	PrivateKey []byte `yaml:"k"`
	Mnemonic   []byte `yaml:"m"`
	Password   []byte `yaml:"p"`
	ID         uint32 `yaml:"i"`
}

func (c *mnemonicConfig) DecryptMnemonic() ([]byte, error) {

	privateKey, err := x509.ParsePKCS1PrivateKey(c.PrivateKey)
	if err != nil {
		return nil, err
	}
	return privateKey.Decrypt(nil, c.Mnemonic, &rsa.OAEPOptions{Hash: crypto.SHA256})
}

func (c *mnemonicConfig) DecryptPassword() ([]byte, error) {

	privateKey, err := x509.ParsePKCS1PrivateKey(c.PrivateKey)
	if err != nil {
		return nil, err
	}
	return privateKey.Decrypt(nil, c.Password, &rsa.OAEPOptions{Hash: crypto.SHA256})
}

func hashECDSAPublicKey(publicKey *ecdsa.PublicKey) []byte {
	k := ecrypto.FromECDSAPub(publicKey)
	h := hash.Hash160b(k[1:])
	return h[:]
}

func init() {
	accountCreateByMnemonicCmd.Flags().UintVarP(&numAccounts, "num", "n", 1,
		config.TranslateInLang(flagNumUsages, config.UILanguage))
}

func createUserByMnemonic() error {
	output.PrintQuery("Set password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	output.PrintQuery("Enter password again\n")
	passwordAgain, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}

	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	output.PrintResult(fmt.Sprintf("Mnemonic pharse: %s\n"+
		"Save them somewhere safe and secret.", mnemonic))

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	publicKey := privateKey.PublicKey

	encryptedMnemonic, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		&publicKey,
		[]byte(mnemonic),
		nil)
	if err != nil {
		return err
	}

	encryptedPassword, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		&publicKey,
		[]byte(password),
		nil)
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(&mnemonicConfig{
		PrivateKey: x509.MarshalPKCS1PrivateKey(privateKey),
		Mnemonic:   encryptedMnemonic,
		Password:   encryptedPassword,
	})
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := ioutil.WriteFile(mnemonicConfigFilePath, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", mnemonicConfigFilePath), err)
	}

	return nil
}

func readMnemonicConfig() (*mnemonicConfig, error) {
	var cfg *mnemonicConfig
	out, err := ioutil.ReadFile(mnemonicConfigFilePath)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(out, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func accountCreateByMnemonic() error {
	var err error
	newAccounts := make([]generatedAccount, 0)

	if !fileutil.FileExists(mnemonicConfigFilePath) {
		return createUserByMnemonic()
	}

	rmc, err := readMnemonicConfig()
	mnemonic, err := rmc.DecryptMnemonic()
	if err != nil {
		return err
	}

	password, err := rmc.DecryptPassword()
	if err != nil {
		return err
	}
	output.PrintResult(fmt.Sprintf("mnemonic:%s\npassword:%s", mnemonic, password))

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
	if err != nil {
		return err
	}

	var i uint32
	for i = rmc.ID; i < rmc.ID+uint32(numAccounts); i++ {
		//https://gowalker.org/github.com/ethereum/go-ethereum/accounts#DefaultBaseDerivationPath
		derivationPath := hdwallet.DefaultRootDerivationPath
		path := hdwallet.MustParseDerivationPath(fmt.Sprintf("%s/%d", derivationPath.String(), i))
		account, err := wallet.Derive(path, false)
		if err != nil {
			return err
		}

		private, err := wallet.PrivateKey(account)
		if err != nil {
			return err
		}
		addr, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
		if err != nil {
			return output.NewError(output.ConvertError, "failed to convert public key into address", err)
		}
		newAccount := generatedAccount{
			Address:    addr.String(),
			PrivateKey: fmt.Sprintf("%x", ecrypto.FromECDSA(private)),
			PublicKey:  fmt.Sprintf("%x", ecrypto.FromECDSAPub(&private.PublicKey)),
		}
		newAccounts = append(newAccounts, newAccount)
	}
	message := createMessage{Accounts: newAccounts}

	rmc.ID = i
	out, err := yaml.Marshal(rmc)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := ioutil.WriteFile(mnemonicConfigFilePath, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", mnemonicConfigFilePath), err)
	}

	fmt.Println(message.String())

	return err
}
