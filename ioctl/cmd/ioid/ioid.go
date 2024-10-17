package ioid

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_ioIDCmdShorts = map[config.Language]string{
		config.English: "Manage ioID",
		config.Chinese: "管理 ioID",
	}
	_flagInsEndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点", // this translation
	}
	_flagInsInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接", // this translation
	}
)

// IoIDCmd represents the ins command
var IoIDCmd = &cobra.Command{
	Use:   "ioid",
	Short: config.TranslateInLang(_ioIDCmdShorts, config.UILanguage),
}

func init() {
	IoIDCmd.AddCommand(_projectRegisterCmd)
	IoIDCmd.AddCommand(_applyCmd)
	IoIDCmd.AddCommand(_projectCmd)
	IoIDCmd.AddCommand(_deviceCmd)
	IoIDCmd.AddCommand(_nameCmd)
	IoIDCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagInsEndPointUsages,
			config.UILanguage))
	IoIDCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsInsecureUsages, config.UILanguage))
}

func waitReceiptByActionHash(h string) (*iotexapi.GetReceiptByActionResponse, error) {
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

	var rsp *iotexapi.GetReceiptByActionResponse
	err = backoff.Retry(func() error {
		rsp, err = cli.GetReceiptByAction(ctx, &iotexapi.GetReceiptByActionRequest{
			ActionHash: h,
		})
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(30*time.Second), 3))
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, output.NewError(output.APIError, "not found", nil)
		} else if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetReceiptByAction api", err)
	}
	return rsp, nil
}
