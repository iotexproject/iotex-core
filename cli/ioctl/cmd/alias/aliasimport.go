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
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// aliasImportCmd represents the alias import command
var aliasImportCmd = &cobra.Command{
	Use:   "import 'DATA'",
	Short: "Import aliases",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := aliasImport(cmd, args)
		if err == nil {
			println(output)
		}
		return err
	},
}

func init() {
	aliasImportCmd.Flags().StringVarP(&format,
		"format=", "f", "json", "set format: json/yaml")
	aliasImportCmd.Flags().BoolVarP(&forceImport,
		"force-import", "F", false, "cover existed aliases forcely")
}

func aliasImport(cmd *cobra.Command, args []string) (string, error) {
	var importedAliases aliases

	switch format {
	default:
		cmd.SilenceUsage = false
		return "", fmt.Errorf("invalid flag %s", format)
	case "json":
		if err := json.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
			return "", err
		}
	case "yaml":
		if err := yaml.Unmarshal([]byte(args[0]), &importedAliases); err != nil {
			return "", err
		}
	}
	importedNum, totalNum := 0, 0
	aliases := GetAliasMap()
	for _, importedAlias := range importedAliases.Aliases {
		totalNum++
		if !forceImport && config.ReadConfig.Aliases[importedAlias.Name] != "" {
			fmt.Println("existed alias " + importedAlias.Name)
			continue
		}
		if aliases[importedAlias.Address] != "" {
			delete(config.ReadConfig.Aliases, aliases[importedAlias.Name])
		}
		config.ReadConfig.Aliases[importedAlias.Name] = importedAlias.Address
		importedNum++
	}
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", config.DefaultConfigFile)
	}
	return fmt.Sprintf("%d/%d aliases imported", importedNum, totalNum), nil
}
