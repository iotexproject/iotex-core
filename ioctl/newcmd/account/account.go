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
	"os"
	"path/filepath"
	"strings"

	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/hdwallet"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
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
	// ErrPasswdNotMatch indicates that the second input password is different from the first
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// NewAccountCmd represents the account command
func NewAccountCmd(client ioctl.Client) *cobra.Command {
	accountUses, _ := client.SelectTranslation(accountCmdUses)
	accountShorts, _ := client.SelectTranslation(accountCmdShorts)

	var endpoint string
	var insecure bool

	ac := &cobra.Command{
		Use:   accountUses,
		Short: accountShorts,
	}

	ac.AddCommand(NewAccountCreate(client))
	ac.AddCommand(NewAccountDelete(client))
	ac.AddCommand(NewAccountNonce(client))
	ac.AddCommand(NewAccountList(client))
	ac.AddCommand(NewAccountSign(client))
	ac.AddCommand(NewAccountUpdate(client))
	ac.AddCommand(NewAccountInfo(client))

	flagEndpointUsage, _ := client.SelectTranslation(flagEndpoint)
	flagInsecureUsage, _ := client.SelectTranslation(flagInsecure)

	ac.PersistentFlags().StringVar(&endpoint, "endpoint", client.Config().Endpoint, flagEndpointUsage)
	ac.PersistentFlags().BoolVar(&insecure, "insecure", !client.Config().SecureConnect, flagInsecureUsage)

	return ac
}

// Sign sign message with signer
func Sign(client ioctl.Client, signer, password, message string) (string, error) {
	pri, err := PrivateKeyFromSigner(client, signer, password)
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
func keyStoreAccountToPrivateKey(client ioctl.Client, signer, password string) (crypto.PrivateKey, error) {
	addrString, err := util.Address(signer)
	if err != nil {
		return nil, err
	}
	addr, err := address.FromString(addrString)
	if err != nil {
		return nil, fmt.Errorf("invalid account #%s, addr %s", signer, addrString)
	}

	if client.IsCryptoSm2() {
		// find the account in pem files
		pemFilePath := sm2KeyPath(client, addr)
		prvKey, err := crypto.ReadPrivateKeyFromPem(pemFilePath, password)
		if err == nil {
			return prvKey, nil
		}
	} else {
		// find the account in keystore
		ks := client.NewKeyStore()
		for _, account := range ks.Accounts() {
			if bytes.Equal(addr.Bytes(), account.Address.Bytes()) {
				return crypto.KeystoreToPrivateKey(account, password)
			}
		}
	}
	return nil, fmt.Errorf("account #%s does not match all local keys", signer)
}

// PrivateKeyFromSigner returns private key from signer
func PrivateKeyFromSigner(client ioctl.Client, signer, password string) (crypto.PrivateKey, error) {
	var (
		prvKey crypto.PrivateKey
		err    error
	)

	if !IsSignerExist(client, signer) && !util.AliasIsHdwalletKey(signer) {
		return nil, fmt.Errorf("invalid address #%s", signer)
	}

	if password == "" {
		client.PrintInfo(fmt.Sprintf("Enter password for #%s:\n", signer))
		password, err = client.ReadSecret()
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
	return keyStoreAccountToPrivateKey(client, signer, password)
}

// AccountMeta gets account metadata
func AccountMeta(client ioctl.Client, addr string) (*iotextypes.AccountMeta, error) {
	endpoint := client.Config().Endpoint
	insecure := client.Config().SecureConnect && !config.Insecure
	apiServiceClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	})
	if err != nil {
		return nil, output.NewError(output.NetworkError, "", err)
	}

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := apiServiceClient.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
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
func IsSignerExist(client ioctl.Client, signer string) bool {
	addr, err := address.FromString(signer)
	if err != nil {
		return false
	}

	if client.IsCryptoSm2() {
		// find the account in pem files
		_, err = findSm2PemFile(client, addr)
		return err == nil
	}

	// find the account in keystore
	ks := client.NewKeyStore()
	for _, ksAccount := range ks.Accounts() {
		if address.Equal(addr, ksAccount.Address) {
			return true
		}
	}

	return false
}

func newAccount(client ioctl.Client, alias string) (string, error) {
	client.PrintInfo(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := client.ReadSecret()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	client.PrintInfo(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := client.ReadSecret()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return "", output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}
	ks := client.NewKeyStore()
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

func newAccountSm2(client ioctl.Client, alias string) (string, error) {
	client.PrintInfo(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := client.ReadSecret()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	client.PrintInfo(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := client.ReadSecret()
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

	pemFilePath := sm2KeyPath(client, addr)
	if err := crypto.WritePrivateKeyToPem(pemFilePath, priKey.(*crypto.P256sm2PrvKey), password); err != nil {
		return "", output.NewError(output.KeystoreError, "failed to save private key into pem file ", err)
	}

	return addr.String(), nil
}

func newAccountByKey(client ioctl.Client, alias string, privateKey string) (string, error) {
	client.PrintInfo(fmt.Sprintf("#%s: Set password\n", alias))
	password, err := client.ReadSecret()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	client.PrintInfo(fmt.Sprintf("#%s: Enter password again\n", alias))
	passwordAgain, err := client.ReadSecret()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return "", output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}

	return storeKey(client, privateKey, password)
}

func newAccountByKeyStore(client ioctl.Client, alias, passwordOfKeyStore, keyStorePath string) (string, error) {
	privateKey, err := client.DecryptPrivateKey(passwordOfKeyStore, keyStorePath)
	if err != nil {
		return "", err
	}
	return newAccountByKey(client, alias, hex.EncodeToString(ecrypto.FromECDSA(privateKey)))
}

func newAccountByPem(client ioctl.Client, alias, passwordOfPem, pemFilePath string) (string, error) {
	prvKey, err := crypto.ReadPrivateKeyFromPem(pemFilePath, passwordOfPem)
	if err != nil {
		return "", output.NewError(output.CryptoError, "failed to read private key from pem file", err)
	}

	return newAccountByKey(client, alias, prvKey.HexString())
}

func storeKey(client ioctl.Client, privateKey, password string) (string, error) {
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
		ks := client.NewKeyStore()
		if _, err := ks.ImportECDSA(sk, password); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to import private key into keystore ", err)
		}
	case *crypto.P256sm2PrvKey:
		pemFilePath := sm2KeyPath(client, addr)
		if err := crypto.WritePrivateKeyToPem(pemFilePath, sk, password); err != nil {
			return "", output.NewError(output.KeystoreError, "failed to save private key into pem file ", err)
		}
	default:
		return "", output.NewError(output.CryptoError, "invalid private key", nil)
	}

	return addr.String(), nil
}

func sm2KeyPath(client ioctl.Client, addr address.Address) string {
	return filepath.Join(client.Config().Wallet, "sm2sk-"+addr.String()+".pem")
}

func findSm2PemFile(client ioctl.Client, addr address.Address) (string, error) {
	filePath := sm2KeyPath(client, addr)
	_, err := os.Stat(filePath)
	if err != nil {
		return "", output.NewError(output.ReadFileError, "crypto file not found", err)
	}
	return filePath, nil
}

func listSm2Account(client ioctl.Client) ([]string, error) {
	sm2Accounts := make([]string, 0)
	files, err := os.ReadDir(client.Config().Wallet)
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
