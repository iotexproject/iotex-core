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
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   "account",
	Short: "Deal with accounts of IoTeX blockchain",
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
}

// Sign use the password to unlock key associated with name, and signs the hash
func Sign(signer, password string, hash []byte) ([]byte, error) {
	addr, err := Address(signer)
	if err != nil {
		return nil, err
	}
	address, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	// find the key in keystore and sign
	ks := keystore.NewKeyStore(config.Get("wallet"), keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(address.Bytes(), v.Address.Bytes()) {
			return ks.SignHashWithPassphrase(v, password, hash)
		}
	}
	return nil, errors.Errorf("account %s's address does not match with keys in keystore", signer)
}

// Address returns the address corresponding to name
func Address(in string) (string, error) {
	if len(in) >= validator.IoAddrLen {
		if err := validator.ValidateAddress(in); err != nil {
			return "", err
		}
		return in, nil

	}
	config, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	addr, ok := config.AccountList[in]
	if !ok {
		return "", errors.Errorf("can't find account from #" + in)
	}
	return addr, nil
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

func newAccount(name string, walletDir string) (string, error) {
	fmt.Printf("#%s: Set password\n", name)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	password := string(bytePassword)
	fmt.Printf("#%s: Enter password again\n", name)
	bytePassword, err = terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	if password != string(bytePassword) {
		return "", errors.New("password doesn't match")
	}
	ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", err
	}
	addr, err := address.FromBytes(account.Address.Bytes())
	if err != nil {
		log.L().Error(err.Error(), zap.Error(err))
		return "", err
	}
	return addr.String(), nil
}

func newAccountByKey(name string, privateKey string, walletDir string) (string, error) {
	fmt.Printf("#%s: Set password\n", name)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	password := string(bytePassword)
	fmt.Printf("#%s: Enter password again\n", name)
	bytePassword, err = terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	if password != string(bytePassword) {
		return "", errors.New("password doesn't match")
	}
	ks := keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)
	priKey, err := keypair.HexStringToPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	account, err := ks.ImportECDSA(priKey.EcdsaPrivateKey(), password)
	if err != nil {
		return "", err
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return addr.String(), nil
}
