// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestNewDefaultConfig(t *testing.T) {
	// Default config doesn't have block producer addr setup
	cfg, err := New()
	require.NotNil(t, err)
	require.Nil(t, cfg)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
}

func TestNewConfigWithoutValidation(t *testing.T) {
	cfg, err := New(DoNotValidate)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, Default, *cfg)
}

func TestNewConfigWithWrongConfigPath(t *testing.T) {
	Path = "wrong_path"
	defer func() { Path = "" }()

	cfg, err := New()
	require.NotNil(t, err)
	require.Nil(t, cfg)
	require.Equal(t, "open wrong_path: no such file or directory", err.Error())
}

func TestNewConfigWithOverride(t *testing.T) {
	cfgStr := fmt.Sprintf(`
nodeType: %s
chain:
    producerPrivKey: "%s"
    producerPubKey: "%s"
`,
		DelegateType,
		hex.EncodeToString(testaddress.Addrinfo["alfa"].PrivateKey),
		hex.EncodeToString(testaddress.Addrinfo["alfa"].PublicKey),
	)
	Path = filepath.Join(os.TempDir(), "config.yaml")
	ioutil.WriteFile(Path, []byte(cfgStr), 0666)
	defer func() {
		err := os.Remove(Path)
		Path = ""
		require.Nil(t, err)
	}()

	cfg, err := New()
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, DelegateType, cfg.NodeType)
	require.Equal(t, hex.EncodeToString(testaddress.Addrinfo["alfa"].PrivateKey), cfg.Chain.ProducerPrivKey)
	require.Equal(t, hex.EncodeToString(testaddress.Addrinfo["alfa"].PublicKey), cfg.Chain.ProducerPubKey)
}

func TestValidateAddr(t *testing.T) {
	cfg := Default
	cfg.Chain.ProducerPubKey = "hello world"
	cfg.Chain.ProducerPrivKey = "world hello"
	err := ValidateAddr(&cfg)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "encoding/hex:"), err.Error())

	cfg.Chain.ProducerPubKey = hex.EncodeToString(testaddress.Addrinfo["alfa"].PublicKey)
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(testaddress.Addrinfo["bravo"].PrivateKey)
	err = ValidateAddr(&cfg)
	assert.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "block producer has unmatched pubkey and prikey"),
	)
}

func TestValidateExplorer(t *testing.T) {
	cfg := Default
	cfg.Explorer.Enabled = true
	cfg.Explorer.TpsWindow = 0
	err := ValidateExplorer(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "tps window is not a positive integer when the explorer is enabled"),
	)
}

func TestValidateConsensusScheme(t *testing.T) {
	cfg := Default
	cfg.NodeType = FullNodeType
	cfg.Consensus.Scheme = RollDPoSScheme
	err := ValidateConsensusScheme(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "consensus scheme of fullnode should be NOOP"),
	)

	cfg.NodeType = LightweightType
	err = ValidateConsensusScheme(&cfg)
	assert.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "consensus scheme of lightweight node should be NOOP"),
	)

	cfg.NodeType = "Unknown"
	err = ValidateConsensusScheme(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "unknown node type"),
	)
}

func TestValidateDispatcher(t *testing.T) {
	cfg := Default
	cfg.Dispatcher.EventChanSize = 0
	err := ValidateDispatcher(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "dispatcher event chan size should be greater than 0"),
	)
}

func TestValidateRollDPoS(t *testing.T) {
	cfg := Default
	cfg.NodeType = DelegateType
	cfg.Consensus.Scheme = RollDPoSScheme
	cfg.Consensus.RollDPoS.EventChanSize = 0
	err := ValidateRollDPoS(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "roll-DPoS event chan size should be greater than 0"),
	)
}

func TestValidateNetwork(t *testing.T) {
	cfg := Default
	cfg.Network.PeerDiscovery = false
	err := ValidateNetwork(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "either peer discover should be enabled or a topology should be given"),
	)
}

func TestValidateDelegate(t *testing.T) {
	cfg := Default
	cfg.Delegate.RollNum = 2
	err := ValidateDelegate(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "rolling delegates number is greater than total configured delegates"),
	)
}

func TestValidateActPool(t *testing.T) {
	cfg := Default
	cfg.ActPool.MaxNumActPerAcct = 0
	err := ValidateActPool(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool or per account cannot be zero or negative",
		),
	)

	cfg.ActPool.MaxNumActPerAcct = 100
	cfg.ActPool.MaxNumActPerPool = 0
	err = ValidateActPool(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool or per account cannot be zero or negative",
		),
	)

	cfg.ActPool.MaxNumActPerPool = 99
	err = ValidateActPool(&cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool cannot be less than maximum number of actions per account",
		),
	)
}

func TestCheckNodeType(t *testing.T) {
	cfg := Default
	require.True(t, cfg.IsFullnode())
	require.False(t, cfg.IsDelegate())
	require.False(t, cfg.IsLightweight())

	cfg.NodeType = DelegateType
	require.False(t, cfg.IsFullnode())
	require.True(t, cfg.IsDelegate())
	require.False(t, cfg.IsLightweight())

	cfg.NodeType = LightweightType
	require.False(t, cfg.IsFullnode())
	require.False(t, cfg.IsDelegate())
	require.True(t, cfg.IsLightweight())
}
