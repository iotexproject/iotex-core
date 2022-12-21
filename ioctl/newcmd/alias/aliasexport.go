// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package alias

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_shorts = map[config.Language]string{
		config.English: "Export aliases to either json or yaml format",
		config.Chinese: "以json或yaml格式导出别名",
	}
	_flagUsages = map[config.Language]string{
		config.English: "set format: json/yaml",
		config.Chinese: "设置格式：json / yaml",
	}
	_invalidFlag = map[config.Language]string{
		config.English: "invalid flag %s",
		config.Chinese: "不可用的flag参数 %s",
	}
)

// NewAliasExport represents the alias export command
func NewAliasExport(c ioctl.Client) *cobra.Command {
	var format string
	short, _ := c.SelectTranslation(_shorts)
	flagUsage, _ := c.SelectTranslation(_flagUsages)
	_invalidFlag, _ := c.SelectTranslation(_invalidFlag)
	ec := &cobra.Command{
		Use:   "export",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {

			cmd.SilenceUsage = true
			exportAliases := aliases{}
			for name, address := range c.Config().Aliases {
				exportAliases.Aliases = append(exportAliases.Aliases, alias{Name: name, Address: address})
			}

			switch format {
			case "json":
				output, err := json.Marshal(exportAliases)
				if err != nil {
					return nil
				}
				cmd.Println(string(output))
				return nil
			case "yaml":
				output, err := yaml.Marshal(exportAliases)
				if err != nil {
					return nil
				}
				cmd.Println(string(output))
				return nil
			default:
				return errors.Errorf(_invalidFlag, format)
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
