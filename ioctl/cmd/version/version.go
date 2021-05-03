// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	ver "github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Multi-language support
var (
	versionCmdUses = map[config.Language]string{
		config.English: "version",
		config.Chinese: "version",
	}
	versionCmdShorts = map[config.Language]string{
		config.English: "Print the version of ioctl and node",
		config.Chinese: "打印ioctl和节点的版本",
	}
	flagEndpointUsage = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagInsecureUsage = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   config.TranslateInLang(versionCmdUses, config.UILanguage),
	Short: config.TranslateInLang(versionCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := version()
		return err
	},
}

type versionMessage struct {
	Object      string                 `json:"object"`
	VersionInfo *iotextypes.ServerMeta `json:"versionInfo"`
}

func init() {
	VersionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(flagEndpointUsage, config.UILanguage))
	VersionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(flagInsecureUsage, config.UILanguage))
}

func version() error {
	message := versionMessage{}

	message.Object = "Client"
	message.VersionInfo = &iotextypes.ServerMeta{
		PackageVersion:  ver.PackageVersion,
		PackageCommitID: ver.PackageCommitID,
		GitStatus:       ver.GitStatus,
		GoVersion:       ver.GoVersion,
		BuildTime:       ver.BuildTime,
	}
	fmt.Println(message.String())

	message = versionMessage{Object: config.ReadConfig.Endpoint}
	cli, ctx, err := util.GetAPIClientAndContext()
	if err != nil {
		return err
	}
	request := &iotexapi.GetServerMetaRequest{}

	response, err := cli.GetServerMeta(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError,
			"failed to get version from server", err)
	}

	message.VersionInfo = response.ServerMeta
	fmt.Println(message.String())
	return nil
}

func (m *versionMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s:\n%+v\n", m.Object, m.VersionInfo)
	}
	return output.FormatString(output.Result, m)
}
