// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	serverCfg "github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Regexp patterns
const (
	_ipPattern               = `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`
	_domainPattern           = `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`
	_urlPattern              = `[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
	_localPattern            = "localhost"
	_endpointPattern         = "(" + _ipPattern + "|(" + _domainPattern + ")" + "|(" + _localPattern + "))" + `(:\d{1,5})?`
	_defaultAnalyserEndpoint = "https://iotex-analyser-api-mainnet.chainanalytics.org"
	_defaultConfigFileName   = "config.default"
)

var (
	_supportedLanguage = []string{"English", "中文"}
	_validArgs         = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height"}
	_validGetArgs      = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "analyserEndpoint", "all"}
	_validExpl         = []string{"iotexscan", "iotxplorer"}
	_endpointCompile   = regexp.MustCompile("^" + _endpointPattern + "$")
	_configDir         = os.Getenv("HOME") + "/.config/ioctl/default"
)

// info contains the information of config file
type info struct {
	readConfig        config.Config
	defaultConfigFile string // Path to config file
}

// InitConfig load config data from default config file
func InitConfig() (config.Config, string, error) {
	info := &info{
		readConfig: config.Config{
			Aliases: make(map[string]string),
		},
	}

	// Create path to config directory
	err := os.MkdirAll(_configDir, 0700)
	if err != nil {
		return info.readConfig, info.defaultConfigFile, err
	}
	info.defaultConfigFile = filepath.Join(_configDir, _defaultConfigFileName)

	// Load or reset config file
	err = info.loadConfig()
	if os.IsNotExist(err) {
		err = info.reset() // Config file doesn't exist
	} else if err != nil {
		return info.readConfig, info.defaultConfigFile, err
	}

	// Check completeness of config file
	completeness := true
	if info.readConfig.Wallet == "" {
		info.readConfig.Wallet = _configDir
		completeness = false
	}
	if info.readConfig.Language == "" {
		info.readConfig.Language = _supportedLanguage[0]
		completeness = false
	}
	if info.readConfig.Nsv2height == 0 {
		info.readConfig.Nsv2height = serverCfg.Default.Genesis.FairbankBlockHeight
	}
	if info.readConfig.AnalyserEndpoint == "" {
		info.readConfig.AnalyserEndpoint = _defaultAnalyserEndpoint
		completeness = false
	}
	if !completeness {
		if err = info.writeConfig(); err != nil {
			return info.readConfig, info.defaultConfigFile, err
		}
	}
	// Set language for ioctl
	if info.isSupportedLanguage(info.readConfig.Language) == -1 {
		fmt.Printf("Warn: Language %s is not supported, English instead.\n", info.readConfig.Language)
	}

	return info.readConfig, info.defaultConfigFile, nil
}

// newInfo create config info
func newInfo(readConfig config.Config, defaultConfigFile string) *info {
	return &info{
		readConfig:        readConfig,
		defaultConfigFile: defaultConfigFile,
	}
}

// reset resets all values of config
func (c *info) reset() error {
	c.readConfig.Wallet = path.Dir(c.defaultConfigFile)
	c.readConfig.Endpoint = ""
	c.readConfig.SecureConnect = true
	c.readConfig.DefaultAccount = *new(config.Context)
	c.readConfig.Explorer = _validExpl[0]
	c.readConfig.Language = _supportedLanguage[0]
	c.readConfig.AnalyserEndpoint = _defaultAnalyserEndpoint

	err := c.writeConfig()
	if err != nil {
		return err
	}

	fmt.Println("Config set to default values")
	return nil
}

// isSupportedLanguage checks if the language is a supported option and returns index when supported
func (c *info) isSupportedLanguage(arg string) config.Language {
	if index, err := strconv.Atoi(arg); err == nil && index >= 0 && index < len(_supportedLanguage) {
		return config.Language(index)
	}
	for i, lang := range _supportedLanguage {
		if strings.EqualFold(arg, lang) {
			return config.Language(i)
		}
	}
	return config.Language(-1)
}

// writeConfig writes to config file
func (c *info) writeConfig() error {
	out, err := yaml.Marshal(&c.readConfig)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}
	if err := os.WriteFile(c.defaultConfigFile, out, 0600); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to write to config file %s", c.defaultConfigFile))
	}
	return nil
}

// loadConfig loads config file in yaml format
func (c *info) loadConfig() error {
	in, err := os.ReadFile(c.defaultConfigFile)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(in, &c.readConfig); err != nil {
		return errors.Wrap(err, "failed to unmarshal config")
	}
	return nil
}
