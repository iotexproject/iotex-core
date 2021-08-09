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

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/hdwallet"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Multi-language support
var (
	actionCmdShorts = map[config.Language]string{
		config.English: "Manage actions of IoTeX blockchain",
		config.Chinese: "管理IoTex区块链的行为", // this translation
	}
	actionCmdUses = map[config.Language]string{
		config.English: "action",
		config.Chinese: "action 行为", // this translation
	}
	flagActionEndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点", // this translation
	}
	flagActionInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接", // this translation
	}
)

const defaultGasLimit = uint64(20000000)

// var defaultGasPrice = big.NewInt(unit.Qev)

// Flags
var (
	gasLimitFlag = flag.NewUint64VarP("gas-limit", "l", defaultGasLimit, "set gas limit")
	gasPriceFlag = flag.NewStringVarP("gas-price", "p", "1", "set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is \"0\"")
	nonceFlag    = flag.NewUint64VarP("nonce", "n", 0, "set nonce (default using pending nonce)")
	signerFlag   = flag.NewStringVarP("signer", "s", "", "choose a signing account")
	bytecodeFlag = flag.NewStringVarP("bytecode", "b", "", "set the byte code")
	yesFlag      = flag.BoolVarP("assume-yes", "y", false, "answer yes for all confirmations")
	passwordFlag = flag.NewStringVarP("password", "P", "", "input password for account")
)

// ActionCmd represents the action command
var ActionCmd = &cobra.Command{
	Use:   config.TranslateInLang(actionCmdUses, config.UILanguage),
	Short: config.TranslateInLang(actionCmdShorts, config.UILanguage),
}

type sendMessage struct {
	Info   string `json:"info"`
	TxHash string `json:"txHash"`
	URL    string `json:"url"`
}

func (m *sendMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s\nWait for several seconds and query this action by hash: %s", m.Info, m.URL)
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
		config.ReadConfig.Endpoint, config.TranslateInLang(flagActionEndPointUsages,
			config.UILanguage))
	ActionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(flagActionInsecureUsages, config.UILanguage))
}

func decodeBytecode() ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(bytecodeFlag.Value().(string)))
}

// Signer returns signer's address
func Signer() (address string, err error) {
	addressOrAlias := signerFlag.Value().(string)
	if util.AliasIsHdwalletKey(addressOrAlias) {
		return addressOrAlias, nil
	}

	if addressOrAlias == "" {
		addressOrAlias, err = config.GetContextAddressOrAlias()
		if err != nil {
			return
		}
	}
	return util.GetAddress(addressOrAlias)
}

func nonce(executor string) (uint64, error) {
	if util.AliasIsHdwalletKey(executor) {
		// for hdwallet key, get the nonce in SendAction()
		return 0, nil
	}
	nonce := nonceFlag.Value().(uint64)
	if nonce != 0 {
		return nonce, nil
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return 0, output.NewError(0, "failed to get account meta", err)
	}
	return accountMeta.PendingNonce, nil
}

// RegisterWriteCommand registers action flags for command
func RegisterWriteCommand(cmd *cobra.Command) {
	gasLimitFlag.RegisterCommand(cmd)
	gasPriceFlag.RegisterCommand(cmd)
	signerFlag.RegisterCommand(cmd)
	nonceFlag.RegisterCommand(cmd)
	yesFlag.RegisterCommand(cmd)
	passwordFlag.RegisterCommand(cmd)
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau() (*big.Int, error) {
	if account.CryptoSm2 {
		return big.NewInt(0), nil
	}
	gasPrice := gasPriceFlag.Value().(string)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.SuggestGasPriceRequest{}
	response, err := cli.SuggestGasPrice(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke SuggestGasPrice api", err)
	}
	return new(big.Int).SetUint64(response.GasPrice), nil
}

func fixGasLimit(caller string, execution *action.Execution) (*action.Execution, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: caller,
	}

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	res, err := cli.EstimateActionGasConsumption(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError,
			"failed to invoke EstimateActionGasConsumption api", err)
	}
	return action.NewExecution(execution.Contract(), execution.Nonce(), execution.Amount(), res.Gas, execution.GasPrice(), execution.Data())
}

