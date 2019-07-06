// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	ver "github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of ioctl and node",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		oErr := version()
		if output.OutputFormat == "" && oErr.Code != 0 {
			return fmt.Errorf("Code %d, Info:%s", oErr.Code, oErr.Info)
		}
		return nil
	},
}

type versionMessage struct {
	Type        output.MessageType     `json:"type"`
	Object      string                 `json:"object"`
	VersionInfo *iotextypes.ServerMeta `json:"versionInfo"`
}

func init() {
	VersionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	VersionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
}

func version() output.Error {
	message := versionMessage{Type: output.Result}
	emptyError := output.Error{Code: 0, Info: ""}
	var oErr output.Error

	message.Object = "Client"
	message.VersionInfo = &iotextypes.ServerMeta{
		PackageVersion:  ver.PackageVersion,
		PackageCommitID: ver.PackageCommitID,
		GitStatus:       ver.GitStatus,
		GoVersion:       ver.GoVersion,
		BuildTime:       ver.BuildTime,
	}
	printVersion(message, emptyError)

	message = versionMessage{Type: output.Result, Object: config.ReadConfig.Endpoint}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		oErr = output.Error{Code: output.NetworkError, Info: err.Error()}
		printVersion(message, oErr)
		return oErr
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetServerMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetServerMeta(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			oErr = output.Error{Code: 1, Info: sta.Message()}
		} else {
			oErr = output.Error{
				Code: output.APIError,
				Info: "failed to get version from server: " + err.Error(),
			}
		}
		printVersion(message, oErr)
		return oErr
	}

	message.VersionInfo = response.ServerMeta
	printVersion(message, emptyError)
	return oErr
}

func printVersion(message versionMessage, oErr output.Error) {
	switch {
	default:
		if oErr.Code == 0 {
			fmt.Printf("%s:\n%+v\n\n", message.Object, message.VersionInfo)
		} else {
			fmt.Printf("%s:\n", message.Object)
		}

	case output.OutputFormat == "json":
		out := output.Output{Error: oErr, Message: message}
		byteAsJSON, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Panic(err)
		}
		fmt.Println(string(byteAsJSON))
	}
}
