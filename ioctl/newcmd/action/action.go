// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Multi-language support
var (
	_actionCmdShorts = map[config.Language]string{
		config.English: "Manage actions of IoTeX blockchain",
		config.Chinese: "管理IoTex区块链的行为", // this translation
	}
	_infoWarn = map[config.Language]string{
		config.English: "** This is an irreversible action!\n" +
			"Once an account is deleted, all the assets under this account may be lost!\n" +
			"Type 'YES' to continue, quit for anything else.",
		config.Chinese: "** 这是一个不可逆转的操作!\n" +
			"一旦一个账户被删除, 该账户下的所有资源都可能会丢失!\n" +
			"输入 'YES' 以继续, 否则退出",
	}
	_infoQuit = map[config.Language]string{
		config.English: "quit",
		config.Chinese: "退出",
	}
	_flagGasLimitUsages = map[config.Language]string{
		config.English: "set gas limit",
		config.Chinese: "设置燃气上限",
	}
	_flagGasPriceUsages = map[config.Language]string{
		config.English: `set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is "0"`,
		config.Chinese: `设置燃气费（单位：10^(-6)IOTX），如果输入为「0」，则使用默认燃气费`,
	}
	_flagNonceUsages = map[config.Language]string{
		config.English: "set nonce (default using pending nonce)",
		config.Chinese: "设置 nonce (默认使用 pending nonce)",
	}
	_flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择要签名的帐户",
	}
	_flagBytecodeUsages = map[config.Language]string{
		config.English: "set the byte code",
		config.Chinese: "设置字节码",
	}
	_flagAssumeYesUsages = map[config.Language]string{
		config.English: "answer yes for all confirmations",
		config.Chinese: "为所有确认设置 yes",
	}
	_flagPasswordUsages = map[config.Language]string{
		config.English: "input password for account",
		config.Chinese: "设置密码",
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
		log.L().Panic("input flag must be string", zap.Error(err))
	}
	return v
}

func mustUint64(v uint64, err error) uint64 {
	if err != nil {
		log.L().Panic("input flag must be uint64", zap.Error(err))
	}
	return v
}

func mustBoolean(v bool, err error) bool {
	if err != nil {
		log.L().Panic("input flag must be boolean", zap.Error(err))
	}
	return v
}


func gasLimitFlagValue(cmd *cobra.Command) (v uint64) {
	return mustUint64(cmd.Flags().GetUint64(gasLimitFlagLabel))
}

func gasPriceFlagValue(cmd *cobra.Command) (v string) {
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
	return hex.DecodeString(util.TrimHexPrefix(getBytecodeFlagValue(cmd)))
}

func getAssumeYesFlagValue(cmd *cobra.Command) (v bool) {
	return mustBoolean(cmd.Flags().GetBool(assumeYesFlagLabel))
}

func getPasswordFlagValue(cmd *cobra.Command) (v string) {
	return mustString(cmd.Flags().GetString(passwordFlagLabel))
}

func selectTranslation(client ioctl.Client, trls map[config.Language]string) string {
	txt, _ := client.SelectTranslation(trls)
	return txt
}

// NewActionCmd represents the action command
func NewActionCmd(client ioctl.Client) *cobra.Command {
	ac := &cobra.Command{
		Use:   "action",
		Short: selectTranslation(client, _actionCmdShorts),
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

	client.SetEndpointWithFlag(ac.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(ac.PersistentFlags().BoolVar)

	return ac
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
func Signer(client ioctl.Client, cmd *cobra.Command) (address string, err error) {
	addressOrAlias := getSignerFlagValue(cmd)
	if util.AliasIsHdwalletKey(addressOrAlias) {
		return addressOrAlias, nil
	}
	return client.AddressWithDefaultIfNotExist(addressOrAlias)
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
	gasPrice := gasPriceFlagValue(cmd)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}

	cli, err := client.APIServiceClient()
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
	cli, err := client.APIServiceClient()
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
func SendRaw(client ioctl.Client, cmd *cobra.Command, selp *iotextypes.Action) error {
	cli, err := client.APIServiceClient()
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
	URL := "https://"
	endpoint := client.Config().Endpoint
	explorer := client.Config().Explorer
	switch explorer {
	case "iotexscan":
		if strings.Contains(endpoint, "testnet") {
			URL += "testnet."
		}
		URL += "iotexscan.io/action/" + txhash
	case "iotxplorer":
		URL = "iotxplorer.io/actions/" + txhash
	default:
		URL = explorer + txhash
	}
	cmd.Printf("Action has been sent to blockchain.\nWait for several seconds and query this action by hash: %s\n", URL)
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
	// actionInfo, err := printActionProto(selp)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to print action proto message")
	// }
	// cmd.Println(actionInfo)

	if !getAssumeYesFlagValue(cmd) {
		infoWarn := selectTranslation(client, _infoWarn)
		infoQuit := selectTranslation(client, _infoQuit)
		if !client.AskToConfirm(infoWarn) {
			cmd.Println(infoQuit)
		}
		return nil
	}

	return SendRaw(client, cmd, selp)
}

// Execute sends signed execution transaction to blockchain
func Execute(client ioctl.Client, cmd *cobra.Command, contract string, amount *big.Int, bytecode []byte) error {
	if len(contract) == 0 && len(bytecode) == 0 {
		return errors.New("failed to deploy contract with empty bytecode")
	}
	gasPriceRau, err := gasPriceInRau(client, cmd)
	if err != nil {
		return errors.Wrap(err, "failed to get gas price")
	}
	signer, err := Signer(client, cmd)
	if err != nil {
		return errors.Wrap(err, "failed to get signer address")
	}
	nonce, err := nonce(client, cmd, signer)
	if err != nil {
		return errors.Wrap(err, "failed to get nonce")
	}

	gasLimit := gasLimitFlagValue(cmd)
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
	cli, err := client.APIServiceClient()
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to endpoint")
	}

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	callerAddr, _ := Signer(client, cmd)
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
			GasLimit:      gasLimitFlagValue(cmd),
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
