package node

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_nodeCmdShorts = map[config.Language]string{
		config.English: "Deal with nodes of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链的节点",
	}
)

// NewNodeCmd represents the new node command.
func NewNodeCmd(client ioctl.Client) *cobra.Command {
	nodeShorts, _ := client.SelectTranslation(_nodeCmdShorts)
	
	nc := &cobra.Command{
		Use:   "node",
		Short: nodeShorts,
	}
	nc.AddCommand(NewNodeDelegateCmd(client))
	nc.AddCommand(NewNodeRewardCmd(client))

	client.SetEndpointWithFlag(nc.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(nc.PersistentFlags().BoolVar)
	return nc
}
