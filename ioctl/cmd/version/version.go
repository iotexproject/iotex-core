// Copyright (c) 2022 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package version

import (
	_ "embed" // embed ioctl standalone version file
	"fmt"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	ver "github.com/iotexproject/iotex-core/v2/pkg/version"
)

// Multi-language support
var (
	_versionCmdShorts = map[config.Language]string{
		config.English: "Print the version of ioctl and node",
		config.Chinese: "打印ioctl和节点的版本",
	}
	_flagEndpointUsage = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecureUsage = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
	//go:embed version
	_ioctlStandaloneVersion string
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: config.TranslateInLang(_versionCmdShorts, config.UILanguage),
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
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpointUsage, config.UILanguage))
	VersionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsecureUsage, config.UILanguage))
}

func version() error {
	fmt.Printf("Version: %s\n\n", _ioctlStandaloneVersion)

	message := versionMessage{}

	message.Object = "Build Info"
	message.VersionInfo = &iotextypes.ServerMeta{
		PackageVersion:  ver.PackageVersion,
		PackageCommitID: ver.PackageCommitID,
		GitStatus:       ver.GitStatus,
		GoVersion:       ver.GoVersion,
		BuildTime:       ver.BuildTime,
	}
	fmt.Println(message.String())
	return nil
}

func (m *versionMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s:\n%+v\n", m.Object, m.VersionInfo)
	}
	return output.FormatString(output.Result, m)
}
