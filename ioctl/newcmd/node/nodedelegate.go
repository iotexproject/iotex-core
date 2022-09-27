// Copyright (c) 2022 IoTeX
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

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/state"
)

// Multi-language support
var (
	_delegateUses = map[config.Language]string{
		config.English: "delegate [-e epoch-num|-n]",
		config.Chinese: "delegate [-e epoch数|-n]",
	}
	_delegateShorts = map[config.Language]string{
		config.English: "Print consensus delegates information in certain epoch",
		config.Chinese: "打印在特定epoch内的共识委托信息",
	}
	_flagEpochNumUsages = map[config.Language]string{
		config.English: "specify specific epoch",
		config.Chinese: "指定特定epoch",
	}
	_flagNextEpochUsages = map[config.Language]string{
		config.English: "query delegate of upcoming epoch",
		config.Chinese: "查询即将到来的epoch的委托",
	}
)

type nextDelegatesMessage struct {
	Epoch      int        `json:"epoch"`
	Determined bool       `json:"determined"`
	Delegates  []delegate `json:"delegates"`
}

type delegate struct {
	Address        string `json:"address"`
	Rank           int    `json:"rank"`
	Alias          string `json:"alias"`
	Active         bool   `json:"active"`
	Production     int    `json:"production"`
	Votes          string `json:"votes"`
	ProbatedStatus bool   `json:"_probatedStatus"`
}

type delegatesMessage struct {
	Epoch       int        `json:"epoch"`
	StartBlock  int        `json:"startBlock"`
	TotalBlocks int        `json:"totalBlocks"`
	Delegates   []delegate `json:"delegates"`
}

// NewNodeDelegateCmd represents the node delegate command
func NewNodeDelegateCmd(client ioctl.Client) *cobra.Command {
	var (
		epochNum  uint64
		nextEpoch bool
	)

	use, _ := client.SelectTranslation(_delegateUses)
	short, _ := client.SelectTranslation(_delegateShorts)
	flagEpochNumUsage, _ := client.SelectTranslation(_flagEpochNumUsages)
	flagNextEpochUsage, _ := client.SelectTranslation(_flagNextEpochUsages)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if nextEpoch {
				//nextDelegates
				//deprecated: It won't be able to query next delegate after Easter height, because it will be determined at the end of the epoch.
				apiServiceClient, err := client.APIServiceClient()
				if err != nil {
					return err
				}
				chainMeta, err := bc.GetChainMeta(client)
				if err != nil {
					return errors.Wrap(err, "failed to get chain meta")
				}
				epochNum = chainMeta.GetEpoch().GetNum() + 1
				message := nextDelegatesMessage{Epoch: int(epochNum)}

				ctx := context.Background()
				jwtMD, err := util.JwtAuth()
				if err == nil {
					ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
				}
				abpResponse, err := apiServiceClient.ReadState(
					ctx,
					&iotexapi.ReadStateRequest{
						ProtocolID: []byte("poll"),
						MethodName: []byte("ActiveBlockProducersByEpoch"),
						Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
					},
				)
				if err != nil {
					sta, ok := status.FromError(err)
					if ok && sta.Code() == codes.NotFound {
						message.Determined = false
						cmd.Println(message.String(epochNum))
						return nil
					} else if ok {
						return errors.New(sta.Message())
					}
					return errors.Wrap(err, "failed to invoke ReadState api")
				}
				message.Determined = true
				var abps state.CandidateList
				if err := abps.Deserialize(abpResponse.Data); err != nil {
					return errors.Wrap(err, "failed to deserialize active bps")
				}

				bpResponse, err := apiServiceClient.ReadState(
					ctx,
					&iotexapi.ReadStateRequest{
						ProtocolID: []byte("poll"),
						MethodName: []byte("BlockProducersByEpoch"),
						Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
					},
				)

				if err != nil {
					sta, ok := status.FromError(err)
					if ok {
						return errors.New(sta.Message())
					}
					return errors.Wrap(err, "failed to invoke ReadState api")
				}
				var bps state.CandidateList
				if err := bps.Deserialize(bpResponse.Data); err != nil {
					return errors.Wrap(err, "failed to deserialize bps")
				}
				isActive := make(map[string]bool)
				for _, abp := range abps {
					isActive[abp.Address] = true
				}
				aliases := client.AliasMap()
				for rank, bp := range bps {
					votes := big.NewInt(0).SetBytes(bp.Votes.Bytes())
					message.Delegates = append(message.Delegates, delegate{
						Address: bp.Address,
						Rank:    rank + 1,
						Alias:   aliases[bp.Address],
						Active:  isActive[bp.Address],
						Votes:   util.RauToString(votes, util.IotxDecimalNum),
					})
				}
				cmd.Println(message.String(epochNum))
			} else {
				if epochNum == 0 {
					chainMeta, err := bc.GetChainMeta(client)
					if err != nil {
						return errors.Wrap(err, "failed to get chain meta")
					}
					epochNum = chainMeta.GetEpoch().GetNum()
				}

				response, err := bc.GetEpochMeta(client, epochNum)
				if err != nil {
					return errors.Wrap(err, "failed to get epoch meta")
				}
				if response.EpochData == nil {
					return errors.New("ROLLDPOS is not registered")
				}
				epochData := response.EpochData
				aliases := client.AliasMap()
				message := delegatesMessage{
					Epoch:       int(epochData.Num),
					StartBlock:  int(epochData.Height),
					TotalBlocks: int(response.TotalBlocks),
				}
				probationList, err := getProbationList(client, epochNum, epochData.Height)
				if err != nil {
					return errors.Wrap(err, "failed to get probation list")
				}
				for rank, bp := range response.BlockProducersInfo {
					votes, ok := new(big.Int).SetString(bp.Votes, 10)
					if !ok {
						return errors.New("failed to convert votes into big int")
					}
					isProbated := false
					if _, ok := probationList.ProbationInfo[bp.Address]; ok {
						// if it exists in probation list
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
				cmd.Println(message.String())
			}
			return nil
		},
	}
	cmd.Flags().Uint64VarP(&epochNum, "epoch-num", "e", 0,
		flagEpochNumUsage)
	cmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", false,
		flagNextEpochUsage)
	return cmd
}

func (m *nextDelegatesMessage) String(epochNum uint64) string {
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
		if bp.Active {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, "active", bp.Votes))
		} else {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, "false", bp.Votes))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *delegatesMessage) String() string {
	aliasLen := 5
	for _, bp := range m.Delegates {
		if len(bp.Alias) > aliasLen {
			aliasLen = len(bp.Alias)
		}
	}
	lines := []string{fmt.Sprintf("Epoch: %d,  Start block height: %d,Total blocks in epoch: %d\n",
		m.Epoch, m.StartBlock, m.TotalBlocks)}
	formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6s   %-12s    %s"
	formatDataString := "%-41s   %4d   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6d   %-12s    %s"
	lines = append(lines, fmt.Sprintf(formatTitleString,
		"Address", "Rank", "Alias", "Status", "Blocks", "ProbatedStatus", "Votes"))
	for _, bp := range m.Delegates {
		if bp.Active {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, "active", bp.Production, "probated", bp.Votes))
		} else {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, "false", bp.Production, "", bp.Votes))
		}
	}
	return strings.Join(lines, "\n")
}
