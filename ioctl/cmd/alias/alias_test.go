// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

func TestAlias(t *testing.T) {
	require := require.New(t)

	require.NoError(testInit())

	raullen := "raullen"
	qevan := "qevan"
	jing := "jing"
	frankonly := "ifyouraliasistoolongthealiassetcommandmayfailwhenrunningmycode"
	aliasTestCase := [][]string{
		{qevan, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"},
		{frankonly, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"},
		{qevan, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1"},
		{raullen, "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"},
		{jing, "io188fptstp82y53l3x0eadfhxg6qmywgny24mgfp"},
	}
	expected := []string{
		"",
		validator.ErrLongAlias.Error(),
		validator.ErrInvalidAddr.Error(),
		"",
		"",
	}
	for i, testCase := range aliasTestCase {
		err := set(testCase)
		if err != nil {
			require.Contains(err.Error(), expected[i])
		}
	}
	require.Equal("io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx", config.ReadConfig.Aliases[raullen])
	require.Equal("", config.ReadConfig.Aliases[qevan])
	require.Equal("io188fptstp82y53l3x0eadfhxg6qmywgny24mgfp", config.ReadConfig.Aliases[jing])
	aliases := GetAliasMap()
	require.Equal(raullen, aliases["io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"])
	require.Equal(jing, aliases["io188fptstp82y53l3x0eadfhxg6qmywgny24mgfp"])
	require.Equal("", aliases["io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1"])
	require.NoError(remove(raullen))
	require.Equal("", config.ReadConfig.Aliases[raullen])
	require.NoError(set([]string{jing, "io1kmpejl35lys5pxcpk74g8am0kwmzwwuvsvqrp8"}))
	require.Equal("io1kmpejl35lys5pxcpk74g8am0kwmzwwuvsvqrp8", config.ReadConfig.Aliases[jing])
	aliases = GetAliasMap()
	require.Equal("", aliases["io188fptstp82y53l3x0eadfhxg6qmywgny24mgfp"])
	require.Equal(jing, aliases["io1kmpejl35lys5pxcpk74g8am0kwmzwwuvsvqrp8"])
}

func testInit() error {
	testPathd, _ := os.MkdirTemp(os.TempDir(), "kstest")
	config.ConfigDir = testPathd
	var err error
	config.DefaultConfigFile = config.ConfigDir + "/config.default"
	config.ReadConfig, err = config.LoadConfig()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	config.ReadConfig.Wallet = config.ConfigDir
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return err
	}
	return nil
}
