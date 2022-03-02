// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	shorts = map[ioctl.Language]string{
		ioctl.English: "Export aliases to either json or yaml format",
		ioctl.Chinese: "以json或yaml格式导出别名",
	}
	uses = map[ioctl.Language]string{
		ioctl.English: "export",
		ioctl.Chinese: "export",
	}
	flagUsages = map[ioctl.Language]string{
		ioctl.English: "set format: json/yaml",
		ioctl.Chinese: "设置格式：json / yaml",
	}
	invalidFlag = map[ioctl.Language]string{
		ioctl.English: "invalid flag %s",
		ioctl.Chinese: "不可用的flag参数 %s",
	}
)

// NewAliasExport represents the alias export command
func NewAliasExport(c ioctl.Client) *cobra.Command {
	var format string

	use, _ := c.SelectTranslation(uses)
	short, _ := c.SelectTranslation(shorts)
	flagUsage, _ := c.SelectTranslation(flagUsages)
	invalidFlag, _ := c.SelectTranslation(invalidFlag)
	ec := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {

			cmd.SilenceUsage = true
			exportAliases := aliases{}
			for name, address := range config.ReadConfig.Aliases {
				exportAliases.Aliases = append(exportAliases.Aliases, alias{Name: name, Address: address})
			}

			switch format {

			default:
				cmd.SilenceUsage = false
				return fmt.Errorf(invalidFlag, format)
			case "json":
				output, err := json.Marshal(exportAliases)
				if err != nil {
					return nil
				}
				println(string(output))
				return nil
			case "yaml":
				output, err := yaml.Marshal(exportAliases)
				if err != nil {
					return nil
				}
				println(string(output))
				return nil
			}

		},
	}
	ec.Flags().StringVarP(&format,
		"format", "f", "json", flagUsage)

	return ec

}

type aliases struct {
	Aliases []alias `json:"aliases" yaml:"aliases"`
}

type alias struct {
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
}
