package alias

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	listShorts = map[ioctl.Language]string{
		ioctl.English: "list all alias",
		ioctl.Chinese: "列出全部别名",
	}
	listUses = map[ioctl.Language]string{
		ioctl.English: "list",
		ioctl.Chinese: "list",
	}
)

// NewAliasListCmd represents the alias list command
func NewAliasListCmd(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(listUses)
	short, _ := c.SelectTranslation(listShorts)
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			var keys []string
			for name := range c.Config().Aliases {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			message := aliasListMessage{AliasNumber: len(keys)}
			for _, name := range keys {
				aliasMeta := alias{Address: c.Config().Aliases[name], Name: name}
				message.AliasList = append(message.AliasList, aliasMeta)
			}
			fmt.Println(message.String())
			return nil
		},
	}
}

type aliasListMessage struct {
	AliasNumber int     `json:"aliasNumber"`
	AliasList   []alias `json:"aliasList"`
}

func (m *aliasListMessage) String() string {
	if output.Format == "" {
		lines := make([]string, 0)
		for _, aliasMeta := range m.AliasList {
			lines = append(lines, fmt.Sprintf("%s - %s", aliasMeta.Address, aliasMeta.Name))
		}
		return fmt.Sprint(strings.Join(lines, "\n"))
	}
	return output.FormatString(output.Result, m)
}
