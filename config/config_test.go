// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"math/big"
	"os"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"


	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
)

func TestDB_SplitDBSize(t *testing.T) {
	var db = DB{SplitDBSizeMB: uint64(1)}
	var expected = uint64(1 * 1024 * 1024)
	require.Equal(t, expected, db.SplitDBSize())
}

func TestStrs_String(t *testing.T) {
	ss := strs{"test"}
	str := "TEST"
	require.Nil(t, ss.Set(str))
}

func TestNewDefaultConfig(t *testing.T) {
	_, err := New()
	require.NoError(t, err)
}

func TestNewConfigWithoutValidation(t *testing.T) {
	cfg, err := New(DoNotValidate)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	exp := Default
	exp.Network.MasterKey = cfg.Chain.ProducerPrivKey
	require.Equal(t, exp, cfg)
}

func TestNewConfigWithWrongConfigPath(t *testing.T) {
	_overwritePath = "wrong_path"
	defer func() { _overwritePath = "" }()

	cfg, err := New()
	require.Error(t, err)
	require.Equal(t, Config{}, cfg)
	if strings.Contains(err.Error(),
		"open wrong_path: The system cannot find the file specified") == false { // for Windows
		require.Contains(t, err.Error(), "open wrong_path: no such file or directory")
	}
}

func TestNewConfigWithPlugins(t *testing.T) {
	_plugins = strs{
		"gateway",
	}
	cfg, err := New()

	require.Nil(t, cfg.Plugins[GatewayPlugin])
	require.NoError(t, err)

	_plugins = strs{
		"trick",
	}

	cfg, err = New()

	require.Equal(t, Config{}, cfg)
	require.Error(t, err)

	defer func() {
		_plugins = nil
	}()
}

func TestNewConfigWithOverride(t *testing.T) {
	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
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
		require.NoError(t, err)
	}()

	cfg, err := New()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithSecret(t *testing.T) {
	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_overwritePath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_overwritePath, []byte(cfgStr), 0666)
	require.NoError(t, err)

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
		require.NoError(t, err)
		_overwritePath = ""
		err = os.Remove(_secretPath)
		require.NoError(t, err)
		_secretPath = ""
	}()

	cfg, err := New()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithLookupEnv(t *testing.T) {
	oldEnv, oldExist := os.LookupEnv("IOTEX_TEST_NODE_TYPE")

	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
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
		require.NoError(t, err)
		_overwritePath = ""
		if oldExist {
			err = os.Setenv("IOTEX_TEST_NODE_TYPE", oldEnv)
		} else {
			err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
		}
		require.NoError(t, err)
	}()

	cfg, err := New()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
	require.NoError(t, err)

	cfg, err = New()
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestValidateDispatcher(t *testing.T) {
	cfg := Default
	cfg.Dispatcher.EventChanSize = 0
	err := ValidateDispatcher(cfg)
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
	require.Equal(t, ErrInvalidCfg, errors.Cause(err))
	require.True(
		t,
		strings.Contains(
			err.Error(),
			"maximum number of actions per pool cannot be less than maximum number of actions per account",
		),
	)
}

func TestValidateMinGasPrice(t *testing.T) {
	ap := ActPool{MinGasPriceStr: Default.ActPool.MinGasPriceStr}
	mgp := ap.MinGasPrice()
	fmt.Printf("%T,%v", mgp, mgp)
	require.IsType(t, &big.Int{}, mgp)
}

func TestValidateProducerPrivateKey(t *testing.T) {
	cfg := Default
	sk := cfg.ProducerPrivateKey()
	require.NotNil(t, sk)
}

func TestValidateProducerAddress(t *testing.T) {
	cfg := Default
	addr := cfg.ProducerAddress()
	require.NotNil(t, addr)
}

func TestNewSubDefaultConfig(t *testing.T) {
	_, err := NewSub()
	require.NoError(t, err)
}

func TestNewSubConfigWithoutValidation(t *testing.T) {
	cfg, err := NewSub(DoNotValidate)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestNewSubConfigWithWrongConfigPath(t *testing.T) {
	_subChainPath = "wrong_path"
	defer func() { _subChainPath = "" }()
	cfg, err := NewSub()
	require.Error(t, err)
	require.Equal(t, Config{}, cfg)
	if strings.Contains(err.Error(),
		"open wrong_path: The system cannot find the file specified") == false { // for Windows
		require.Contains(t, err.Error(), "open wrong_path: no such file or directory")
	}
}

func TestNewSubConfigWithSubChainPath(t *testing.T) {
	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_subChainPath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_subChainPath, []byte(cfgStr), 0666)
	require.NoError(t, err)
	defer func() {
		err = os.Remove(_subChainPath)
		_subChainPath = ""
		require.NoError(t, err)
	}()

	cfg, err := NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewSubConfigWithSecret(t *testing.T) {
	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_subChainPath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_subChainPath, []byte(cfgStr), 0666)
	require.NoError(t, err)

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
		err = os.Remove(_subChainPath)
		require.NoError(t, err)
		_subChainPath = ""
		err = os.Remove(_secretPath)
		require.NoError(t, err)
		_secretPath = ""
	}()

	cfg, err := NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewSubConfigWithLookupEnv(t *testing.T) {
	oldEnv, oldExist := os.LookupEnv("IOTEX_TEST_NODE_TYPE")

	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	_subChainPath = filepath.Join(os.TempDir(), "config.yaml")
	err = ioutil.WriteFile(_subChainPath, []byte(cfgStr), 0666)
	require.NoError(t, err)

	defer func() {
		err = os.Remove(_subChainPath)
		require.NoError(t, err)
		_subChainPath = ""
		if oldExist {
			err = os.Setenv("IOTEX_TEST_NODE_TYPE", oldEnv)
		} else {
			err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
		}
		require.NoError(t, err)
	}()

	cfg, err := NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
	require.NoError(t, err)

	cfg, err = NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestNewSubConfigWithoutSubChainPath(t *testing.T) {
	_subChainPath = ""
	cfg, err := NewSub()
	require.Equal(t, Config{}, cfg)
	require.Nil(t, err)
}
