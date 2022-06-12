package node

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_probationlistCmdUses = map[config.Language]string{
		config.English: "probationlist [-e epoch-num]",
		config.Chinese: "probationlist [-e epoch数]",
	}
	_probationlistCmdShorts = map[config.Language]string{
		config.English: "Print probation list at given epoch",
		config.Chinese: "",
	}

	_epochNum uint64
)

type probationListMessage struct {
	EpochNumber   uint64   `json:"epochnumber"`
	IntensityRate uint32   `json:"intensityrate"`
	DelegateList  []string `json:"delegatelist"`
}

// String is a string representation of the probationListMessage struct.
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

// NewNodeProbationListCmd prints the probation list at the given epoch.
func NewNodeProbationListCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_probationlistCmdUses)
	short, _ := client.SelectTranslation(_probationlistCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		// Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if _epochNum == 0 {
				chainMeta, err := bc.GetChainMeta(client)
				if err != nil {
					return errors.Wrap(err, "failed to get chain meta")
				}

				epochData := chainMeta.GetEpoch()
				if epochData == nil {
					return errors.New("ROLLDPOS is not registered")
				}

				_epochNum = epochData.Num
			}

			probationlist, err := getProbationList(client, _epochNum)
			if err != nil {
				return errors.Wrap(err, "failed to get probation list")
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
		},
	}

	return cmd
}

func getProbationList(client ioctl.Client, _epochNum uint64) (*vote.ProbationList, error) {
	probationListRes, err := bc.GetProbationList(client, _epochNum)
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
