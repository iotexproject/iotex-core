// Copyright (c) 2022 IoTeX Foundation
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
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Multi-language support
var (
	_actionCmdShorts = map[config.Language]string{
		config.English: "Manage actions of IoTeX blockchain",
		config.Chinese: "管理IoTex区块链的行为", // this translation
	}
	_actionCmdUses = map[config.Language]string{
		config.English: "action",
		config.Chinese: "action 行为", // this translation
	}
	_flagActionEndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点", // this translation
	}
	_flagActionInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接", // this translation
	}
	_flagGasLimitUsages = map[config.Language]string{
		config.English: "set gas limit",
	}
	_flagGasPriceUsages = map[config.Language]string{
		config.English: `set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is "0"`,
	}
	_flagNonceUsages = map[config.Language]string{
		config.English: "set nonce (default using pending nonce)",
	}
	_flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
	}
	_flagBytecodeUsages = map[config.Language]string{
		config.English: "set the byte code",
	}
	_flagAssumeYesUsages = map[config.Language]string{
		config.English: "answer yes for all confirmations",
	}
	_flagPasswordUsages = map[config.Language]string{
		config.English: "input password for account",
	}
)

// Flag label, short label and defaults
const (
	gasLimitFlagLabel       = "gas-limit"
	gasLimitFlagShortLabel  = "l"
	gasLimitFlagDefault     = uint64(20000000)
	gasPriceFlagLabel       = "gas-price"
	gasPriceFlagShortLabel  = "p"
	gasPriceFlagDefault     = "1"
	nonceFlagLabel          = "nonce"
	nonceFlagShortLabel     = "n"
	nonceFlagDefault        = uint64(0)
	signerFlagLabel         = "signer"
	signerFlagShortLabel    = "s"
	signerFlagDefault       = ""
	bytecodeFlagLabel       = "bytecode"
	bytecodeFlagShortLabel  = "b"
	bytecodeFlagDefault     = ""
	assumeYesFlagLabel      = "assume-yes"
	assumeYesFlagShortLabel = "y"
	assumeYesFlagDefault    = false
	passwordFlagLabel       = "password"
	passwordFlagShortLabel  = "P"
	passwordFlagDefault     = ""
)

func registerGasLimitFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagGasLimitUsages)
	flag.NewUint64VarP(gasLimitFlagLabel, gasLimitFlagShortLabel, gasLimitFlagDefault, usage).RegisterCommand(cmd)
}

func registerGasPriceFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagGasPriceUsages)
	flag.NewStringVarP(gasPriceFlagLabel, gasPriceFlagShortLabel, gasPriceFlagDefault, usage).RegisterCommand(cmd)
}

func registerNonceFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagNonceUsages)
	flag.NewUint64VarP(nonceFlagLabel, nonceFlagShortLabel, nonceFlagDefault, usage).RegisterCommand(cmd)
}

func registerSignerFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagSignerUsages)
	flag.NewStringVarP(signerFlagLabel, signerFlagShortLabel, signerFlagDefault, usage).RegisterCommand(cmd)
}

func registerBytecodeFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagBytecodeUsages)
	flag.NewStringVarP(bytecodeFlagLabel, bytecodeFlagShortLabel, bytecodeFlagDefault, usage).RegisterCommand(cmd)
}

func registerAssumeYesFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagAssumeYesUsages)
	flag.BoolVarP(assumeYesFlagLabel, assumeYesFlagShortLabel, assumeYesFlagDefault, usage).RegisterCommand(cmd)
}

func registerPasswordFlag(client ioctl.Client, cmd *cobra.Command) {
	usage, _ := client.SelectTranslation(_flagPasswordUsages)
	flag.NewStringVarP(passwordFlagLabel, passwordFlagShortLabel, passwordFlagDefault, usage).RegisterCommand(cmd)
}

func mustString(v string, err error) string {
	if err != nil {
		panic(err)
	}
	return v
}

func mustUint64(v uint64, err error) uint64 {
	if err != nil {
		panic(err)
	}
	return v
}

func mustBoolean(v bool, err error) bool {
	if err != nil {
		panic(err)
	}
	return v
}

func getGasLimitFlagValue(cmd *cobra.Command) (v uint64) {
	return mustUint64(cmd.Flags().GetUint64(gasLimitFlagLabel))
}

func getGasPriceFlagValue(cmd *cobra.Command) (v string) {
	return mustString(cmd.Flags().GetString(gasPriceFlagLabel))
}

func getNonceFlagValue(cmd *cobra.Command) (v uint64) {
	return mustUint64(cmd.Flags().GetUint64(nonceFlagLabel))
}

