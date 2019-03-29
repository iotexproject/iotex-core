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

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	ver "github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of ioctl and node",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version())
	},
}

func version() string {
	versionInfo := &iotextypes.ServerMeta{
		PackageVersion:  ver.PackageVersion,
		PackageCommitID: ver.PackageCommitID,
		GitStatus:       ver.GitStatus,
		GoVersion:       ver.GoVersion,
		BuildTime:       ver.BuildTime,
	}
	fmt.Printf("Client:\n%+v\n\n", versionInfo)
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetServerMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetServerMeta(ctx, request)
	if err != nil {
		return "failed to get version from server: " + err.Error()
	}
	return fmt.Sprintf("Server: %s\n%+v", config.ReadConfig.Endpoint, response.ServerMeta)
}
