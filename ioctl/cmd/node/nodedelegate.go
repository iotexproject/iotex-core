// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/state"
)

const (
	protocolID          = "staking"
	readCandidatesLimit = 20000
)

// Multi-language support
var (
	delegateCmdUses = map[config.Language]string{
		config.English: "delegate [-e epoch-num|-n]",
		config.Chinese: "delegate [-e epoch数|-n]",
	}
	delegateCmdShorts = map[config.Language]string{
		config.English: "Print consensus delegates information in certain epoch",
		config.Chinese: "打印在特定epoch内的共识委托信息",
	}
	flagEpochNumUsages = map[config.Language]string{
		config.English: "specify specific epoch",
		config.Chinese: "指定特定epoch",
	}
	flagNextEpochUsages = map[config.Language]string{
		config.English: "query delegate of upcoming epoch",
		config.Chinese: "查询即将到来的epoch的委托",
	}
)

var (
	epochNum       uint64
	nextEpoch      bool
	nodeStatus     map[bool]string
	probatedStatus map[bool]string
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   config.TranslateInLang(delegateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(delegateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := delegates()
		return output.PrintError(err)

	},
}

type delegate struct {
	Address        string `json:"address"`
	Name           string `json:"string"`
	Rank           int    `json:"rank"`
	Alias          string `json:"alias"`
	Active         bool   `json:"active"`
	Production     int    `json:"production"`
	Votes          string `json:"votes"`
	ProbatedStatus bool   `json:"probatedStatus"`
}

type delegatesMessage struct {
	Epoch       int        `json:"epoch"`
	StartBlock  int        `json:"startBlock"`
	TotalBlocks int        `json:"totalBlocks"`
	Delegates   []delegate `json:"delegates"`
}

func (m *delegatesMessage) String() string {
	if output.Format == "" {
		aliasLen := 5
		for _, bp := range m.Delegates {
			if len(bp.Alias) > aliasLen {
				aliasLen = len(bp.Alias)
			}
		}
		lines := []string{fmt.Sprintf("Epoch: %d,  Start block height: %d,Total blocks produced in epoch: %d\n",
			m.Epoch, m.StartBlock, m.TotalBlocks)}
		formatTitleString := "%-41s   %-12s   %-4s   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6s   %-12s    %s"
		formatDataString := "%-41s   %-12s   %4d   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6d   %-12s    %s"
		lines = append(lines, fmt.Sprintf(formatTitleString,
			"Address", "Name", "Rank", "Alias", "Status", "Blocks", "ProbatedStatus", "Votes"))
		for _, bp := range m.Delegates {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Name, bp.Rank, bp.Alias, nodeStatus[bp.Active], bp.Production, probatedStatus[bp.ProbatedStatus], bp.Votes))
		}
		return strings.Join(lines, "\n")
	}
	return output.FormatString(output.Result, m)
}

func init() {
	nodeDelegateCmd.Flags().Uint64VarP(&epochNum, "epoch-num", "e", 0,
		config.TranslateInLang(flagEpochNumUsages, config.UILanguage))
	nodeDelegateCmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", false,
		config.TranslateInLang(flagNextEpochUsages, config.UILanguage))
	nodeStatus = map[bool]string{true: "active", false: ""}
	probatedStatus = map[bool]string{true: "probated", false: ""}
}

func delegates() error {
	if epochNum == 0 {
		chainMeta, err := bc.GetChainMeta()
		if err != nil {
			return output.NewError(0, "failed to get chain meta", err)
		}
		epochNum = chainMeta.Epoch.Num
	}
	response, err := bc.GetEpochMeta(epochNum)
	if err != nil {
		return output.NewError(0, "failed to get epoch meta", err)
	}
	epochData := response.EpochData
	aliases := alias.GetAliasMap()
	message := delegatesMessage{
		Epoch:       int(epochData.Num),
		StartBlock:  int(epochData.Height),
		TotalBlocks: int(response.TotalBlocks),
	}
	probationList, err := getProbationList(epochNum)
	if err != nil {
		return output.NewError(0, "failed to get probation list", err)
	}
	if epochData.Height >= config.ReadConfig.FairBankHeight {
		return delegatesV2(probationList, response, &message)
	}
	for rank, bp := range response.BlockProducersInfo {
		votes, ok := big.NewInt(0).SetString(bp.Votes, 10)
		if !ok {
			return output.NewError(output.ConvertError, "failed to convert votes into big int", nil)
		}
		isProbated := false
		if _, ok := probationList.ProbationInfo[bp.Address]; ok {
			// if it exists in probation info
			isProbated = true
		}
		delegate := delegate{
			Address:        bp.Address,
			Rank:           rank + 1,
			Alias:          aliases[bp.Address],
			Active:         bp.Active,
			Production:     int(bp.Production),
			Votes:          util.RauToString(votes, util.IotxDecimalNum),
			ProbatedStatus: isProbated,
		}
		message.Delegates = append(message.Delegates, delegate)
	}
	fmt.Println(message.String())
	return nil
}

