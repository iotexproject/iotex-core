// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_listCmdShorts = map[config.Language]string{
		config.English: "List aliases",
		config.Chinese: "列出别名",
	}
)

// _aliasListCmd represents the alias list command
var _aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: config.TranslateInLang(_listCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		aliasList()
	},
}

type aliasListMessage struct {
	AliasNumber int     `json:"aliasNumber"`
	AliasList   []alias `json:"aliasList"`
}

func aliasList() {
	var keys []string
	for name := range config.ReadConfig.Aliases {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	message := aliasListMessage{AliasNumber: len(keys)}
	for _, name := range keys {
		aliasMeta := alias{Address: config.ReadConfig.Aliases[name], Name: name}
		message.AliasList = append(message.AliasList, aliasMeta)
	}
	fmt.Println(message.String())
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
