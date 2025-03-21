// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/hdwallet"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/ioctl/validator"
)

// Multi-language support
var (
	_accountCmdShorts = map[config.Language]string{
		config.English: "Manage accounts of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的账号",
	}
	_flagEndpoint = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecure = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接",
	}
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// CryptoSm2 is a flag for sm2 cryptographic algorithm
var CryptoSm2 bool

var _passwordFlag = flag.NewStringVarP("password", "P", "", "input password for account")

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   "account",
	Short: config.TranslateInLang(_accountCmdShorts, config.UILanguage),
}

func init() {
	AccountCmd.AddCommand(accountBalanceCmd)
	AccountCmd.AddCommand(_accountCreateCmd)
	AccountCmd.AddCommand(_accountCreateAddCmd)
	AccountCmd.AddCommand(_accountDeleteCmd)
	AccountCmd.AddCommand(_accountEthaddrCmd)
	AccountCmd.AddCommand(_accountExportCmd)
	AccountCmd.AddCommand(_accountExportPublicCmd)
	AccountCmd.AddCommand(_accountImportCmd)
	AccountCmd.AddCommand(_accountInfoCmd)
	AccountCmd.AddCommand(_accountListCmd)
	AccountCmd.AddCommand(_accountNonceCmd)
	AccountCmd.AddCommand(_accountSignCmd)
	AccountCmd.AddCommand(_accountUpdateCmd)
	AccountCmd.AddCommand(_accountVerifyCmd)
	AccountCmd.AddCommand(_accountActionsCmd)
	AccountCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpoint, config.UILanguage))
	AccountCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure, config.TranslateInLang(_flagInsecure, config.UILanguage))
}

func RegisterPasswordFlag(cmd *cobra.Command) {
	_passwordFlag.RegisterCommand(cmd)
}

func PasswordByFlag() string {
	return _passwordFlag.Value().(string)
}

// Sign sign message with signer
func Sign(signer, password, message string) (string, error) {
	pri, err := PrivateKeyFromSigner(signer, password)
	if err != nil {
		return "", err
	}
	mes := message
	head := message[:2]
	if strings.EqualFold(head, "0x") {
		mes = message[2:]
	}
	b, err := hex.DecodeString(mes)
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(b))
	msg := append([]byte(prefix), b...)
	mesToSign := hash.Hash256b(msg)
	ret, err := pri.Sign(mesToSign[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ret), nil
}

// keyStoreAccountToPrivateKey generates our PrivateKey interface from Keystore account
func keyStoreAccountToPrivateKey(signer, password string) (crypto.PrivateKey, error) {
	addrString, err := util.Address(signer)
	if err != nil {
		return nil, err
	}
	addr, err := address.FromString(addrString)
	if err != nil {
		return nil, fmt.Errorf("invalid account #%s, addr %s", signer, addrString)
	}

	if CryptoSm2 {
		// find the account in pem files
		pemFilePath := sm2KeyPath(addr)
		prvKey, err := crypto.ReadPrivateKeyFromPem(pemFilePath, password)
		if err == nil {
			return prvKey, nil
		}
	} else {
		// find the account in keystore
		ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
			keystore.StandardScryptN, keystore.StandardScryptP)
		for _, account := range ks.Accounts() {
			if bytes.Equal(addr.Bytes(), account.Address.Bytes()) {
				return crypto.KeystoreToPrivateKey(account, password)
			}
		}
	}

	return nil, fmt.Errorf("account #%s does not match all local keys", signer)
}

// PrivateKeyFromSigner returns private key from signer
func PrivateKeyFromSigner(signer, password string) (crypto.PrivateKey, error) {
	var (
		prvKey crypto.PrivateKey
		err    error
	)

	if !IsSignerExist(signer) && !util.AliasIsHdwalletKey(signer) {
		return nil, fmt.Errorf("invalid address #%s", signer)
	}

	if password == "" {
		output.PrintQuery(fmt.Sprintf("Enter password for #%s:\n", signer))
		password, err = util.ReadSecretFromStdin()
		if err != nil {
			return nil, output.NewError(output.InputError, "failed to get password", err)
		}
	}

	if util.AliasIsHdwalletKey(signer) {
		account, change, index, err := util.ParseHdwPath(signer)
		if err != nil {
			return nil, output.NewError(output.InputError, "invalid HDWallet key format", err)
		}
		_, prvKey, err = hdwallet.DeriveKey(account, change, index, password)
		if err != nil {
			return nil, output.NewError(output.InputError, "failed to derive key from HDWallet", err)
		}
		return prvKey, nil
	}
	return keyStoreAccountToPrivateKey(signer, password)
}

// GetAccountMeta gets account metadata
func GetAccountMeta(addr string) (*iotextypes.AccountMeta, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := iotexapi.GetAccountRequest{Address: addr}

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}
	response, err := cli.GetAccount(ctx, &request)

	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetAccount api", err)
	}
	return response.AccountMeta, nil
}

