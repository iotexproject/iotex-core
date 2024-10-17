package bc

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

var versionCmdShorts = map[config.Language]string{
	config.English: "get blockchain version",
	config.Chinese: "获取API节点版本信息",
}

var _bcVersionCmd = &cobra.Command{
	Use:   "version",
	Short: config.TranslateInLang(versionCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := bcVersion()
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(out)
		return nil
	},
}

func bcVersion() (string, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	req := &iotexapi.GetServerMetaRequest{}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	rsp, err := cli.GetServerMeta(ctx, req)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return "", output.NewError(output.APIError, sta.Message(), nil)
		}
		return "", output.NewError(output.NetworkError, "failed to get version from server", err)
	}

	title := "API endpoint version"
	detail := output.JSONString(rsp.ServerMeta)

	return fmt.Sprintf("%s:\n%s\n", title, detail), nil
}
