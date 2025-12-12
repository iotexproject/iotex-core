// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
	// WsEndpoint w3bstream endpoint
	WsEndpoint string `json:"wsEndpoint" yaml:"wsEndpoint"`
	// IPFSEndpoint ipfs endpoint for uploading
	IPFSEndpoint string `json:"ipfsEndpoint" yaml:"ipfsEndpoint"`
	// IPFSGateway ipfs gateway for resource fetching (with scheme)
	IPFSGateway string `json:"ipfsGateway" yaml:"ipfsGateway"`
	// WsProjectRegisterContract w3bstream project register contract address
	WsProjectRegisterContract string `json:"wsProjectRegisterContract" yaml:"wsProjectRegisterContract"`
	// WsProjectStoreContract w3bstream project store contract address
	WsProjectStoreContract string `json:"wsProjectStoreContract" yaml:"wsProjectStoreContract"`
	// WsFleetManagementContract w3bstream fleet management contract address
	WsFleetManagementContract string `json:"wsFleetManagementContract" yaml:"wsFleetManagementContract"`
	// WsProverStoreContract w3bstream Prover store contract address
	WsProverStoreContract string `json:"wsProverStoreContract" yaml:"wsProverStoreContract"`
	// WsProjectDevicesContract w3bstream Project devices contract address
	WsProjectDevicesContract string `json:"wsProjectDevicesContract" yaml:"wsProjectDevicesContract"`
	// WsRouterContract w3bstream Router contract address
	WsRouterContract string `json:"wsRouterContract" yaml:"wsRouterContract"`
	// WsVmTypeContract w3bstream VMType contract address
	WsVmTypeContract string `json:"wsVmTypeContract" yaml:"wsVmTypeContract"`
	// IoidProjectRegisterContract is the ioID project register contract address
	IoidProjectRegisterContract string `json:"ioidProjectRegisterContract" yaml:"ioidProjectRegisterContract"`
	// IoidProjectStoreContract is the ioID project store contract address
	IoidProjectStoreContract string `json:"ioidProjectStoreContract" yaml:"ioidProjectStoreContract"`
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
		completeness = false
	}
	if ReadConfig.Endpoint == "" {
		ReadConfig.Endpoint = _defaultEndpoint
		ReadConfig.SecureConnect = true
		completeness = false
	}
	if ReadConfig.AnalyserEndpoint == "" {
		ReadConfig.AnalyserEndpoint = _defaultAnalyserEndpoint
		completeness = false
	}
	if ReadConfig.WsEndpoint == "" {
		ReadConfig.WsEndpoint = _defaultWsEndpoint
		completeness = false
	}
	if ReadConfig.IPFSEndpoint == "" {
		ReadConfig.IPFSEndpoint = _defaultIPFSEndpoint
		completeness = false
	}
	if ReadConfig.IPFSGateway == "" {
		ReadConfig.IPFSGateway = _defaultIPFSGateway
		completeness = false
	}
	if ReadConfig.WsProjectRegisterContract == "" {
		ReadConfig.WsProjectRegisterContract = _defaultWsProjectRegisterContract
		completeness = false
	}
	if ReadConfig.WsProjectStoreContract == "" {
		ReadConfig.WsProjectStoreContract = _defaultWsProjectStoreContract
		completeness = false
	}
	if ReadConfig.WsFleetManagementContract == "" {
		ReadConfig.WsFleetManagementContract = _defaultWsFleetManagementContract
		completeness = false
	}
	if ReadConfig.WsProverStoreContract == "" {
		ReadConfig.WsProverStoreContract = _defaultWsProverStoreContract
		completeness = false
	}
	if ReadConfig.WsProjectDevicesContract == "" {
		ReadConfig.WsProjectDevicesContract = _defaultWsProjectDevicesContract
		completeness = false
	}
	if ReadConfig.WsRouterContract == "" {
		ReadConfig.WsRouterContract = _defaultWsRouterContract
		completeness = false
	}
	if ReadConfig.WsVmTypeContract == "" {
		ReadConfig.WsVmTypeContract = _defaultWsVmTypeContract
		completeness = false
	}
	if ReadConfig.IoidProjectRegisterContract == "" {
		ReadConfig.IoidProjectRegisterContract = _defaultIoidProjectRegisterContract
	}
	if ReadConfig.IoidProjectStoreContract == "" {
		ReadConfig.IoidProjectStoreContract = _defaultIoidProjectStoreContract
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
	in, err := os.ReadFile(filepath.Clean(DefaultConfigFile))
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
