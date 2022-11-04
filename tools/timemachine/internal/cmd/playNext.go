// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
)

// playNext represents the playnext command
var playNext = &cobra.Command{
	Use:   "playnext",
	Short: "Play next height of block",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		return nil
	},
}
