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

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
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
	epochNum   uint64
	nextEpoch  bool
	nodeStatus map[bool]string
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   config.TranslateInLang(delegateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(delegateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var err error
		if nextEpoch {
			err = nextDelegates()
		} else {
			err = delegates()
		}
		return output.PrintError(err)

	},
}

type delegate struct {
	Address    string `json:"address"`
	Rank       int    `json:"rank"`
	Alias      string `json:"alias"`
	Active     bool   `json:"active"`
	Production int    `json:"production"`
	Votes      string `json:"votes"`
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
		lines := []string{fmt.Sprintf("Epoch: %d,  Start block height: %d,Total blocks in epoch: %d\n",
			m.Epoch, m.StartBlock, m.TotalBlocks)}
		formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6s   %s"
		formatDataString := "%-41s   %4d   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6d   %s"
		lines = append(lines, fmt.Sprintf(formatTitleString,
			"Address", "Rank", "Alias", "Status", "Blocks", "Votes"))
		for _, bp := range m.Delegates {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, nodeStatus[bp.Active], bp.Production, bp.Votes))
		}
		return strings.Join(lines, "\n")
	}
	return output.FormatString(output.Result, m)
}

type nextDelegatesMessage struct {
	Epoch      int        `json:"epoch"`
	Determined bool       `json:"determined"`
	Delegates  []delegate `json:"delegates"`
}

func (m *nextDelegatesMessage) String() string {
	if output.Format == "" {
		if !m.Determined {
			return fmt.Sprintf("delegates of upcoming epoch #%d are not determined", epochNum)
		}
		aliasLen := 5
		for _, bp := range m.Delegates {
			if len(bp.Alias) > aliasLen {
				aliasLen = len(bp.Alias)
			}
		}
		lines := []string{fmt.Sprintf("Epoch: %d\n", epochNum)}
		formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %s"
		formatDataString := "%-41s   %4d   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %s"
		lines = append(lines, fmt.Sprintf(formatTitleString, "Address", "Rank", "Alias", "Status", "Votes"))
		for _, bp := range m.Delegates {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, nodeStatus[bp.Active], bp.Votes))
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
	for rank, bp := range response.BlockProducersInfo {
		votes, ok := big.NewInt(0).SetString(bp.Votes, 10)
		if !ok {
			return output.NewError(output.ConvertError, "failed to convert votes into big int", nil)
		}
		delegate := delegate{
			Address:    bp.Address,
			Rank:       rank + 1,
			Alias:      aliases[bp.Address],
			Active:     bp.Active,
			Production: int(bp.Production),
			Votes:      util.RauToString(votes, util.IotxDecimalNum),
		}
		message.Delegates = append(message.Delegates, delegate)
	}
	fmt.Println(message.String())
	return nil
}

func nextDelegates() error {
	chainMeta, err := bc.GetChainMeta()
	if err != nil {
		return output.NewError(0, "failed to get chain meta", err)
	}
	epochNum = chainMeta.Epoch.Num + 1
	message := nextDelegatesMessage{Epoch: int(epochNum)}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
	}
	abpResponse, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			message.Determined = false
			fmt.Println(message.String())
			return nil
		} else if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	message.Determined = true
	var ABPs state.CandidateList
	if err := ABPs.Deserialize(abpResponse.Data); err != nil {
		return output.NewError(output.SerializationError, "failed to deserialize active BPs", err)
	}
	request = &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("BlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
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
	aliases := alias.GetAliasMap()
	for rank, bp := range BPs {
		votes := big.NewInt(0).SetBytes(bp.Votes.Bytes())
		message.Delegates = append(message.Delegates, delegate{
			Address: bp.Address,
			Rank:    rank + 1,
			Alias:   aliases[bp.Address],
			Active:  isActive[bp.Address],
			Votes:   util.RauToString(votes, util.IotxDecimalNum),
		})
	}
	fmt.Println(message.String())
	return nil
}