// IsSignerExist checks whether signer account is existed
func IsSignerExist(signer string) bool {
	addr, err := address.FromString(signer)
	if err != nil {
		return false
	}

	if CryptoSm2 {
		// find the account in pem files
		_, err = findSm2PemFile(addr)
		return err == nil
	}

	// find the account in keystore
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, ksAccount := range ks.Accounts() {
		if address.Equal(addr, ksAccount.Address) {
			return true
		}
	}

	return false
}

func newAccount(alias string) (string, error) {
	output.PrintQuery(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return "", output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", output.NewError(output.KeystoreError, "failed to create new keystore", err)
	}
	addr, err := address.FromBytes(account.Address.Bytes())
	if err != nil {
		return "", output.NewError(output.ConvertError, "failed to convert bytes into address", err)
	}
	return addr.String(), nil
}

func newAccountSm2(alias string) (string, error) {
	output.PrintQuery(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return "", output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}
	priKey, err := crypto.GenerateKeySm2()
	if err != nil {
		return "", output.NewError(output.CryptoError, "failed to generate sm2 private key", err)
	}

	addr := priKey.PublicKey().Address()
	if addr == nil {
		return "", output.NewError(output.ConvertError, "failed to convert bytes into address", nil)
	}

	pemFilePath := sm2KeyPath(addr)
	if err := crypto.WritePrivateKeyToPem(pemFilePath, priKey.(*crypto.P256sm2PrvKey), password); err != nil {
		return "", output.NewError(output.KeystoreError, "failed to save private key into pem file ", err)
	}

	return addr.String(), nil
}

func newAccountByKey(alias string, privateKey string, walletDir string) (string, error) {
	output.PrintQuery(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return "", output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}

	return storeKey(privateKey, walletDir, password)
}

func newAccountByKeyStore(alias, passwordOfKeyStore, keyStorePath string, walletDir string) (string, error) {
	keyJSON, err := os.ReadFile(filepath.Clean(keyStorePath))
	if err != nil {
		return "", output.NewError(output.ReadFileError,
			fmt.Sprintf("keystore file \"%s\" read error", keyStorePath), nil)
	}
	key, err := keystore.DecryptKey(keyJSON, passwordOfKeyStore)
	if key != nil && key.PrivateKey != nil {
		// clear private key in memory prevent from attack
		defer func(k *ecdsa.PrivateKey) {
			b := k.D.Bits()
			for i := range b {
				b[i] = 0
			}
		}(key.PrivateKey)
	}
	if err != nil {
		return "", output.NewError(output.KeystoreError, "failed to decrypt key", err)
	}
	return newAccountByKey(alias, hex.EncodeToString(ecrypto.FromECDSA(key.PrivateKey)), walletDir)
}

func newAccountByPem(alias, passwordOfPem, pemFilePath string, walletDir string) (string, error) {
	prvKey, err := crypto.ReadPrivateKeyFromPem(pemFilePath, passwordOfPem)
	if err != nil {
		return "", output.NewError(output.CryptoError, "failed to read private key from pem file", err)
	}

	return newAccountByKey(alias, prvKey.HexString(), walletDir)
}

func storeKey(privateKey, walletDir, password string) (string, error) {
	priKey, err := crypto.HexStringToPrivateKey(privateKey)
	if err != nil {
		return "", output.NewError(output.CryptoError, "failed to generate private key from hex string ", err)
	}
	defer priKey.Zero()

	addr := priKey.PublicKey().Address()
	if addr == nil {
		return "", output.NewError(output.ConvertError, "failed to convert bytes into address", nil)
	}

	switch sk := priKey.EcdsaPrivateKey().(type) {
	case *ecdsa.PrivateKey:
		ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
		if _, err := ks.ImportECDSA(sk, password); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to import private key into keystore ", err)
		}
	case *crypto.P256sm2PrvKey:
		pemFilePath := sm2KeyPath(addr)
		if err := crypto.WritePrivateKeyToPem(pemFilePath, sk, password); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to save private key into pem file ", err)
		}
	default:
		return "", output.NewError(output.CryptoError, "invalid private key", nil)
	}

	return addr.String(), nil
}

func sm2KeyPath(addr address.Address) string {
	return filepath.Join(config.ReadConfig.Wallet, "sm2sk-"+addr.String()+".pem")
}

func findSm2PemFile(addr address.Address) (string, error) {
	filePath := sm2KeyPath(addr)
	_, err := os.Stat(filePath)
	if err != nil {
		return "", output.NewError(output.ReadFileError, "crypto file not found", err)
	}
	return filePath, nil
}

func listSm2Account() ([]string, error) {
	sm2Accounts := make([]string, 0)
	files, err := os.ReadDir(config.ReadConfig.Wallet)
	if err != nil {
		return nil, output.NewError(output.ReadFileError, "failed to read files in wallet", err)
	}
	for _, f := range files {
		if !f.IsDir() {
			if strings.HasSuffix(f.Name(), ".pem") {
				addr := strings.TrimSuffix(strings.TrimPrefix(f.Name(), "sm2sk-"), ".pem")
				if err := validator.ValidateAddress(addr); err == nil {
					sm2Accounts = append(sm2Accounts, addr)
				}
			}
		}
	}
	return sm2Accounts, nil
}
