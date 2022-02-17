package node

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/state"
)

// Multi-language support
var (
	delegateUses = map[config.Language]string{
		config.English: "delegate [-e epoch-num|-n]",
		config.Chinese: "delegate [-e epoch数|-n]",
	}
	delegateShorts = map[config.Language]string{
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
	ProbatedStatus bool   `json:"probatedStatus"`
}

type delegatesMessage struct {
	Epoch       int        `json:"epoch"`
	StartBlock  int        `json:"startBlock"`
	TotalBlocks int        `json:"totalBlocks"`
	Delegates   []delegate `json:"delegates"`
}

// NewNodeDelegateCmd represents the node delegate command
func NewNodeDelegateCmd(c ioctl.Client) *cobra.Command {
	var endpoint string
	var insecure bool

	use, _ := c.SelectTranslation(delegateUses)
	short, _ := c.SelectTranslation(delegateShorts)
	flagEpochNumUsage, _ := c.SelectTranslation(flagEpochNumUsages)
	flagNextEpochUsage, _ := c.SelectTranslation(flagNextEpochUsages)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var err error
			if nextEpoch {
				//nextDelegates
				//deprecated: It won't be able to query next delegate after Easter height, because it will be determined at the end of the epoch.
				apiServiceClient, err := c.APIServiceClient(ioctl.APIServiceConfig{
					Endpoint: endpoint,
					Insecure: insecure,
				})
				if err != nil {
					return err
				}
				chainMeta, err := bc.GetChainMeta(c)
				if err != nil {
					return output.NewError(0, "failed to get chain meta", err)
				}
				epochNum = chainMeta.Epoch.Num + 1
				message := nextDelegatesMessage{Epoch: int(epochNum)}

				ctx := context.Background()
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
				aliases := c.AliasMap()
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
			} else {
				if epochNum == 0 {
					chainMeta, err := bc.GetChainMeta(c)
					if err != nil {
						return output.NewError(0, "failed to get chain meta", err)
					}
					epochNum = chainMeta.Epoch.Num
				}

				response, err := bc.GetEpochMeta(epochNum, c)

				if err != nil {
					return output.NewError(0, "failed to get epoch meta", err)
				}
				epochData := response.EpochData
				aliases := c.AliasMap()
				message := delegatesMessage{
					Epoch:       int(epochData.Num),
					StartBlock:  int(epochData.Height),
					TotalBlocks: int(response.TotalBlocks),
				}
				probationListRes, err := bc.GetProbationList(epochNum, c)
				if err != nil {
					return output.NewError(0, "failed to get probation list", err)
				}
				probationList := &vote.ProbationList{}
				if probationListRes != nil {
					if err := probationList.Deserialize(probationListRes.Data); err != nil {
						return output.NewError(output.SerializationError, "failed to deserialize probation list", err)
					}
				}
				for rank, bp := range response.BlockProducersInfo {
					votes, ok := big.NewInt(0).SetString(bp.Votes, 10)
					if !ok {
						return output.NewError(output.ConvertError, "failed to convert votes into big int", nil)
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
				fmt.Println(message.String())
			}

			return output.PrintError(err)
		},
	}
	cmd.Flags().Uint64VarP(&epochNum, "epoch-num", "e", 0,
		flagEpochNumUsage)
	cmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", false,
		flagNextEpochUsage)
	nodeStatus = map[bool]string{true: "active", false: ""}
	probatedStatus = map[bool]string{true: "probated", false: ""}
	return cmd
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
		formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6s   %-12s    %s"
		formatDataString := "%-41s   %4d   %-" + strconv.Itoa(aliasLen) + "s   %-6s   %-6d   %-12s    %s"
		lines = append(lines, fmt.Sprintf(formatTitleString,
			"Address", "Rank", "Alias", "Status", "Blocks", "ProbatedStatus", "Votes"))
		for _, bp := range m.Delegates {
			lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, bp.Rank,
				bp.Alias, nodeStatus[bp.Active], bp.Production, probatedStatus[bp.ProbatedStatus], bp.Votes))
		}
		return strings.Join(lines, "\n")
	}
	return output.FormatString(output.Result, m)
}
