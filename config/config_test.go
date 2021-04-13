// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
)

const (
	overwritePath = "_overwritePath"
	secretPath    = "_secretPath"
	subChainPath  = "_subChainPath"
)

func makePathAndWriteFile(cfgStr, flagForPath string) (err error) {
	switch flagForPath {
	case overwritePath:
		_overwritePath = filepath.Join(os.TempDir(), "config.yaml")
		err = ioutil.WriteFile(_overwritePath, []byte(cfgStr), 0666)
	case secretPath:
		_secretPath = filepath.Join(os.TempDir(), "secret.yaml")
		err = ioutil.WriteFile(_secretPath, []byte(cfgStr), 0666)
	case subChainPath:
		_subChainPath = filepath.Join(os.TempDir(), "config.yaml")
		err = ioutil.WriteFile(_subChainPath, []byte(cfgStr), 0666)
	}
	return err
}

func resetPathValues(t *testing.T, flagForPath []string) {
	for _, pathValue := range flagForPath {
		switch pathValue {
		case overwritePath:
			err := os.Remove(_overwritePath)
			_overwritePath = ""
			require.NoError(t, err)
		case secretPath:
			err := os.Remove(_secretPath)
			_secretPath = ""
			require.NoError(t, err)
		case subChainPath:
			err := os.Remove(_subChainPath)
			_subChainPath = ""
			require.NoError(t, err)
		}
	}
}

func resetPathValuesWithLookupEnv(t *testing.T, oldEnv string, oldExist bool, flagForPath string) {
	switch flagForPath {
	case overwritePath:
		err := os.Remove(_overwritePath)
		require.NoError(t, err)
		_overwritePath = ""
		if oldExist {
			err = os.Setenv("IOTEX_TEST_NODE_TYPE", oldEnv)
		} else {
			err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
		}
		require.NoError(t, err)
	case subChainPath:
		err := os.Remove(_subChainPath)
		require.NoError(t, err)
		_subChainPath = ""
		if oldExist {
			err = os.Setenv("IOTEX_TEST_NODE_TYPE", oldEnv)
		} else {
			err = os.Unsetenv("IOTEX_TEST_NODE_TYPE")
		}
		require.NoError(t, err)
	}
}

func generateProducerPrivKey() (crypto.PrivateKey, string, error) {
	sk, err := crypto.GenerateKey()
	cfgStr := fmt.Sprintf(`
chain:
    producerPrivKey: "%s"
`,
		sk.HexString(),
	)
	return sk, cfgStr, err
}

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
	cfg, err := New()
	require.NoError(t, err)
	require.Equal(t, cfg.Chain.EVMNetworkID, EVMNetworkID())
	require.Equal(t, cfg.Genesis.Timestamp, GenesisTimestamp())
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
	sk, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)

	require.NoError(t, makePathAndWriteFile(cfgStr, "_overwritePath"))

	defer resetPathValues(t, []string{"_overwritePath"})

	cfg, err := New()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithSecret(t *testing.T) {
	sk, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)

	require.NoError(t, makePathAndWriteFile(cfgStr, "_overwritePath"))

	require.NoError(t, makePathAndWriteFile(cfgStr, "_secretPath"))

	defer resetPathValues(t, []string{"_overwritePath", "_secretPath"})

	cfg, err := New()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewConfigWithLookupEnv(t *testing.T) {
	oldEnv, oldExist := os.LookupEnv("IOTEX_TEST_NODE_TYPE")

	_, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)
	require.NoError(t, makePathAndWriteFile(cfgStr, "_overwritePath"))

	defer resetPathValuesWithLookupEnv(t, oldEnv, oldExist, "_overwritePath")

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

