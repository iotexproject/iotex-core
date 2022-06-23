// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
)

// Multi-language support
var (
	_probationlistCmdUses = map[config.Language]string{
		config.English: "probationlist [-e epoch-num]",
		config.Chinese: "probationlist [-e epoch数]",
	}
	_probationlistCmdShorts = map[config.Language]string{
		config.English: "Print probation list at given epoch",
		config.Chinese: "打印给定epoch内的试用名单",
	}
)

// NewNodeProbationlistCmd represents querying probation list command
func NewNodeProbationlistCmd(client ioctl.Client) *cobra.Command {
	var epochNum uint64
	use, _ := client.SelectTranslation(_probationlistCmdUses)
	short, _ := client.SelectTranslation(_probationlistCmdShorts)
	flagEpochNumUsages, _ := client.SelectTranslation(_flagEpochNumUsages)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
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
			probationlist, err := getProbationList(client, epochNum, response.EpochData.Height)
			if err != nil {
				return errors.Wrap(err, "failed to get probation list")
			}
			delegateList := make([]string, 0)
			for addr := range probationlist.ProbationInfo {
				delegateList = append(delegateList, addr)
			}
			byteAsJSON, err := json.MarshalIndent(delegateList, "", "  ")
			if err != nil {
				log.Panic(err)
			}
			cmd.Printf("EpochNumber : %d, IntensityRate : %d%%\nProbationList : %s",
				epochNum,
				probationlist.IntensityRate,
				fmt.Sprint(string(byteAsJSON)),
			)
			return nil
		},
	}
	cmd.PersistentFlags().Uint64VarP(&epochNum, "epoch-num", "e", 0, flagEpochNumUsages)
	return cmd
}

func getProbationList(client ioctl.Client, epochNum uint64, epochStartHeight uint64) (*vote.ProbationList, error) {
	probationListRes, err := bc.GetProbationList(client, epochNum, epochStartHeight)
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
