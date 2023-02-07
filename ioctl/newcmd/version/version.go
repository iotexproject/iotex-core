// Copyright (c) 2022 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package version

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	ver "github.com/iotexproject/iotex-core/pkg/version"
)

// Multi-language support
var (
	_shorts = map[config.Language]string{
		config.English: "Print the version of ioctl and node",
		config.Chinese: "打印ioctl和节点的版本号",
	}
)

// NewVersionCmd represents the version command
func NewVersionCmd(cli ioctl.Client) *cobra.Command {
	short, _ := cli.SelectTranslation(_shorts)
	vc := &cobra.Command{
		Use:   "version",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.Printf("Client:\n%+v\n", &iotextypes.ServerMeta{
				PackageVersion:  ver.PackageVersion,
				PackageCommitID: ver.PackageCommitID,
				GitStatus:       ver.GitStatus,
				GoVersion:       ver.GoVersion,
				BuildTime:       ver.BuildTime,
			})
			apiClient, err := cli.APIServiceClient()
			if err != nil {
				return err
			}

			jwtMD, err := util.JwtAuth()
			var ctx context.Context
			if err == nil {
				ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
			}
			response, err := apiClient.GetServerMeta(
				ctx,
				&iotexapi.GetServerMetaRequest{},
			)
			if err != nil {
				if sta, ok := status.FromError(err); ok {
					if sta.Code() == codes.Unavailable {
						return ioctl.ErrInvalidEndpointOrInsecure
					}
					return errors.New(sta.Message())
				}
				return errors.Wrap(err, "failed to get version from server")
			}
			cmd.Printf("%s:\n%+v\n", cli.Config().Endpoint, response.ServerMeta)
			return nil
		},
	}

	cli.SetEndpointWithFlag(vc.PersistentFlags().StringVar)
	cli.SetInsecureWithFlag(vc.PersistentFlags().BoolVar)
	return vc
}
