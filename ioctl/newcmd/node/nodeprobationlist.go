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
					return errors.New("failed to get chain meta")
				}
				epochNum = chainMeta.GetEpoch().GetNum()
			}
			probationlist, err := getProbationList(client, epochNum)
			if err != nil {
				return errors.New("failed to get probation list")
			}
			message := &probationListMessage{
				EpochNumber:   epochNum,
				IntensityRate: probationlist.IntensityRate,
				DelegateList:  make([]string, 0),
			}
			for addr := range probationlist.ProbationInfo {
				message.DelegateList = append(message.DelegateList, addr)
			}

			cmd.Println(message.String())
			return nil
		},
	}
	cmd.PersistentFlags().Uint64VarP(&epochNum, "epoch-num", "e", 0, flagEpochNumUsages)
	return cmd
}

type probationListMessage struct {
	EpochNumber   uint64   `json:"epochnumber"`
	IntensityRate uint32   `json:"intensityrate"`
	DelegateList  []string `json:"delegatelist"`
}

func (m *probationListMessage) String() string {
	byteAsJSON, err := json.MarshalIndent(m.DelegateList, "", "  ")
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprintf("EpochNumber : %d, IntensityRate : %d%%\nProbationList : %s",
		m.EpochNumber,
		m.IntensityRate,
		fmt.Sprint(string(byteAsJSON)),
	)
}
