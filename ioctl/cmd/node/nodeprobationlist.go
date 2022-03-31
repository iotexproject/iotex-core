// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	probationlistCmdUses = map[config.Language]string{
		config.English: "probationlist [-e epoch-num]",
		config.Chinese: "probationlist [-e epoch数]",
	}
	probationlistCmdShorts = map[config.Language]string{
		config.English: "Print probation list at given epoch",
		config.Chinese: "",
	}
)

// nodeProbationlistCmd represents querying probation list command
var nodeProbationlistCmd = &cobra.Command{
	Use:   config.TranslateInLang(probationlistCmdUses, config.UILanguage),
	Short: config.TranslateInLang(probationlistCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := probationlist()
		return output.PrintError(err)
	},
}

type probationListMessage struct {
	EpochNumber   uint64   `json:"epochnumber"`
	IntensityRate uint32   `json:"intensityrate"`
	DelegateList  []string `json:"delegatelist"`
}

func (m *probationListMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("EpochNumber : %d, IntensityRate : %d%%\nProbationList : %s",
			m.EpochNumber,
			m.IntensityRate,
			output.JSONString(m.DelegateList),
		)
		return message
	}
	return output.FormatString(output.Result, m)
}

func init() {
	nodeProbationlistCmd.Flags().Uint64VarP(&_epochNum, "epoch-num", "e", 0,
		config.TranslateInLang(_flagEpochNumUsages, config.UILanguage))
}

func probationlist() error {
	if _epochNum == 0 {
		chainMeta, err := bc.GetChainMeta()
		if err != nil {
			return output.NewError(0, "failed to get chain meta", err)
		}
		epochData := chainMeta.GetEpoch()
		if epochData == nil {
			return output.NewError(0, "ROLLDPOS is not registered", nil)
		}
		_epochNum = epochData.Num
	}
	response, err := bc.GetEpochMeta(_epochNum)
	if err != nil {
		return output.NewError(0, "failed to get epoch meta", err)
	}
	probationlist, err := getProbationList(_epochNum, response.EpochData.Height)
	if err != nil {
		return output.NewError(0, "failed to get probation list", err)
	}
	message := &probationListMessage{
		EpochNumber:   _epochNum,
		IntensityRate: probationlist.IntensityRate,
		DelegateList:  make([]string, 0),
	}
	for addr := range probationlist.ProbationInfo {
		message.DelegateList = append(message.DelegateList, addr)
	}

	fmt.Println(message.String())
	return nil
}
