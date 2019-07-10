// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// aliasImportCmd represents the alias import command
var aliasImportCmd = &cobra.Command{
	Use:   "import 'DATA'",
	Short: "Import aliases",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := aliasImport(cmd, args)
		return err
	},
}

type importMessage struct {
	ImportedNumber int     `json:"importedNumber"`
	TotalNumber    int     `json:"totalNumber"`
	Imported       []alias `json:"imported"`
	Unimported     []alias `json:"unimported"`
}

func init() {
	aliasImportCmd.Flags().StringVarP(&format,
		"format=", "f", "json", "set format: json/yaml")
	aliasImportCmd.Flags().BoolVarP(&forceImport,
		"force-import", "F", false, "cover existed aliases forcely")
}

func aliasImport(cmd *cobra.Command, args []string) error {
	var importedAliases aliases
	switch format {
	default:
		return output.PrintError(output.FlagError, fmt.Sprintf("invalid format flag %s", format))
	case "json":
		if err := json.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
			return output.PrintError(output.SerializationError, err.Error())
		}
	case "yaml":
		if err := yaml.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
			return output.PrintError(output.SerializationError, err.Error())
		}
	}
	importedNum, totalNum := 0, 0
	aliases := GetAliasMap()
	message := importMessage{TotalNumber: len(importedAliases.Aliases), ImportedNumber: 0}
	for _, importedAlias := range importedAliases.Aliases {
		totalNum++
		if !forceImport && config.ReadConfig.Aliases[importedAlias.Name] != "" {
			message.Unimported = append(message.Unimported, importedAlias)
			message.ImportedNumber++
			continue
		}
		for aliases[importedAlias.Address] != "" {
			delete(config.ReadConfig.Aliases, aliases[importedAlias.Address])
			aliases = GetAliasMap()
		}
		config.ReadConfig.Aliases[importedAlias.Name] = importedAlias.Address
		message.Imported = append(message.Imported, importedAlias)
		importedNum++
	}
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.PrintError(output.SerializationError, err.Error())
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.PrintError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile))
	}
	fmt.Println(message.String())
	return nil
}

func (m *importMessage) String() string {
	if output.Format == "" {
		line:=fmt.Sprintf("%d/%d aliases imported\nExisted aliases:", m.ImportedNumber, m.TotalNumber)
		for _, alias := range m.Unimported {
			line+=fmt.Sprint(" " + alias.Name)
		}
		return line
	}
	return output.FormatString(output.Result, m)
}
