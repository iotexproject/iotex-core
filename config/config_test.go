// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/keypair"
)

func TestNewDefaultConfig(t *testing.T) {
	_, err := New()
	require.Nil(t, err)
}

func TestNewConfigWithoutValidation(t *testing.T) {
	cfg, err := New(DoNotValidate)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	exp := Default
	exp.Network.MasterKey = cfg.Chain.ProducerPrivKey
	require.Equal(t, exp, cfg)
}

func TestNewConfigWithWrongConfigPath(t *testing.T) {
	_overwritePath = "wrong_path"
	defer func() { _overwritePath = "" }()

	cfg, err := New()
	require.NotNil(t, err)
	require.Equal(t, Config{}, cfg)
	require.Contains(t, err.Error(), "open wrong_path: no such file or directory")
}

func TestNewConfigWithOverride(t *testing.T) {
	sk, err := keypair.GenerateKey()
	require.Nil(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_overwritePath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_overwritePath, []byte(cfgStr), 0666)
	require.NoError(t, err)
	defer func() {
		err := os.Remove(_overwritePath)
		_overwritePath = ""
		require.Nil(t, err)
	}()

	cfg, err := New()
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithSecret(t *testing.T) {
	sk, err := keypair.GenerateKey()
	require.Nil(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_overwritePath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_overwritePath, []byte(cfgStr), 0666)
	require.NoError(t, err)
	defer func() {
	}()

	cfgStr = fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_secretPath = filepath.Join(os.TempDir(), "secret.yaml")
	err = ioutil.WriteFile(_secretPath, []byte(cfgStr), 0666)
	require.NoError(t, err)

	defer func() {
		err := os.Remove(_overwritePath)
		require.Nil(t, err)
		_overwritePath = ""
		err = os.Remove(_secretPath)
		require.Nil(t, err)
		_secretPath = ""
	}()

	cfg, err := New()
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithLookupEnv(t *testing.T) {
	oldEnv, oldExist := os.LookupEnv("IOTEX_TEST_NODE_TYPE")

	sk, err := keypair.GenerateKey()
	require.Nil(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_overwritePath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_overwritePath, []byte(cfgStr), 0666)
	require.NoError(t, err)

	defer func() {
		err := os.Remove(_overwritePath)
		require.Nil(t, err)
		_overwritePath = ""
		if oldExist {
			err = os.Setenv("IOTEX_TEST_NODE_TYPE", oldEnv)
		} else {
			err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
		}
		require.Nil(t, err)
	}()

	cfg, err := New()
	require.Nil(t, err)
	require.NotNil(t, cfg)

	err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
	require.Nil(t, err)

	cfg, err = New()
	require.Nil(t, err)
	require.NotNil(t, cfg)
}

func TestValidateExplorer(t *testing.T) {
	cfg := Default
	cfg.Explorer.Enabled = true
	cfg.Explorer.TpsWindow = 0
	err := ValidateExplorer(cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "tps window is not a positive integer when the explorer is enabled"),
	)
}

func TestValidateDispatcher(t *testing.T) {
	cfg := Default
	cfg.Dispatcher.EventChanSize = 0
	err := ValidateDispatcher(cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "dispatcher event chan size should be greater than 0"),
	)
}

func TestValidateRollDPoS(t *testing.T) {
	cfg := Default
	cfg.Consensus.Scheme = RollDPoSScheme

	cfg.Consensus.RollDPoS.FSM.EventChanSize = 0
	err := ValidateRollDPoS(cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(err.Error(), "roll-DPoS event chan size should be greater than 0"),
	)
}

func TestValidateActPool(t *testing.T) {
	cfg := Default
	cfg.ActPool.MaxNumActsPerAcct = 0
	err := ValidateActPool(cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool or per account cannot be zero or negative",
		),
	)

	cfg.ActPool.MaxNumActsPerAcct = 100
	cfg.ActPool.MaxNumActsPerPool = 0
	err = ValidateActPool(cfg)
	require.NotNil(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool or per account cannot be zero or negative",
		),
	)

	cfg.ActPool.MaxNumActsPerPool = 99
	err = ValidateActPool(cfg)
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
