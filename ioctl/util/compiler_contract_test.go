// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testSource = `
pragma solidity >0.0.0;
contract test {
   /// @notice Will multiply ` + "`a`" + ` by 7.
   function multiply(uint a) public returns(uint d) {
       return a * 7;
   }
}
`
)

func skipWithoutSolc(t *testing.T) {
	if _, err := exec.LookPath("solc"); err != nil {
		t.Skip(err)
	}
}

func TestSolidityCompiler(t *testing.T) {
	skipWithoutSolc(t)

	require := require.New(t)
	contracts, err := CompileSolidityString("", testSource)
	require.NoError(err)
	require.Len(contracts, 1)

	c, ok := contracts["Test"]
	require.False(ok)
	c, ok = contracts["<stdin>:test"]
	require.True(ok)

	require.True(ok)
	require.NotEmpty(c.Code)
	require.Equal(testSource, c.Info.Source)
	require.NotEmpty(c.Info.LanguageVersion)

	contracts, err = CompileSolidityString("", testSource[4:])
	require.Error(err)
	require.Contains(err.Error(), "exit status")
}
