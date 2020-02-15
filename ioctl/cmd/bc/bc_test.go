// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
    "io/ioutil"
    "os"
    "testing"

    "github.com/stretchr/testify/require"
    "gopkg.in/yaml.v2"

    "github.com/iotexproject/iotex-core/ioctl/config"
)

const (
    testPath = "kstest"
)

func TestGetChainMeta(t *testing.T) {
    require := require.New(t)

    require.NoError(testInit())

    chainMeta, err := GetChainMeta()
    require.NoError(err)
    require.NotNil(chainMeta)
}

func TestGetEpochMeta(t *testing.T) {
    require := require.New(t)

    require.NoError(testInit())

    epochMeta, err := GetEpochMeta(1)
    require.NoError(err)
    require.NotNil(epochMeta)
}

func TestBcInfo(t *testing.T) {
    require := require.New(t)

    require.NoError(testInit())
    require.NoError(bcInfo())
}

func TestBcBlock(t *testing.T) {
    require := require.New(t)

    require.NoError(testInit())

    // GetBlockMetaByHeight
    blockMetaByHeight, err := GetBlockMetaByHeight(3313707)
    require.NoError(err)
    require.NotNil(blockMetaByHeight)

    // GetBlockMetaByHash
    blockMetaByHash, err := GetBlockMetaByHash(blockMetaByHeight.Hash)
    require.NoError(err)
    require.NotNil(blockMetaByHash)

    require.Equal(blockMetaByHeight, blockMetaByHash)
}

func testInit() error {
    testPathd, _ := ioutil.TempDir(os.TempDir(), testPath)
    config.ConfigDir = testPathd
    var err error
    config.DefaultConfigFile = config.ConfigDir + "/config.default"
    config.ReadConfig, err = config.LoadConfig()
    if err != nil && !os.IsNotExist(err) {
        return err
    }
    config.ReadConfig.Wallet = config.ConfigDir
    config.ReadConfig.Endpoint = "api.iotex.one:443"
    config.ReadConfig.SecureConnect = true

    out, err := yaml.Marshal(&config.ReadConfig)
    if err != nil {
        return err
    }
    if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
        return err
    }
    return nil
}