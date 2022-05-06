// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	ver "github.com/iotexproject/iotex-core/pkg/version"
)

// Multi-language support
var (
	_uses = map[config.Language]string{
		config.English: "version",
		config.Chinese: "版本",
	}
	_shorts = map[config.Language]string{
		config.English: "Print the version of ioctl and node",
		config.Chinese: "打印ioctl和节点的版本号",
	}
	_flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
)

type versionMessage struct {
	Object      string                 `json:"object"`
	VersionInfo *iotextypes.ServerMeta `json:"versionInfo"`
}

// NewVersionCmd represents the version command
func NewVersionCmd(cli ioctl.Client) *cobra.Command {
	use, _ := cli.SelectTranslation(_uses)
	short, _ := cli.SelectTranslation(_shorts)
	vc := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := version(cli)
			return err
		},
	}

	_flagEndpointUsage, _ := cli.SelectTranslation(_flagEndpointUsages)
	_flagInsecureUsage, _ := cli.SelectTranslation(_flagInsecureUsages)
	cli.SetEndpointWithFlag(vc.PersistentFlags().StringVar, _flagEndpointUsage)
	cli.SetInsecureWithFlag(vc.PersistentFlags().BoolVar, _flagInsecureUsage)
	return vc
}

func version(cli ioctl.Client) error {
	message := versionMessage{
		Object: "Client",
		VersionInfo: &iotextypes.ServerMeta{
			PackageVersion:  ver.PackageVersion,
			PackageCommitID: ver.PackageCommitID,
			GitStatus:       ver.GitStatus,
			GoVersion:       ver.GoVersion,
			BuildTime:       ver.BuildTime,
		},
	}
	fmt.Println(message.String())
	apiClient, err := cli.APIServiceClient()
	if err != nil {
		return err
	}

	response, err := apiClient.GetServerMeta(
		context.Background(),
		&iotexapi.GetServerMetaRequest{},
	)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to get version from server", err)
	}
	message = versionMessage{
		Object:      config.ReadConfig.Endpoint,
		VersionInfo: response.ServerMeta,
	}
	fmt.Println(message.String())
	return nil
}

// TODO: output is an odd implementation, especially output.Format
func (m *versionMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s:\n%+v\n", m.Object, m.VersionInfo)
	}
	return output.FormatString(output.Result, m)
}
