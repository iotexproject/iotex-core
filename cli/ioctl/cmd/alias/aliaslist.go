// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// aliasListCmd represents the alias list command
var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List aliases",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(aliasList())
	},
}

func aliasList() string {
	lines := make([]string, 0)
	for alias, addr := range config.ReadConfig.Aliases {
		lines = append(lines, addr+" - "+alias)
	}
	return strings.Join(lines, "\n")
}
