// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// walletListCmd represents the wallet list command
var walletListCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing wallet for ioctl",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(walletList())
	},
}

func walletList() string {
	file, err := ioutil.ReadFile(config.DefaultConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			// special case of empty config file just being created
			return "No wallet found"
		}
		return fmt.Sprintf("failed to open config file %s", config.DefaultConfigFile)
	}
	// parse the wallet section from config file
	start, end, find, _ := parseConfig(file, walletPrefix, walletEnd, "")
	if !find {
		return "No wallet found"
	}
	lines := strings.Split(string(file), "\n")
	return strings.Join(lines[start+1:end], "\n")
}