// SendRaw sends raw action to blockchain
func SendRaw(selp *iotextypes.Action) error {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.SendActionRequest{Action: selp}
	if _, err = cli.SendAction(ctx, request); err != nil {
		if sta, ok := status.FromError(err); ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke SendAction api", err)
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	txhash := hex.EncodeToString(shash[:])
	message := sendMessage{Info: "Action has been sent to blockchain.", TxHash: txhash, URL: "https://"}
	switch config.ReadConfig.Explorer {
	case "iotexscan":
		if strings.Contains(config.ReadConfig.Endpoint, "testnet") {
			message.URL += "testnet."
		}
		message.URL += "iotexscan.io/action/" + txhash
	case "iotxplorer":
		message.URL = "iotxplorer.io/actions/" + txhash
	default:
		message.URL = config.ReadConfig.Explorer + txhash
	}
	fmt.Println(message.String())
	return nil
}

// PrivateKeyFromSigner returns private key from signer
func PrivateKeyFromSigner(signer string) (crypto.PrivateKey, error) {
	var prvKey crypto.PrivateKey
	var err error

	if account.IsSignerExist(signer) || util.AliasIsHdwalletKey(signer) {
		// Get signer's password
		password := passwordFlag.Value().(string)
		if password == "" {
			output.PrintQuery(fmt.Sprintf("Enter password #%s:\n", signer))
			password, err = util.ReadSecretFromStdin()
			if err != nil {
				return nil, output.NewError(output.InputError, "failed to get password", err)
			}
		}

		if util.AliasIsHdwalletKey(signer) {
			account, change, index, err := util.ParseHdwPath(signer)
			if err != nil {
				return nil, output.NewError(output.InputError, "invalid hdwallet key format", err)
			}
			_, prvKey, err = hdwallet.DeriveKey(account, change, index, password)
			if err != nil {
				return nil, output.NewError(output.InputError, "failed to derive key from HDWallet", err)
			}
		} else {
			prvKey, err = account.LocalAccountToPrivateKey(signer, password)
			if err != nil {
				return nil, output.NewError(output.KeystoreError, "failed to get private key from keystore", err)
			}
		}
		return prvKey, nil
	}

	// Get private key
	output.PrintQuery(fmt.Sprintf("Enter private key #%s:", signer))
	prvKeyString, err := util.ReadSecretFromStdin()
	if err != nil {
		return nil, output.NewError(output.InputError, "failed to get private key", err)
	}

	prvKey, err = crypto.HexStringToPrivateKey(prvKeyString)
	if err != nil {
		return nil, output.NewError(output.InputError, "failed to create private key from HexString input", err)
	}
	return prvKey, nil
}

// SendAction sends signed action to blockchain
func SendAction(elp action.Envelope, signer string) error {
	prvKey, err := PrivateKeyFromSigner(signer)
	if err != nil {
		return err
	}

	if util.AliasIsHdwalletKey(signer) {
		addr, _ := address.FromBytes(prvKey.PublicKey().Hash())
		signer = addr.String()
		nonce, err := nonce(signer)
		if err != nil {
			return output.NewError(0, "failed to get nonce ", err)
		}
		elp.SetNonce(nonce)
	}

	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		return output.NewError(output.CryptoError, "failed to sign action", err)
	}
	if err := isBalanceEnough(signer, sealed); err != nil {
		return output.NewError(0, "failed to pass balance check", err) // TODO: undefined error
	}

	selp := sealed.Proto()
	actionInfo, err := printActionProto(selp)
	if err != nil {
		return output.NewError(0, "failed to print action proto message", err)
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

	return SendRaw(selp)
}

// Execute sends signed execution transaction to blockchain
func Execute(contract string, amount *big.Int, bytecode []byte) error {
	if len(contract) == 0 && len(bytecode) == 0 {
		return output.NewError(output.InputError, "failed to deploy contract with empty bytecode", nil)
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	signer, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	nonce, err := nonce(signer)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	gasLimit := gasLimitFlag.Value().(uint64)
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil || tx == nil {
		return output.NewError(output.InstantiationError, "failed to make a Execution instance", err)
	}
	if gasLimit == 0 {
		tx, err = fixGasLimit(signer, tx)
		if err != nil || tx == nil {
			return output.NewError(0, "failed to fix Execution gaslimit", err)
		}
		gasLimit = tx.GasLimit()
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(tx).Build(),
		signer,
	)
}

// Read reads smart contract on IoTeX blockchain
func Read(contract address.Address, amount string, bytecode []byte) (string, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	callerAddr, _ := Signer()
	if callerAddr == "" {
		callerAddr = address.ZeroAddress
	}
	res, err := iotexapi.NewAPIServiceClient(conn).ReadContract(
		ctx,
		&iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Amount:   amount,
				Contract: contract.String(),
				Data:     bytecode,
			},
			CallerAddress: callerAddr,
			GasLimit:      gasLimitFlag.Value().(uint64),
		},
	)
	if err == nil {
		return res.Data, nil
	}
	if sta, ok := status.FromError(err); ok {
		return "", output.NewError(output.APIError, sta.Message(), nil)
	}
	return "", output.NewError(output.NetworkError, "failed to invoke ReadContract api", err)
}

func isBalanceEnough(address string, act action.SealedEnvelope) error {
	accountMeta, err := account.GetAccountMeta(address)
	if err != nil {
		return output.NewError(0, "failed to get account meta", err)
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert balance into big int", nil)
	}
	cost, err := act.Cost()
	if err != nil {
		return output.NewError(output.RuntimeError, "failed to check cost of an action", nil)
	}
	if balance.Cmp(cost) < 0 {
		return output.NewError(output.ValidationError, "balance is not enough", nil)
	}
	return nil
}
