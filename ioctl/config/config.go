// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Directories
var (
	// ConfigDir is the directory to store config file
	ConfigDir string
	// DefaultConfigFile is the default config file name
	DefaultConfigFile string
)

// Error strings
var (
	// ErrConfigNotMatch indicates error for no config matches
	ErrConfigNotMatch = fmt.Errorf("no matching config")
	// ErrEmptyEndpoint indicates error for empty endpoint
	ErrEmptyEndpoint = fmt.Errorf("no endpoint has been set")
	// ErrConfigDefaultAccountNotSet indicates an error for the default account not being set
	ErrConfigDefaultAccountNotSet = fmt.Errorf("default account not set")
)

// Language type used to enumerate supported language of ioctl
type Language int

// Multi-language support
const (
	English Language = iota
	Chinese
)

// ConfigCmd represents the config command
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Get, set, or reset configuration for ioctl",
}

// Context represents the current context
type Context struct {
	AddressOrAlias string `json:"addressOrAlias" yaml:"addressOrAlias"`
}

// Config defines the config schema
type Config struct {
	Wallet           string            `json:"wallet" yaml:"wallet"`
	Endpoint         string            `json:"endpoint" yaml:"endpoint"`
	SecureConnect    bool              `json:"secureConnect" yaml:"secureConnect"`
	Aliases          map[string]string `json:"aliases" yaml:"aliases"`
	DefaultAccount   Context           `json:"defaultAccount" yaml:"defaultAccount"`
	Explorer         string            `json:"explorer" yaml:"explorer"`
	Language         string            `json:"language" yaml:"language"`
	Nsv2height       uint64            `json:"nsv2height" yaml:"nsv2height"`
	AnalyserEndpoint string            `json:"analyserEndpoint" yaml:"analyserEndpoint"`
}

var (
	// ReadConfig represents the current config read from local
	ReadConfig Config
	// Insecure represents the insecure connect option of grpc dial, default is false
	Insecure = false
	// UILanguage represents the language of ioctl user interface, default is 0 representing English
	UILanguage Language
)

func init() {
	ConfigDir = os.Getenv("HOME") + "/.config/ioctl/default"
	// Create path to config directory
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		log.L().Panic(err.Error())
	}
	// Path to config file
	DefaultConfigFile = ConfigDir + "/config.default"
	// Load or reset config file
	var err error
	ReadConfig, err = LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			err = reset() // Config file doesn't exist
		}
		if err != nil {
			log.L().Panic(err.Error())
		}
	}
	// Check completeness of config file
	completeness := true
	if ReadConfig.Wallet == "" {
		ReadConfig.Wallet = ConfigDir
		completeness = false
	}
	if ReadConfig.Language == "" {
		ReadConfig.Language = _supportedLanguage[0]
		completeness = false
	}
	if ReadConfig.Nsv2height == 0 {
		ReadConfig.Nsv2height = genesis.Default.FairbankBlockHeight
	}
	if ReadConfig.AnalyserEndpoint == "" {
		ReadConfig.AnalyserEndpoint = _defaultAnalyserEndpoint
		completeness = false
	}
	if !completeness {
		err := writeConfig()
		if err != nil {
			log.L().Panic(err.Error())
		}
	}
	// Set language for ioctl
	UILanguage = isSupportedLanguage(ReadConfig.Language)
	if UILanguage == -1 {
		UILanguage = 0
		message := output.StringMessage(fmt.Sprintf("Language %s is not supported, English instead.",
			ReadConfig.Language))
		fmt.Println(message.Warn())
	}
	// Init subcommands
	ConfigCmd.AddCommand(_configGetCmd)
	ConfigCmd.AddCommand(_configSetCmd)
	ConfigCmd.AddCommand(_configResetCmd)
}

// LoadConfig loads config file in yaml format
func LoadConfig() (Config, error) {
	ReadConfig := Config{
		Aliases: make(map[string]string),
	}
	in, err := os.ReadFile(DefaultConfigFile)
	if err == nil {
		if err := yaml.Unmarshal(in, &ReadConfig); err != nil {
			return ReadConfig, err
		}
	}
	return ReadConfig, err
}

// TranslateInLang returns translation in selected language
func TranslateInLang(translations map[Language]string, lang Language) string {
	if tsl, ok := translations[lang]; ok {
		return tsl
	}

	// Assumption: English should always be provided
	return translations[English]
}

// Lang returns the selected language, default is English
func (c *Config) Lang() Language {
	switch c.Language {
	case "中文":
		return Chinese
	default:
		return English
	}
}
