package action

import (
	"context"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_stake2MigrateCmdUses = map[config.Language]string{
		config.English: "migrate BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "migrate 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_stake2MigrateCmdShorts = map[config.Language]string{
		config.English: "Migrate native bucket to NFT bucket on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上将原生票迁移到NFT票",
	}
)

// _stake2MigrateCmd represents the stake2 migrate command
var _stake2MigrateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2MigrateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2MigrateCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Migrate(args)
		return output.PrintError(err)

	},
}

func init() {
	RegisterWriteCommand(_stake2MigrateCmd)
}

func stake2Migrate(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		s2t := action.NewMigrateStake(bucketIndex)
		gas, err := migrateGasLimit(sender, s2t)
		if err != nil {
			return output.NewError(0, "failed to estimate gas limit", err)
		}
		gasLimit = gas
	}

	s2t := action.NewMigrateStake(bucketIndex)
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}

func migrateGasLimit(caller string, act *action.MigrateStake) (uint64, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return 0, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeMigrate{
			StakeMigrate: act.Proto(),
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