func getSignerFlagValue(cmd *cobra.Command) (v string) {
	return mustString(cmd.Flags().GetString(signerFlagLabel))
}

func getBytecodeFlagValue(cmd *cobra.Command) (v string) {
	return mustString(cmd.Flags().GetString(bytecodeFlagLabel))
}

func getDecodeBytecode(cmd *cobra.Command) ([]byte, error) {
	bytecode, err := cmd.Flags().GetString(bytecodeFlagLabel)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(util.TrimHexPrefix(bytecode))
}

func getAssumeYesFlagValue(cmd *cobra.Command) (v bool) {
	return mustBoolean(cmd.Flags().GetBool(assumeYesFlagLabel))
}

func getPasswordFlagValue(cmd *cobra.Command) (v string) {
	return mustString(cmd.Flags().GetString(passwordFlagLabel))
}

// NewAction represents the action command
func NewAction(client ioctl.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   client.SelectTranslationText(_actionCmdUses),
		Short: client.SelectTranslationText(_actionCmdShorts),
	}

	// TODO add sub commands
	// cmd.AddCommand(NewActionHash(client))
	// cmd.AddCommand(NewActionTransfer(client))
	// cmd.AddCommand(NewActionDeploy(client))
	// cmd.AddCommand(NewActionInvoke(client))
	// cmd.AddCommand(NewActionRead(client))
	// cmd.AddCommand(NewActionClaim(client))
	// cmd.AddCommand(NewActionDeposit(client))
	// cmd.AddCommand(NewActionSendRaw(client))

	var (
		insecure bool
		endpoint string
	)
	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", client.Config().Endpoint, client.SelectTranslationText(_flagActionEndPointUsages))
	cmd.PersistentFlags().BoolVar(&insecure, "insecure", !client.Config().SecureConnect, client.SelectTranslationText(_flagActionInsecureUsages))

	return cmd
}

// RegisterWriteCommand registers action flags for command
func RegisterWriteCommand(client ioctl.Client, cmd *cobra.Command) {
	registerGasLimitFlag(client, cmd)
	registerGasPriceFlag(client, cmd)
	registerSignerFlag(client, cmd)
	registerNonceFlag(client, cmd)
	registerAssumeYesFlag(client, cmd)
	registerPasswordFlag(client, cmd)
}

func handleClientRequestError(err error, apiName string) error {
	sta, ok := status.FromError(err)
	if ok {
		return errors.New(sta.Message())
	}
	return errors.Wrapf(err, "failed to invoke %s api", apiName)
}

// Signer returns signer's address
func Signer(cmd *cobra.Command) (address string, err error) {
	addressOrAlias := getSignerFlagValue(cmd)
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

func nonce(client ioctl.Client, cmd *cobra.Command, executor string) (uint64, error) {
	if util.AliasIsHdwalletKey(executor) {
		// for hdwallet key, get the nonce in SendAction()
		return 0, nil
	}
	if nonce := getNonceFlagValue(cmd); nonce != 0 {
		return nonce, nil
	}
	accountMeta, err := account.Meta(client, executor)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get account meta")
	}
	return accountMeta.PendingNonce, nil
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau(client ioctl.Client, cmd *cobra.Command) (*big.Int, error) {
	if client.IsCryptoSm2() {
		return big.NewInt(0), nil
	}
	gasPrice := getGasPriceFlagValue(cmd)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}

	cli, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: client.Config().Endpoint,
		Insecure: client.Config().SecureConnect && !config.Insecure,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to endpoint")
	}

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	rsp, err := cli.SuggestGasPrice(ctx, &iotexapi.SuggestGasPriceRequest{})
	if err != nil {
		return nil, handleClientRequestError(err, "SuggestGasPrice")
	}
	return new(big.Int).SetUint64(rsp.GasPrice), nil
}

func fixGasLimit(client ioctl.Client, caller string, execution *action.Execution) (*action.Execution, error) {
	cli, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: client.Config().Endpoint,
		Insecure: client.Config().SecureConnect && !config.Insecure,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to endpoint")
	}

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	res, err := cli.EstimateActionGasConsumption(ctx,
		&iotexapi.EstimateActionGasConsumptionRequest{
			Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
				Execution: execution.Proto(),
			},
			CallerAddress: caller,
		})
	if err != nil {
		return nil, handleClientRequestError(err, "EstimateActionGasConsumption")
	}
	return action.NewExecution(execution.Contract(), execution.Nonce(), execution.Amount(), res.Gas, execution.GasPrice(), execution.Data())
}

