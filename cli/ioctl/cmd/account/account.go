// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"context"
	"fmt"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage accounts of IoTeX blockchain",
	Args:  cobra.MinimumNArgs(1),
}

func init() {
	AccountCmd.AddCommand(accountBalanceCmd)
	AccountCmd.AddCommand(accountCreateCmd)
	AccountCmd.AddCommand(accountCreateAddCmd)
	AccountCmd.AddCommand(accountDeleteCmd)
	AccountCmd.AddCommand(accountImportCmd)
	AccountCmd.AddCommand(accountListCmd)
	AccountCmd.AddCommand(accountNonceCmd)
	AccountCmd.AddCommand(accountUpdateCmd)
}

// KsAccountToPrivateKey generates our PrivateKey interface from Keystore account
func KsAccountToPrivateKey(signer, password string) (keypair.PrivateKey, error) {
	addr, err := alias.Address(signer)
	if err != nil {
		return nil, err
	}
	address, err := address.FromString(addr)
	if err != nil {
		log.L().Error("failed to convert string into address", zap.Error(err))
		return nil, err
	}
	// find the account in keystore
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, account := range ks.Accounts() {
		if bytes.Equal(address.Bytes(), account.Address.Bytes()) {
			return keypair.KeystoreToPrivateKey(account, password)
		}
	}
	return nil, fmt.Errorf("account #%s does not match all keys in keystore", signer)
}

// GetAccountMeta gets account metadata
func GetAccountMeta(addr string) (*iotextypes.AccountMeta, error) {
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := iotexapi.GetAccountRequest{Address: addr}
	response, err := cli.GetAccount(ctx, &request)
	if err != nil {
		return nil, err
	}
	return response.AccountMeta, nil
}

func newAccount(alias string, walletDir string) (string, error) {
	fmt.Printf("#%s: Set password\n", alias)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	password := string(bytePassword)
	fmt.Printf("#%s: Enter password again\n", alias)
	bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	if password != string(bytePassword) {
		return "", ErrPasswdNotMatch
	}
	ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", err
	}
	addr, err := address.FromBytes(account.Address.Bytes())
	if err != nil {
		log.L().Error("failed to convert bytes into address", zap.Error(err))
		return "", err
	}
	return addr.String(), nil
}

func newAccountByKey(alias string, privateKey string, walletDir string) (string, error) {
	fmt.Printf("#%s: Set password\n", alias)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	password := string(bytePassword)
	fmt.Printf("#%s: Enter password again\n", alias)
	bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	if password != string(bytePassword) {
		return "", ErrPasswdNotMatch
	}
	ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
	priKey, err := keypair.HexStringToPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	defer priKey.Zero()
	account, err := ks.ImportECDSA(priKey.EcdsaPrivateKey(), password)
	priKey.Zero()
	if err != nil {
		return "", err
	}
	addr, err := address.FromBytes(account.Address.Bytes())
	if err != nil {
		log.L().Error("failed to convert bytes into address", zap.Error(err))
		return "", err
	}
	return addr.String(), nil
}