func delegatesV2(pb *vote.ProbationList, epochMeta *iotexapi.GetEpochMetaResponse, message *delegatesMessage) error {
	chainMeta, err := bc.GetChainMeta()
	if err != nil {
		return output.NewError(0, "failed to get chain meta", err)
	}
	epochNum = chainMeta.Epoch.Num
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
	}
	abpResponse, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			fmt.Println(message.String())
			return nil
		} else if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	var ABPs state.CandidateList
	if err := ABPs.Deserialize(abpResponse.Data); err != nil {
		return output.NewError(output.SerializationError, "failed to deserialize active BPs", err)
	}
	request = &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("BlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
	}
	bpResponse, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	var BPs state.CandidateList
	if err := BPs.Deserialize(bpResponse.Data); err != nil {
		return output.NewError(output.SerializationError, "failed to deserialize BPs", err)
	}
	isActive := make(map[string]bool)
	for _, abp := range ABPs {
		isActive[abp.Address] = true
	}
	production := make(map[string]int)
	for _, info := range epochMeta.BlockProducersInfo {
		production[info.Address] = int(info.Production)
	}
	aliases := alias.GetAliasMap()
	for rank, bp := range BPs {
		isProbated := false
		if _, ok := pb.ProbationInfo[bp.Address]; ok {
			isProbated = true
		}
		votes := big.NewInt(0).SetBytes(bp.Votes.Bytes())
		message.Delegates = append(message.Delegates, delegate{
			Address:        bp.Address,
			Rank:           rank + 1,
			Alias:          aliases[bp.Address],
			Active:         isActive[bp.Address],
			Production:     production[bp.Address],
			Votes:          util.RauToString(votes, util.IotxDecimalNum),
			ProbatedStatus: isProbated,
		})
	}
	fillMessage(cli, message, aliases, isActive, pb)
	fmt.Println(message.String())
	return nil
}

func getProbationList(epochNum uint64) (*vote.ProbationList, error) {
	probationListRes, err := bc.GetProbationList(epochNum)
	if err != nil {
		return nil, err
	}
	probationList := &vote.ProbationList{}
	if probationListRes != nil {
		if err := probationList.Deserialize(probationListRes.Data); err != nil {
			return nil, err
		}
	}
	return probationList, nil
}

func fillMessage(cli iotexapi.APIServiceClient, message *delegatesMessage, alias map[string]string, active map[string]bool, pb *vote.ProbationList) error {
	cl, err := getAllStakingCandidates(cli)
	if err != nil {
		return err
	}
	addressMap := make(map[string]*iotextypes.CandidateV2)
	for _, candidate := range cl.Candidates {
		addressMap[candidate.OperatorAddress] = candidate
	}
	delegateAddressMap := make(map[string]struct{})
	for _, m := range message.Delegates {
		delegateAddressMap[m.Address] = struct{}{}
	}
	for i, m := range message.Delegates {
		if c, ok := addressMap[m.Address]; ok {
			message.Delegates[i].Name = c.Name
			continue
		}
	}
	rank := len(message.Delegates) + 1
	for _, candidate := range cl.Candidates {
		if _, ok := delegateAddressMap[candidate.OperatorAddress]; ok {
			continue
		}
		isProbated := false
		if _, ok := pb.ProbationInfo[candidate.OwnerAddress]; ok {
			isProbated = true
		}
		iotx, err := util.StringToIOTX(candidate.TotalWeightedVotes)
		if err != nil {
			return err
		}
		message.Delegates = append(message.Delegates, delegate{
			Address:        candidate.OperatorAddress,
			Name:           candidate.Name,
			Rank:           rank,
			Alias:          alias[candidate.OperatorAddress],
			Active:         active[candidate.OperatorAddress],
			Votes:          iotx,
			ProbatedStatus: isProbated,
		})
		rank++
	}
	return nil
}

func getAllStakingCandidates(chainClient iotexapi.APIServiceClient) (candidateListAll *iotextypes.CandidateListV2, err error) {
	candidateListAll = &iotextypes.CandidateListV2{}
	for i := uint32(0); ; i++ {
		offset := i * readCandidatesLimit
		size := uint32(readCandidatesLimit)
		candidateList, err := getStakingCandidates(chainClient, offset, size)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get candidates")
		}
		candidateListAll.Candidates = append(candidateListAll.Candidates, candidateList.Candidates...)
		if len(candidateList.Candidates) < readCandidatesLimit {
			break
		}
	}
	return
}

func getStakingCandidates(chainClient iotexapi.APIServiceClient, offset, limit uint32) (candidateList *iotextypes.CandidateListV2, err error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(protocolID),
		MethodName: methodName,
		Arguments:  [][]byte{arg},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return
	}
	candidateList = &iotextypes.CandidateListV2{}
	if err := proto.Unmarshal(readStateRes.GetData(), candidateList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal VoteBucketList")
	}
	return
}
