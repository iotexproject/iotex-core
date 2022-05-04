// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_importShorts = map[config.Language]string{
		config.English: "Import aliases",
		config.Chinese: "导入别名",
	}
	_importUses = map[config.Language]string{
		config.English: "import 'DATA'",
		config.Chinese: "import '数据'",
	}
	_flagImportFormatUsages = map[config.Language]string{
		config.English: "set format: json/yaml",
		config.Chinese: "设置格式：json/yaml",
	}
	_flagForceImportUsages = map[config.Language]string{
		config.English: "override existing aliases",
		config.Chinese: "覆盖现有别名",
	}
	_failToWriteToConfigFile = map[config.Language]string{
		config.English: "Failed to write to config file.",
		config.Chinese: "写入配置文件失败",
	}
)

type importMessage struct {
	ImportedNumber int     `json:"importedNumber"`
	TotalNumber    int     `json:"totalNumber"`
	Imported       []alias `json:"imported"`
	Unimported     []alias `json:"unimported"`
}

// NewAliasImportCmd represents the alias import command
func NewAliasImportCmd(c ioctl.Client) *cobra.Command {
	var (
		format      string
		forceImport bool
	)

	use, _ := c.SelectTranslation(_importUses)
	short, _ := c.SelectTranslation(_importShorts)
	flagImportFormatUsage, _ := c.SelectTranslation(_flagImportFormatUsages)
	flagForceImportUsage, _ := c.SelectTranslation(_flagForceImportUsages)
	failToWriteToConfigFile, _ := c.SelectTranslation(_failToWriteToConfigFile)

	ec := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var importedAliases aliases

			switch format {
			case "json":
				if err := json.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
					return errors.Wrap(err, "failed to unmarshal imported aliases")
				}
			case "yaml":
				if err := yaml.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
					return errors.Wrap(err, "failed to unmarshal imported aliases")
				}
			default:
				return errors.New(fmt.Sprintf("invalid flag%s", _format))
			}
			aliases := c.AliasMap()
			message := importMessage{TotalNumber: len(importedAliases.Aliases), ImportedNumber: 0}
			for _, importedAlias := range importedAliases.Aliases {
				if !forceImport && c.Config().Aliases[importedAlias.Name] != "" {
					message.Unimported = append(message.Unimported, importedAlias)
					continue
				}
				for aliases[importedAlias.Address] != "" {
					if err := c.DeleteAlias(aliases[importedAlias.Address]); err != nil {
						return errors.Wrap(err, failToWriteToConfigFile)
					}
					aliases = c.AliasMap()
				}
				if err := c.SetAlias(args[0], importedAlias.Address); err != nil {
					return errors.Wrapf(err, failToWriteToConfigFile)
				}
				message.Imported = append(message.Imported, importedAlias)
				message.ImportedNumber++
			}
			c.WriteAlias()
			line := fmt.Sprintf("%d/%d aliases imported\nExisted aliases:", message.ImportedNumber, message.TotalNumber)
			for _, alias := range message.Unimported {
				line += fmt.Sprint(" " + alias.Name)
			}
			cmd.Println(line)
			return nil

		},
	}

	ec.Flags().StringVarP(&format,
		"format=", "f", "json", flagImportFormatUsage)
	ec.Flags().BoolVarP(&forceImport,
		"force-import", "F", false, flagForceImportUsage)

	return ec
}
