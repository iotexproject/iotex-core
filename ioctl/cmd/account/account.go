// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/dustinxie/gmsm/sm2"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	accountCmdShorts = map[config.Language]string{
		config.English: "Manage accounts of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的账号",
	}
	accountCmdUses = map[config.Language]string{
		config.English: "account",
		config.Chinese: "账户",
	}
	flagEndpoint = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagInsecure = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接",
	}
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   config.TranslateInLang(accountCmdUses, config.UILanguage),
	Short: config.TranslateInLang(accountCmdShorts, config.UILanguage),
}

func init() {
	AccountCmd.AddCommand(accountBalanceCmd)
	AccountCmd.AddCommand(accountCreateCmd)
	AccountCmd.AddCommand(accountCreateAddCmd)
	AccountCmd.AddCommand(accountDeleteCmd)
	AccountCmd.AddCommand(accountEthaddrCmd)
	AccountCmd.AddCommand(accountExportCmd)
	AccountCmd.AddCommand(accountExportPublicCmd)
	AccountCmd.AddCommand(accountGetVotesCmd)
	AccountCmd.AddCommand(accountImportCmd)
	AccountCmd.AddCommand(accountListCmd)
	AccountCmd.AddCommand(accountNonceCmd)
	AccountCmd.AddCommand(accountSignCmd)
	AccountCmd.AddCommand(accountUpdateCmd)
	AccountCmd.AddCommand(accountVerifyCmd)
	AccountCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(flagEndpoint, config.UILanguage))
	AccountCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure, config.TranslateInLang(flagInsecure, config.UILanguage))
}

// Sign sign message with signer
func Sign(signer, password, message string) (signedMessage string, err error) {
	pri, err := KsAccountToPrivateKey(signer, password)
	if err != nil {
		return
	}
	mes := message
	head := message[:2]
	if strings.EqualFold(head, "0x") {
		mes = message[2:]
	}
	b, err := hex.DecodeString(mes)
	if err != nil {
		return
	}
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(b))
	msg := append([]byte(prefix), b...)
	mesToSign := hash.Hash256b(msg)
	ret, err := pri.Sign(mesToSign[:])
	if err != nil {
		return
	}
	signedMessage = hex.EncodeToString(ret)
	return
}

// KsAccountToPrivateKey generates our PrivateKey interface from Keystore account
func KsAccountToPrivateKey(signer, password string) (crypto.PrivateKey, error) {
	addr, err := util.Address(signer)
	if err != nil {
		return nil, err
	}
	address, err := address.FromString(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert bytes into address")
	}
	// TODO: need a way to associate signer with private key type
	// for existing p256k1, loading from keystore (the code below)
	// for sm2, call ReadPrivateKeyFromPem()

	// find the account in keystore
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, account := range ks.Accounts() {
		if bytes.Equal(address.Bytes(), account.Address.Bytes()) {
			return crypto.KeystoreToPrivateKey(account, password)
		}
	}
	return nil, fmt.Errorf("account #%s does not match all keys in keystore", signer)
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

func newAccount(alias string, walletDir string) (string, error) {
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
	ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
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
	keyJSON, err := ioutil.ReadFile(keyStorePath)
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

func storeKey(privateKey, walletDir, password string) (string, error) {
	priKey, err := crypto.HexStringToPrivateKey(privateKey)
	if err != nil {
		return "", output.NewError(output.CryptoError, "failed to generate private key from hex string ", err)
	}
	defer priKey.Zero()

	switch sk := priKey.EcdsaPrivateKey().(type) {
	case *ecdsa.PrivateKey:
		ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
		if _, err := ks.ImportECDSA(sk, password); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to import private key into keystore ", err)
		}
	case *crypto.P256sm2PrvKey:
		if _, err := sm2.WritePrivateKeytoPem(walletDir, sk.PrivateKey, []byte(password)); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to import private key into keystore ", err)
		}
	default:
		return "", output.NewError(output.CryptoError, "invalid private key", nil)
	}

	addr, err := address.FromBytes(priKey.PublicKey().Hash())
	if err != nil {
		return "", output.NewError(output.ConvertError, "failed to convert bytes into address", err)
	}
	return addr.String(), nil
}
