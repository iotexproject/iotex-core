// Copyright (c) 202 IoTeX Foundation
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

	"github.com/iotexproject/iotex-core/ioctl/config"
)

// _aliasExportCmd doesn't support global flag "output-format", use `ioctl alias list -o [FORMAT]` instead

// Multi-language support
var (
	_exportCmd = map[config.Language]string{
		config.English: "Export aliases to either json or yaml format",
		config.Chinese: "以json或yaml格式导出别名",
	}
	_flagExportFormatUsages = map[config.Language]string{
		config.English: "set format: json/yaml",
		config.Chinese: "设置格式：json / yaml",
	}
)

// _aliasExportCmd represents the alias export command
var _aliasExportCmd = &cobra.Command{
	Use:   "export",
	Short: config.TranslateInLang(_exportCmd, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := aliasExport(cmd)
		if err == nil {
			println(output)
		}
		return err
	},
}

func init() {
	_aliasExportCmd.Flags().StringVarP(&_format,
		"format", "f", "json", config.TranslateInLang(_flagExportFormatUsages, config.UILanguage))
}

func aliasExport(cmd *cobra.Command) (string, error) {
	exportAliases := aliases{}
	for name, address := range config.ReadConfig.Aliases {
		exportAliases.Aliases = append(exportAliases.Aliases, alias{Name: name, Address: address})
	}
	switch _format {
	default:
		cmd.SilenceUsage = false
		return "", fmt.Errorf("invalid flag %s", _format)
	case "json":
		output, err := json.Marshal(exportAliases)
		if err != nil {
			return "", nil
		}
		return string(output), nil
	case "yaml":
		output, err := yaml.Marshal(exportAliases)
		if err != nil {
			return "", nil
		}
		return string(output), nil
	}
}
