// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"testing"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/spf13/cobra"
)

func Test_contractTestFunction(t *testing.T) {
	type args struct {
		client ioctl.Client
		cmd    *cobra.Command
		args   []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := contractTestFunction(tt.args.client, tt.args.cmd, tt.args.args); (err != nil) != tt.wantErr {
				t.Errorf("contractTestFunction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