func TestValidateArchiveMode(t *testing.T) {
	cfg := Default
	cfg.Chain.EnableArchiveMode = true
	cfg.Chain.EnableTrielessStateDB = true
	require.Error(t, ErrInvalidCfg, errors.Cause(ValidateArchiveMode(cfg)))
	require.EqualError(t, ValidateArchiveMode(cfg), "Archive mode is incompatible with trieless state DB: invalid config value")
	cfg.Chain.EnableArchiveMode = false
	cfg.Chain.EnableTrielessStateDB = true
	require.NoError(t, errors.Cause(ValidateArchiveMode(cfg)))
	cfg.Chain.EnableArchiveMode = true
	cfg.Chain.EnableTrielessStateDB = false
	require.NoError(t, errors.Cause(ValidateArchiveMode(cfg)))
	cfg.Chain.EnableArchiveMode = false
	cfg.Chain.EnableTrielessStateDB = false
	require.NoError(t, errors.Cause(ValidateArchiveMode(cfg)))
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

func TestValidateForkHeights(t *testing.T) {
	r := require.New(t)

	tests := []struct {
		fork   string
		err    error
		errMsg string
	}{
		{
			"Pacific", ErrInvalidCfg, "Pacific is heigher than Aleutian",
		},
		{
			"Aleutian", ErrInvalidCfg, "Aleutian is heigher than Bering",
		},
		{
			"Bering", ErrInvalidCfg, "Bering is heigher than Cook",
		},
		{
			"Cook", ErrInvalidCfg, "Cook is heigher than Dardanelles",
		},
		{
			"Dardanelles", ErrInvalidCfg, "Dardanelles is heigher than Daytona",
		},
		{
			"Daytona", ErrInvalidCfg, "Daytona is heigher than Easter",
		},
		{
			"Easter", ErrInvalidCfg, "Easter is heigher than FairbankMigration",
		},
		{
			"FbkMigration", ErrInvalidCfg, "FairbankMigration is heigher than Fairbank",
		},
		{
			"Fairbank", ErrInvalidCfg, "Fairbank is heigher than Greenland",
		},
		{
			"", nil, "",
		},
	}

	for _, v := range tests {
		cfg := newTestCfg(v.fork)
		err := ValidateForkHeights(cfg)
		r.Equal(v.err, errors.Cause(err))
		if err != nil {
			r.True(strings.Contains(err.Error(), v.errMsg))
		}
	}
}

func newTestCfg(fork string) Config {
	cfg := Default
	switch fork {
	case "Pacific":
		cfg.Genesis.PacificBlockHeight = cfg.Genesis.AleutianBlockHeight + 1
	case "Aleutian":
		cfg.Genesis.AleutianBlockHeight = cfg.Genesis.BeringBlockHeight + 1
	case "Bering":
		cfg.Genesis.BeringBlockHeight = cfg.Genesis.CookBlockHeight + 1
	case "Cook":
		cfg.Genesis.CookBlockHeight = cfg.Genesis.DardanellesBlockHeight + 1
	case "Dardanelles":
		cfg.Genesis.DardanellesBlockHeight = cfg.Genesis.DaytonaBlockHeight + 1
	case "Daytona":
		cfg.Genesis.DaytonaBlockHeight = cfg.Genesis.EasterBlockHeight + 1
	case "Easter":
		cfg.Genesis.EasterBlockHeight = cfg.Genesis.FbkMigrationBlockHeight + 1
	case "FbkMigration":
		cfg.Genesis.FbkMigrationBlockHeight = cfg.Genesis.FairbankBlockHeight + 1
	case "Fairbank":
		cfg.Genesis.FairbankBlockHeight = cfg.Genesis.GreenlandBlockHeight + 1
	}
	return cfg
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
	sk, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)
	require.NoError(t, makePathAndWriteFile(cfgStr, "_subChainPath"))

	defer resetPathValues(t, []string{"_subChainPath"})
	cfg, err := NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewSubConfigWithSecret(t *testing.T) {
	sk, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)
	require.NoError(t, makePathAndWriteFile(cfgStr, "_subChainPath"))

	require.NoError(t, makePathAndWriteFile(cfgStr, "_secretPath"))

	defer resetPathValues(t, []string{"_subChainPath", "_secretPath"})

	cfg, err := NewSub()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, sk.HexString(), cfg.Chain.ProducerPrivKey)
}

func TestNewSubConfigWithLookupEnv(t *testing.T) {
	oldEnv, oldExist := os.LookupEnv("IOTEX_TEST_NODE_TYPE")

	_, cfgStr, err := generateProducerPrivKey()
	require.NoError(t, err)

	require.NoError(t, makePathAndWriteFile(cfgStr, "_subChainPath"))

	defer resetPathValuesWithLookupEnv(t, oldEnv, oldExist, "_subChainPath")

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

func TestWhitelist(t *testing.T) {
	require := require.New(t)

	cfg, err := NewSub()
	require.NoError(err)
	require.NotNil(cfg)

	sk, err := crypto.HexStringToPrivateKey("308193020100301306072a8648ce3d020106082a811ccf5501822d0479307702010104202d57ec7da578b98dad465997748ed02af0c69092ad809598073e5a2356c20492a00a06082a811ccf5501822da14403420004223356f0c6f40822ade24d47b0cd10e9285402cbc8a5028a8eec9efba44b8dfe1a7e8bc44953e557b32ec17039fb8018a58d48c8ffa54933fac8030c9a169bf6")
	require.NoError(err)
	require.False(cfg.whitelistSignatureScheme(sk))
	cfg.Chain.ProducerPrivKey = sk.HexString()
	require.Panics(func() { cfg.ProducerPrivateKey() })

	cfg.Chain.SignatureScheme = append(cfg.Chain.SignatureScheme, SigP256sm2)
	require.Equal(sk, cfg.ProducerPrivateKey())
}
