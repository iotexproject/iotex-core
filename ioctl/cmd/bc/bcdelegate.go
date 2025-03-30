// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_bcDelegateCmdShorts = map[config.Language]string{
		config.English: "Get delegate information on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上读取代表信息",
	}
	_bcDelegateUses = map[config.Language]string{
		config.English: "delegate [name|address]",
		config.Chinese: "delegate [名字|地址]",
	}
)

// _bcBucketCmd represents the bc Bucket command
var _bcDelegateCmd = &cobra.Command{
	Use:     config.TranslateInLang(_bcDelegateUses, config.UILanguage),
	Short:   config.TranslateInLang(_bcDelegateCmdShorts, config.UILanguage),
	Args:    cobra.ExactArgs(1),
	Example: `ioctl bc delegate name, to read delegate information by name`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		err = getDelegate(args[0])
		return output.PrintError(err)
	},
}

type delegateMessage struct {
	Delegate *iotextypes.CandidateV2 `json:"delegate"`
}

func (m *delegateMessage) delegateString() string {
	var (
		d        = m.Delegate
		vote, _  = new(big.Int).SetString(d.TotalWeightedVotes, 10)
		token, _ = new(big.Int).SetString(d.SelfStakingTokens, 10)
		lines    []string
	)
	lines = append(lines, "{")
	lines = append(lines, fmt.Sprintf("	id: %s", d.Id))
	lines = append(lines, fmt.Sprintf("	name: %s", d.Name))
	lines = append(lines, fmt.Sprintf("	ownerAddress: %s", d.OwnerAddress))
	lines = append(lines, fmt.Sprintf("	operatorAddress: %s", d.OperatorAddress))
	lines = append(lines, fmt.Sprintf("	rewardAddress: %s", d.RewardAddress))
	lines = append(lines, fmt.Sprintf("	totalWeightedVotes: %s", util.RauToString(vote, util.IotxDecimalNum)))
	lines = append(lines, fmt.Sprintf("	selfStakeBucketIdx: %d", d.SelfStakeBucketIdx))
	lines = append(lines, fmt.Sprintf("	selfStakingTokens: %s IOTX", util.RauToString(token, util.IotxDecimalNum)))
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func (m *delegateMessage) String() string {
	if output.Format == "" {
		return m.delegateString()
	}
	return output.FormatString(output.Result, m)
}

func getDelegate(arg string) error {
	var d *iotextypes.CandidateV2
	addr, err := util.Address(arg)
	if err == nil {
		d, err = getDelegateByAddress(addr)
	} else {
		d, err = getDelegateByName(arg)
	}
	if err != nil {
		return err
	}
	message := delegateMessage{
		Delegate: d,
	}
	fmt.Println(message.String())
	return nil
}

func getDelegateByName(name string) (*iotextypes.CandidateV2, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: name,
			},
		},
	}
	requestData, err := proto.Marshal(readStakingdataRequest)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data request", err)
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodData,
		Arguments:  [][]byte{requestData},
	}

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	delegate := iotextypes.CandidateV2{}
	if err := proto.Unmarshal(response.Data, &delegate); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal response", err)
	}
	return &delegate, nil
}

func getDelegateByAddress(addr string) (*iotextypes.CandidateV2, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	readCandidatesLimit := 200
	for i := uint32(0); ; i++ {
		offset := i * uint32(readCandidatesLimit)
		size := uint32(readCandidatesLimit)
		candidateList, err := util.GetStakingCandidates(cli, offset, size)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get candidates")
		}
		if idx := slices.IndexFunc(candidateList.Candidates, func(cand *iotextypes.CandidateV2) bool {
			return cand.OperatorAddress == addr
		}); idx >= 0 {
			return candidateList.Candidates[idx], nil
		}
		if len(candidateList.Candidates) < readCandidatesLimit {
			break
		}
	}
	return nil, output.NewError(output.UndefinedError, "failed to find delegate", nil)
}
