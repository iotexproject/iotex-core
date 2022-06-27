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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
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
)

const _defaultGasLimit = uint64(20000000)

// Flags
var (
	_gasLimitFlag = flag.NewUint64VarP("gas-limit", "l", _defaultGasLimit, "set gas limit")
	_gasPriceFlag = flag.NewStringVarP("gas-price", "p", "1", "set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is \"0\"")
	_nonceFlag    = flag.NewUint64VarP("nonce", "n", 0, "set nonce (default using pending nonce)")
	_signerFlag   = flag.NewStringVarP("signer", "s", "", "choose a signing account")
	_yesFlag      = flag.BoolVarP("assume-yes", "y", false, "answer yes for all confirmations")
	_passwordFlag = flag.NewStringVarP("password", "P", "", "input password for account")
)

// NewActionCmd represents the node delegate command
func NewActionCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_actionCmdUses)
	short, _ := client.SelectTranslation(_actionCmdShorts)

	act := &cobra.Command{
		Use:   use,
		Short: short,
	}

	client.SetEndpointWithFlag(act.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(act.PersistentFlags().BoolVar)

	return act
}

// Signer returns signer's address
func Signer(client ioctl.Client) (address string, err error) {
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
	return client.Address(addressOrAlias)
}

func nonce(client ioctl.Client, executor string) (uint64, error) {
	if util.AliasIsHdwalletKey(executor) {
		// for hdwallet key, get the nonce in SendAction()
		return 0, nil
	}
	nonce := _nonceFlag.Value().(uint64)
	if nonce != 0 {
		return nonce, nil
	}
	accountMeta, err := account.Meta(client, executor)
	if err != nil {
		return 0, errors.New("failed to get account meta")
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
	_passwordFlag.RegisterCommand(cmd)
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau(client ioctl.Client) (*big.Int, error) {
	if client.IsCryptoSm2() {
		return big.NewInt(0), nil
	}
	gasPrice := _gasPriceFlag.Value().(string)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.SuggestGasPriceRequest{}
	response, err := apiServiceClient.SuggestGasPrice(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, errors.New(sta.Message())
		}
		return nil, errors.New("failed to invoke SuggestGasPrice api")
	}
	return new(big.Int).SetUint64(response.GasPrice), nil
}

func fixGasLimit(client ioctl.Client, caller string, execution *action.Execution) (*action.Execution, error) {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return nil, err
	}
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

	res, err := apiServiceClient.EstimateActionGasConsumption(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, errors.New(sta.Message())
		}
		return nil, errors.New("failed to invoke EstimateActionGasConsumption api")
	}
	return action.NewExecution(execution.Contract(), execution.Nonce(), execution.Amount(), res.Gas, execution.GasPrice(), execution.Data())
}

// SendRaw sends raw action to blockchain
func SendRaw(client ioctl.Client, selp *iotextypes.Action) error {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.SendActionRequest{Action: selp}
	if _, err = apiServiceClient.SendAction(ctx, request); err != nil {
		if sta, ok := status.FromError(err); ok {
			return errors.New(sta.Message())
		}
		return errors.New("failed to invoke SendAction api")
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	txhash := hex.EncodeToString(shash[:])
	URL := "https://"
	switch config.ReadConfig.Explorer {
	case "iotexscan":
		if strings.Contains(config.ReadConfig.Endpoint, "testnet") {
			URL += "testnet."
		}
		URL += "iotexscan.io/action/" + txhash
	case "iotxplorer":
		URL = "iotxplorer.io/actions/" + txhash
	default:
		URL = config.ReadConfig.Explorer + txhash
	}
	fmt.Printf("Action has been sent to blockchain.\nWait for several seconds and query this action by hash: %s", URL)
	return nil
}

// SendAction sends signed action to blockchain
func SendAction(client ioctl.Client, cmd *cobra.Command, elp action.Envelope, signer string) error {
	prvKey, err := account.PrivateKeyFromSigner(client, cmd, signer, _passwordFlag.Value().(string))
	if err != nil {
		return err
	}

	chainMeta, err := bc.GetChainMeta(client)
	if err != nil {
		return errors.Wrap(err, "failed to get chain meta")
	}
	elp.SetChainID(chainMeta.GetChainID())

	if util.AliasIsHdwalletKey(signer) {
		addr := prvKey.PublicKey().Address()
		signer = addr.String()
		nonce, err := nonce(client, signer)
		if err != nil {
			return errors.Wrap(err, "failed to get nonce ")
		}
		elp.SetNonce(nonce)
	}

	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		return errors.Wrap(err, "failed to sign action")
	}
	if err := isBalanceEnough(client, signer, sealed); err != nil {
		return errors.Wrap(err, "failed to pass balance check")
	}

	selp := sealed.Proto()
	actionInfo, err := printActionProto(client, selp)
	if err != nil {
		return errors.Wrap(err, "failed to print action proto message")
	}

	if _yesFlag.Value() == false {
		var confirm string
		info := fmt.Sprintln(actionInfo + "\nPlease confirm your action.\n")
		line := fmt.Sprintf("%s\nOptions:", info)
		for _, option := range []string{"yes"} {
			line += " " + option
		}
		line += "\nQuit for anything else."
		fmt.Println(line)

		confirm, err := client.ReadInput()
		if err != nil {
			return err
		}
		if !client.AskToConfirm(confirm) {
			cmd.Println("quit")
			return nil
		}
	}

	return SendRaw(client, selp)
}

// Execute sends signed execution transaction to blockchain
func Execute(client ioctl.Client, cmd *cobra.Command, contract string, amount *big.Int, bytecode []byte) error {
	if len(contract) == 0 && len(bytecode) == 0 {
		return errors.New("failed to deploy contract with empty bytecode")
	}
	gasPriceRau, err := gasPriceInRau(client)
	if err != nil {
		return errors.New("failed to get gas price")
	}
	signer, err := Signer(client)
	if err != nil {
		return errors.New("failed to get signer address")
	}
	nonce, err := nonce(client, signer)
	if err != nil {
		return errors.New("failed to get nonce")
	}
	gasLimit := _gasLimitFlag.Value().(uint64)
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil || tx == nil {
		return errors.New("failed to make a Execution instance")
	}
	if gasLimit == 0 {
		tx, err = fixGasLimit(client, signer, tx)
		if err != nil || tx == nil {
			return errors.New("failed to fix Execution gaslimit")
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
func Read(client ioctl.Client, contract address.Address, amount string, bytecode []byte) (string, error) {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	callerAddr, _ := Signer(client)
	if callerAddr == "" {
		callerAddr = address.ZeroAddress
	}
	res, err := apiServiceClient.ReadContract(
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
		return "", errors.New(sta.Message())
	}
	return "", errors.New("failed to invoke ReadContract api")
}

func isBalanceEnough(client ioctl.Client, address string, act action.SealedEnvelope) error {
	accountMeta, err := account.Meta(client, address)
	if err != nil {
		return errors.New("failed to get account meta")
	}
	balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
	if !ok {
		return errors.New("failed to convert balance into big int")
	}
	cost, err := act.Cost()
	if err != nil {
		return errors.New("failed to check cost of an action")
	}
	if balance.Cmp(cost) < 0 {
		return errors.New("balance is not enough")
	}
	return nil
}
