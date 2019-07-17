// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Flags
var (
	gasLimitFlag = flag.NewUint64VarP("gas-limit", "l", 0, "set gas limit")
	gasPriceFlag = flag.NewStringVarP("gas-price", "p", "1", "set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is \"0\"")
	nonceFlag    = flag.NewUint64VarP("nonce", "n", 0, "set nonce (default using pending nonce)")
	signerFlag   = flag.NewStringVarP("signer", "s", "", "choose a signing account")
	bytecodeFlag = flag.NewStringVarP("bytecode", "b", "", "set the byte code")
	yesFlag      = flag.BoolVarP("assume-yes", "y", false, " answer yes for all confirmations")
	passwordFlag = flag.NewStringVarP("password", "P", "", "input password for account")
)

// ActionCmd represents the action command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage actions of IoTeX blockchain",
}

type sendMessage struct {
	Info   string `json:"info"`
	TxHash string `json:"txHash"`
	URL    string `json:"url"`
}

func (m *sendMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s\nWait for several seconds and query this action by hash:%s", m.Info, m.URL)
	}
	return output.FormatString(output.Result, m)
}

func init() {
	ActionCmd.AddCommand(actionHashCmd)
	ActionCmd.AddCommand(actionTransferCmd)
	ActionCmd.AddCommand(actionDeployCmd)
	ActionCmd.AddCommand(actionInvokeCmd)
	ActionCmd.AddCommand(actionReadCmd)
	ActionCmd.AddCommand(actionClaimCmd)
	ActionCmd.AddCommand(actionDepositCmd)
	ActionCmd.AddCommand(actionSendRawCmd)
	ActionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	ActionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
}

func decodeBytecode() ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(bytecodeFlag.Value().(string), "0x"))
}

func signer() (address string, err error) {
	return util.GetAddress(signerFlag.Value().(string))
}

func nonce(executor string) (uint64, error) {
	nonce := nonceFlag.Value().(uint64)
	if nonce != 0 {
		return nonce, nil
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return 0, err
	}
	return accountMeta.PendingNonce, nil
}

func registerWriteCommand(cmd *cobra.Command) {
	gasLimitFlag.RegisterCommand(cmd)
	gasPriceFlag.RegisterCommand(cmd)
	signerFlag.RegisterCommand(cmd)
	nonceFlag.RegisterCommand(cmd)
	yesFlag.RegisterCommand(cmd)
	passwordFlag.RegisterCommand(cmd)
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau() (*big.Int, error) {
	gasPrice := gasPriceFlag.Value().(string)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.SuggestGasPriceRequest{}
	response, err := cli.SuggestGasPrice(ctx, request)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetUint64(response.GasPrice), nil
}

func fixGasLimit(signer string, execution *action.Execution) (*action.Execution, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	caller, err := address.FromString(signer)
	if err != nil {
		return nil, err
	}
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: caller.String(),
	}
	res, err := cli.EstimateActionGasConsumption(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("error when invoke EstimateActionGasConsumption api")
	}
	return action.NewExecution(execution.Contract(), execution.Nonce(), execution.Amount(), res.Gas, execution.GasPrice(), execution.Data())
}

// execute must be used at the end of a command
func execute(contract string, amount *big.Int, bytecode []byte) error {
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.PrintError(output.ConvertError, err.Error())
	}
	signer, err := signer()
	if err != nil {
		return output.PrintError(output.AddressError, err.Error())
	}
	nonce, err := nonce(signer)
	if err != nil {
		return output.PrintError(0, err.Error()) // TODO: undefined error
	}
	gasLimit := gasLimitFlag.Value().(uint64)
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil || tx == nil {
		return output.PrintError(0, "cannot make a Execution instance"+err.Error()) // TODO: undefined error
	}
	if gasLimit == 0 {
		tx, err = fixGasLimit(signer, tx)
		if err != nil || tx == nil {
			return output.PrintError(0, "cannot fix Execution gaslimit"+err.Error()) // TODO: undefined error
		}
		gasLimit = tx.GasLimit()
	}
	return sendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(tx).Build(),
		signer,
	)
}

// sendRaw must be used at the end of a command
func sendRaw(selp *iotextypes.Action) error {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.PrintError(output.NetworkError, err.Error())
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	request := &iotexapi.SendActionRequest{Action: selp}
	if _, err = cli.SendAction(ctx, request); err != nil {
		if sta, ok := status.FromError(err); ok {
			return output.PrintError(output.APIError, sta.Message())
		}
		return output.PrintError(output.NetworkError, err.Error())
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	txhash := hex.EncodeToString(shash[:])
	message := sendMessage{Info: "Action has been sent to blockchain.", TxHash: txhash}
	switch config.ReadConfig.Explorer {
	case "iotexscan":
		message.URL = "iotexscan.io/action/" + txhash
	case "iotxplorer":
		message.URL = "iotxplorer.io/actions/" + txhash
	default:
		message.URL = config.ReadConfig.Explorer + txhash
	}
	fmt.Println(message.String())
	return nil
}

// sendAction must be used at the end of a command
func sendAction(elp action.Envelope, signer string) error {
	var (
		prvKey           crypto.PrivateKey
		err              error
		prvKeyOrPassword string
	)
	if !signerIsExist(signer) {
		output.PrintQuery(fmt.Sprintf("Enter private key #%s:", signer))
		prvKeyOrPassword, err = util.ReadSecretFromStdin()
		if err != nil {
			return output.PrintError(output.InputError, err.Error())
		}
		prvKey, err = crypto.HexStringToPrivateKey(prvKeyOrPassword)
	} else if passwordFlag.Value() == "" {
		output.PrintQuery(fmt.Sprintf("Enter password #%s:\n", signer))
		prvKeyOrPassword, err = util.ReadSecretFromStdin()
		if err != nil {
			return output.PrintError(output.InputError, err.Error())
		}
	} else {
		prvKeyOrPassword = passwordFlag.Value().(string)
	}
	prvKey, err = account.KsAccountToPrivateKey(signer, prvKeyOrPassword)
	if err != nil {
		return output.PrintError(output.KeystoreError, err.Error())
	}
	defer prvKey.Zero()
	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		return output.PrintError(output.CryptoError, "failed to sign action"+err.Error())
	}
	if err := isBalanceEnough(signer, sealed); err != nil {
		return output.PrintError(0, err.Error()) // TODO: undefined error
	}
	selp := sealed.Proto()

	actionInfo, err := printActionProto(selp)
	if err != nil {
		return output.PrintError(output.ConvertError, err.Error())
	}
	if yesFlag.Value() == false {
		var confirm string
		info := fmt.Sprintln(actionInfo + "\nPlease confirm your action.\n")
		message := output.ConfirmationMessage{Info: info, Options: []string{"yes"}}
		fmt.Println(message.String())
		fmt.Scanf("%s", &confirm)
		if !strings.EqualFold(confirm, "yes") {
			output.PrintResult("quit")
			return nil
		}
	}
	return sendRaw(selp)
}

func isBalanceEnough(address string, act action.SealedEnvelope) (err error) {
	accountMeta, err := account.GetAccountMeta(address)
	if err != nil {
		return
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		err = fmt.Errorf("failed to convert balance into big int")
		return
	}
	cost, err := act.Cost()
	if balance.Cmp(cost) < 0 {
		err = fmt.Errorf("balance is not enough")
		return
	}
	return
}

func signerIsExist(signer string) bool {
	addr, err := address.FromString(signer)
	if err != nil {
		return false
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
