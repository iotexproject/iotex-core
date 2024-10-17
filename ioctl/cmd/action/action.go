// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// Multi-language support
var (
	_actionCmdShorts = map[config.Language]string{
		config.English: "Manage actions of IoTeX blockchain",
		config.Chinese: "管理IoTex区块链的行为", // this translation
	}
	_flagActionEndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点", // this translation
	}
	_flagActionInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接", // this translation
	}
)

const _defaultGasLimit = uint64(20000000)

// Flags
var (
	_gasLimitFlag = flag.NewUint64VarP("gas-limit", "l", _defaultGasLimit, "set gas limit")
	_gasPriceFlag = flag.NewStringVarP("gas-price", "p", "1", "set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is \"0\"")
	_nonceFlag    = flag.NewUint64VarP("nonce", "n", 0, "set nonce (default using pending nonce)")
	_signerFlag   = flag.NewStringVarP("signer", "s", "", "choose a signing account")
	_bytecodeFlag = flag.NewStringVarP("bytecode", "b", "", "set the byte code")
	_yesFlag      = flag.BoolVarP("assume-yes", "y", false, "answer yes for all confirmations")
)

// ActionCmd represents the action command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: config.TranslateInLang(_actionCmdShorts, config.UILanguage),
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
	ActionCmd.AddCommand(_actionHashCmd)
	ActionCmd.AddCommand(_actionTransferCmd)
	ActionCmd.AddCommand(_actionDeployCmd)
	ActionCmd.AddCommand(_actionInvokeCmd)
	ActionCmd.AddCommand(_actionReadCmd)
	ActionCmd.AddCommand(_actionClaimCmd)
	ActionCmd.AddCommand(_actionDepositCmd)
	ActionCmd.AddCommand(_actionSendRawCmd)
	ActionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagActionEndPointUsages,
			config.UILanguage))
	ActionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagActionInsecureUsages, config.UILanguage))
}

func decodeBytecode() ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(_bytecodeFlag.Value().(string)))
}

// Signer returns signer's address
func Signer() (address string, err error) {
	addressOrAlias := _signerFlag.Value().(string)
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
	nonce := _nonceFlag.Value().(uint64)
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
	_gasLimitFlag.RegisterCommand(cmd)
	_gasPriceFlag.RegisterCommand(cmd)
	_signerFlag.RegisterCommand(cmd)
	_nonceFlag.RegisterCommand(cmd)
	_yesFlag.RegisterCommand(cmd)
	account.RegisterPasswordFlag(cmd)
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau() (*big.Int, error) {
	if account.CryptoSm2 {
		return big.NewInt(0), nil
	}
	gasPrice := _gasPriceFlag.Value().(string)
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

func fixGasLimit(caller string, execution *action.Execution) (uint64, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return 0, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
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
			return 0, output.NewError(output.APIError, sta.Message(), nil)
		}
		return 0, output.NewError(output.NetworkError,
			"failed to invoke EstimateActionGasConsumption api", err)
	}
	return res.Gas, nil
}

// SendRaw sends raw action to blockchain
func SendRaw(selp *iotextypes.Action) error {
	_, err := SendRawAndRespond(selp)
	if err != nil {
		return err
	}

	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	txhash := hex.EncodeToString(shash[:])
	outputActionInfo(txhash)
	return nil
}

// SendRawAndRespond sends raw action to blockchain with response and error return
func SendRawAndRespond(selp *iotextypes.Action) (*iotexapi.SendActionResponse, error) {
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

	request := &iotexapi.SendActionRequest{Action: selp}
	response, err := cli.SendAction(ctx, request)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke SendAction api", err)
	}
	return response, nil
}

// SendAction sends signed action to blockchain
func SendAction(elp action.Envelope, signer string) error {
	resp, err := SendActionAndResponse(elp, signer)
	if err != nil {
		return err
	}
	outputActionInfo(resp.ActionHash)
	return nil
}

// SendActionAndResponse sends signed action to blockchain with response and error return
func SendActionAndResponse(elp action.Envelope, signer string) (*iotexapi.SendActionResponse, error) {
	prvKey, err := account.PrivateKeyFromSigner(signer, account.PasswordByFlag())
	if err != nil {
		return nil, err
	}

	chainMeta, err := bc.GetChainMeta()
	if err != nil {
		return nil, output.NewError(0, "failed to get chain meta", err)
	}
	elp.SetChainID(chainMeta.GetChainID())

	if util.AliasIsHdwalletKey(signer) {
		addr := prvKey.PublicKey().Address()
		signer = addr.String()
		nonce, err := nonce(signer)
		if err != nil {
			return nil, output.NewError(0, "failed to get nonce ", err)
		}
		elp.SetNonce(nonce)
	}

	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		return nil, output.NewError(output.CryptoError, "failed to sign action", err)
	}
	if err := isBalanceEnough(signer, sealed); err != nil {
		return nil, output.NewError(0, "failed to pass balance check", err) // TODO: undefined error
	}

	selp := sealed.Proto()
	actionInfo, err := printActionProto(selp)
	if err != nil {
		return nil, output.NewError(0, "failed to print action proto message", err)
	}

	if _yesFlag.Value() == false {
		var confirm string
		info := fmt.Sprintln(actionInfo + "\nPlease confirm your action.\n")
		message := output.ConfirmationMessage{Info: info, Options: []string{"yes"}}
		fmt.Println(message.String())

		if _, err := fmt.Scanf("%s", &confirm); err != nil {
			return nil, output.NewError(output.InputError, "failed to input yes", err)
		}
		if !strings.EqualFold(confirm, "yes") {
			output.PrintResult("quit")
			return nil, nil
		}
	}

	return SendRawAndRespond(selp)
}

// Execute sends signed execution transaction to blockchain
func Execute(contract string, amount *big.Int, bytecode []byte) error {
	resp, err := ExecuteAndResponse(contract, amount, bytecode)
	if err != nil {
		return err
	}
	outputActionInfo(resp.ActionHash)
	return nil
}

// ExecuteAndResponse sends signed execution transaction to blockchain and with response and error return
func ExecuteAndResponse(contract string, amount *big.Int, bytecode []byte) (*iotexapi.SendActionResponse, error) {
	if len(contract) == 0 && len(bytecode) == 0 {
		return nil, output.NewError(output.InputError, "failed to deploy contract with empty bytecode", nil)
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return nil, output.NewError(0, "failed to get gas price", err)
	}
	signer, err := Signer()
	if err != nil {
		return nil, output.NewError(output.AddressError, "failed to get signer address", err)
	}
	nonce, err := nonce(signer)
	if err != nil {
		return nil, output.NewError(0, "failed to get nonce", err)
	}
	gasLimit := _gasLimitFlag.Value().(uint64)
	tx := action.NewExecution(contract, amount, bytecode)
	if gasLimit == 0 {
		gasLimit, err = fixGasLimit(signer, tx)
		if err != nil {
			return nil, output.NewError(0, "failed to fix Execution gaslimit", err)
		}
	}
	return SendActionAndResponse(
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
			GasLimit:      _gasLimitFlag.Value().(uint64),
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

func isBalanceEnough(address string, act *action.SealedEnvelope) error {
	accountMeta, err := account.GetAccountMeta(address)
	if err != nil {
		return output.NewError(0, "failed to get account meta", err)
	}
	balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
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

func outputActionInfo(txhash string) {
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
}