// SendRaw sends raw action to blockchain
func SendRaw(client ioctl.Client, selp *iotextypes.Action) error {
	cli, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: client.Config().Endpoint,
		Insecure: client.Config().SecureConnect && !config.Insecure,
	})
	if err != nil {
		return errors.Wrap(err, "failed to connect to endpoint")
	}

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	_, err = cli.SendAction(ctx, &iotexapi.SendActionRequest{Action: selp})
	if err != nil {
		return handleClientRequestError(err, "SendAction")
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

// SendAction sends signed action to blockchain
func SendAction(client ioctl.Client, cmd *cobra.Command, elp action.Envelope, signer string) error {
	sk, err := account.PrivateKeyFromSigner(client, cmd, signer, getPasswordFlagValue(cmd))
	if err != nil {
		return err
	}

	chainMeta, err := bc.GetChainMeta(client)
	if err != nil {
		return errors.Wrap(err, "failed to get chain meta")
	}
	elp.SetChainID(chainMeta.GetChainID())

	if util.AliasIsHdwalletKey(signer) {
		addr := sk.PublicKey().Address()
		signer = addr.String()
		nonce, err := nonce(client, cmd, signer)
		if err != nil {
			return errors.Wrap(err, "failed to get nonce ")
		}
		elp.SetNonce(nonce)
	}

	sealed, err := action.Sign(elp, sk)
	if err != nil {
		return errors.Wrap(err, "failed to sign action")
	}
	if err := isBalanceEnough(client, signer, sealed); err != nil {
		return errors.Wrap(err, "failed to pass balance check")
	}

	selp := sealed.Proto()
	sk.Zero()
	// TODO wait newcmd/action/actionhash impl pr #3425
	actionInfo, err := "", error(nil) // printActionProto(selp)
	if err != nil {
		return errors.Wrap(err, "failed to print action proto message")
	}

	if getAssumeYesFlagValue(cmd) == false {
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

	return SendRaw(client, selp)
}

// Execute sends signed execution transaction to blockchain
func Execute(client ioctl.Client, cmd *cobra.Command, contract string, amount *big.Int, bytecode []byte) error {
	if len(contract) == 0 && len(bytecode) == 0 {
		return output.NewError(output.InputError, "failed to deploy contract with empty bytecode", nil)
	}
	gasPriceRau, err := gasPriceInRau(client, cmd)
	if err != nil {
		return errors.Wrap(err, "failed to get gas price")
	}
	signer, err := Signer(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to get signer address")
	}
	nonce, err := nonce(client, cmd, signer)
	if err != nil {
		return errors.Wrap(err, "failed to get nonce")
	}
	gasLimit := getGasLimitFlagValue(cmd)
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil || tx == nil {
		return errors.Wrap(err, "failed to make a Execution instance")
	}
	if gasLimit == 0 {
		tx, err = fixGasLimit(client, signer, tx)
		if err != nil || tx == nil {
			return errors.Wrap(err, "failed to fix Execution gas limit")
		}
		gasLimit = tx.GasLimit()
	}
	return SendAction(
		client,
		cmd,
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(tx).Build(),
		signer,
	)
}

// Read reads smart contract on IoTeX blockchain
func Read(client ioctl.Client, cmd *cobra.Command, contract address.Address, amount string, bytecode []byte) (string, error) {
	cli, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: client.Config().Endpoint,
		Insecure: client.Config().SecureConnect && !config.Insecure,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to endpoint")
	}

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	callerAddr, _ := Signer(cmd)
	if callerAddr == "" {
		callerAddr = address.ZeroAddress
	}

	res, err := cli.ReadContract(ctx,
		&iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Amount:   amount,
				Contract: contract.String(),
				Data:     bytecode,
			},
			CallerAddress: callerAddr,
			GasLimit:      getGasLimitFlagValue(cmd),
		},
	)
	if err != nil {
		return "", handleClientRequestError(err, "ReadContract")
	}
	return res.Data, nil
}

func isBalanceEnough(client ioctl.Client, address string, act action.SealedEnvelope) error {
	accountMeta, err := account.Meta(client, address)
	if err != nil {
		return errors.Wrap(err, "failed to get account meta")
	}
	balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
	if !ok {
		return errors.New("failed to convert balance into big int")
	}
	cost, err := act.Cost()
	if err != nil {
		return errors.Wrap(err, "failed to check cost of an action")
	}
	if balance.Cmp(cost) < 0 {
		return errors.New("balance is not enough")
	}
	return nil
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
