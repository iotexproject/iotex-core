// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol/poll/pollpb"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   "delegate",
	Short: "get current delegates",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(delegate())
	},
}

func delegate() string {
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	requestChainMeta := iotexapi.GetChainMetaRequest{}
	ctx := context.Background()
	responseChainMeta, err := cli.GetChainMeta(ctx, &requestChainMeta)
	if err != nil {
		return err.Error()
	}
	height := responseChainMeta.ChainMeta.Height
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ActiveBlockProducersByHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(height)},
	}
	response, err := cli.ReadState(ctx, request)
	if err != nil {
		return err.Error()
	}
	var activeBlockProducers pollpb.BlockProducerList
	err = proto.Unmarshal(response.Data, &activeBlockProducers)
	if err != nil {
		log.L().Error("failed to unmarshal response data", zap.Error(err))
		return err.Error()
	}
	return proto.MarshalTextString(&activeBlockProducers)
}
