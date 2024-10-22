// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"

	serverCfg "github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/validator"
)

// Regexp patterns
const (
	_ipPattern               = `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`
	_domainPattern           = `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`
	_localPattern            = "localhost"
	_endpointPattern         = "(" + _ipPattern + "|(" + _domainPattern + ")" + "|(" + _localPattern + "))" + `(:\d{1,5})?`
	_defaultAnalyserEndpoint = "https://iotex-analyser-api-mainnet.chainanalytics.org"
	_defaultConfigFileName   = "config.default"
	// _defaultWsEndpoint default w3bstream endpoint
	_defaultWsEndpoint = "https://sprout-testnet.w3bstream.com"
	// _defaultIPFSEndpoint default IPFS endpoint for uploading
	_defaultIPFSEndpoint = "ipfs.mainnet.iotex.io"
	// _defaultIPFSGateway default IPFS gateway for resource fetching
	_defaultIPFSGateway = "https://ipfs.io"
	// _defaultWsProjectRegisterContract  default project register contract address
	_defaultWsProjectRegisterContract = "0x6325D51b6F8bC78b00c55e6233e8824231C31DE2"
	// _defaultWsProjectStoreContract  default project store contract address
	_defaultWsProjectStoreContract = "0x3522bBB40D94D5027aB585e1796a68BE003bF36b"
	// _defaultWsFleetManagementContract  default fleet management contract address
	_defaultWsFleetManagementContract = "0x7f23447c0bC51b0532EB0D2C7f2D123304666524"
	// _defaultWsProverStoreContract  default prover store contract address
	_defaultWsProverStoreContract = "0x1BCe261009e73A2300A6144d5900062De7fd8365"
	// _defaultWsProjectDevicesContract  default project device contract address
	_defaultWsProjectDevicesContract = "0xEA0B75d277AE1D13BBeAAe4537291319E2d3d1C2"
	// _defaultWsRouterContract default router contract address
	_defaultWsRouterContract = "0x749C1856D7fdF7b4a2BEBDa6c16E335CE6b33bAe"
	// _defaultWsVmTypeContract default vmType contract address
	_defaultWsVmTypeContract = "0x3C296D654d33901F8c2D08386Bf438458c89dFaB"
)

var (
	_supportedLanguage = []string{"English", "中文"}
	_validArgs         = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "wsEndpoint", "ipfsEndpoint", "ipfsGateway", "wsProjectRegisterContract", "wsProjectStoreContract", "wsFleetManagementContract", "wsProverStoreContract", "wsProjectDevicesContract", "wsRouterContract", "wsVmTypeContract"}
	_validGetArgs      = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "wsEndpoint", "ipfsEndpoint", "ipfsGateway", "analyserEndpoint", "wsProjectRegisterContract", "wsProjectStoreContract", "wsFleetManagementContract", "wsProverStoreContract", "wsProjectDevicesContract", "wsRouterContract", "wsVmTypeContract", "all"}
	_validExpl         = []string{"iotexscan", "iotxplorer"}
	_endpointCompile   = regexp.MustCompile("^" + _endpointPattern + "$")
	_configDir         = os.Getenv("HOME") + "/.config/ioctl/default"
)

// Multi-language support
var (
	_configCmdShorts = map[config.Language]string{
		config.English: "Manage the configuration of ioctl",
		config.Chinese: "ioctl配置管理",
	}
)

// NewConfigCmd represents the new node command.
func NewConfigCmd(client ioctl.Client) *cobra.Command {
	configShorts, _ := client.SelectTranslation(_configCmdShorts)

	cmd := &cobra.Command{
		Use:   "config",
		Short: configShorts,
	}
	cmd.AddCommand(NewConfigSetCmd(client))
	cmd.AddCommand(NewConfigGetCmd(client))
	cmd.AddCommand(NewConfigResetCmd(client))

	return cmd
}

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
		err = info.reset()
	}
	if err != nil {
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
	if info.readConfig.WsEndpoint == "" {
		info.readConfig.WsEndpoint = _defaultWsEndpoint
		completeness = false
	}
	if info.readConfig.IPFSEndpoint == "" {
		info.readConfig.IPFSEndpoint = _defaultIPFSEndpoint
	}
	if info.readConfig.IPFSGateway == "" {
		info.readConfig.IPFSGateway = _defaultIPFSGateway
	}
	if info.readConfig.WsProjectRegisterContract == "" {
		info.readConfig.WsProjectRegisterContract = _defaultWsProjectRegisterContract
	}
	if info.readConfig.WsProjectStoreContract == "" {
		info.readConfig.WsProjectStoreContract = _defaultWsProjectStoreContract
	}
	if info.readConfig.WsFleetManagementContract == "" {
		info.readConfig.WsFleetManagementContract = _defaultWsFleetManagementContract
	}
	if info.readConfig.WsProverStoreContract == "" {
		info.readConfig.WsProverStoreContract = _defaultWsProverStoreContract
	}
	if info.readConfig.WsProjectDevicesContract == "" {
		info.readConfig.WsProjectDevicesContract = _defaultWsProjectDevicesContract
	}
	if info.readConfig.WsRouterContract == "" {
		info.readConfig.WsRouterContract = _defaultWsRouterContract
	}
	if info.readConfig.WsVmTypeContract == "" {
		info.readConfig.WsVmTypeContract = _defaultWsVmTypeContract
	}
	if !completeness {
		if err = info.writeConfig(); err != nil {
			return info.readConfig, info.defaultConfigFile, err
		}
	}
	// Set language for ioctl
	if isSupportedLanguage(info.readConfig.Language) == -1 {
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
	c.readConfig.Wallet = filepath.Dir(c.defaultConfigFile)
	c.readConfig.Endpoint = ""
	c.readConfig.SecureConnect = true
	c.readConfig.DefaultAccount = *new(config.Context)
	c.readConfig.Explorer = _validExpl[0]
	c.readConfig.Language = _supportedLanguage[0]
	c.readConfig.AnalyserEndpoint = _defaultAnalyserEndpoint
	c.readConfig.WsEndpoint = _defaultWsEndpoint
	c.readConfig.IPFSEndpoint = _defaultIPFSEndpoint
	c.readConfig.IPFSGateway = _defaultIPFSGateway
	c.readConfig.WsProjectRegisterContract = _defaultWsProjectRegisterContract
	c.readConfig.WsProjectStoreContract = _defaultWsProjectStoreContract
	c.readConfig.WsFleetManagementContract = _defaultWsFleetManagementContract
	c.readConfig.WsProverStoreContract = _defaultWsProverStoreContract
	c.readConfig.WsProjectDevicesContract = _defaultWsProjectDevicesContract
	c.readConfig.WsRouterContract = _defaultWsRouterContract
	c.readConfig.WsVmTypeContract = _defaultWsVmTypeContract

	err := c.writeConfig()
	if err != nil {
		return err
	}

	fmt.Println("Config set to default values")
	return nil
}

// set sets config variable
func (c *info) set(args []string, insecure bool, client ioctl.Client) (string, error) {
	switch args[0] {
	case "endpoint":
		if !isValidEndpoint(args[1]) {
			return "", errors.Errorf("endpoint %s is not valid", args[1])
		}
		c.readConfig.Endpoint = args[1]
		c.readConfig.SecureConnect = !insecure
	case "analyserEndpoint":
		c.readConfig.AnalyserEndpoint = args[1]
	case "wallet":
		c.readConfig.Wallet = args[1]
	case "explorer":
		lowArg := strings.ToLower(args[1])
		switch {
		case isValidExplorer(lowArg):
			c.readConfig.Explorer = lowArg
		case args[1] == "custom":
			link, err := client.ReadCustomLink()
			if err != nil {
				return "", errors.Wrapf(err, "invalid link %s", link)
			}
			c.readConfig.Explorer = link
		default:
			return "", errors.Errorf("explorer %s is not valid\nValid explorers: %s",
				args[1], append(_validExpl, "custom"))
		}
	case "defaultacc":
		if err := validator.ValidateAlias(args[1]); err == nil {
		} else if err = validator.ValidateAddress(args[1]); err == nil {
		} else {
			return "", errors.Errorf("failed to validate alias or address %s", args[1])
		}
		c.readConfig.DefaultAccount.AddressOrAlias = args[1]
	case "language":
		lang := isSupportedLanguage(args[1])
		if lang == -1 {
			return "", errors.Errorf("language %s is not supported\nSupported languages: %s",
				args[1], _supportedLanguage)
		}
		c.readConfig.Language = _supportedLanguage[lang]
	case "nsv2height":
		height, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return "", errors.Wrapf(err, "invalid height %d", height)
		}
		c.readConfig.Nsv2height = height
	case "wsEndpoint":
		c.readConfig.WsEndpoint = args[1]
	case "ipfsEndpoint":
		c.readConfig.IPFSEndpoint = args[1]
	case "ipfsGateway":
		c.readConfig.IPFSGateway = args[1]
	case "wsProjectRegisterContract":
		c.readConfig.WsProjectRegisterContract = args[1]
	case "wsProjectStoreContract":
		c.readConfig.WsProjectStoreContract = args[1]
	case "wsFleetManagementContract":
		c.readConfig.WsFleetManagementContract = args[1]
	case "wsProverStoreContract":
		c.readConfig.WsProverStoreContract = args[1]
	case "wsProjectDevicesContract":
		c.readConfig.WsProjectDevicesContract = args[1]
	case "wsRouterContract":
		c.readConfig.WsRouterContract = args[1]
	case "wsVmTypeContract":
		c.readConfig.WsVmTypeContract = args[1]
	default:
		return "", config.ErrConfigNotMatch
	}

	err := c.writeConfig()
	if err != nil {
		return "", err
	}

	return cases.Title(language.Und).String(args[0]) + " is set to " + args[1], nil
}

// get retrieves a config item from its key.
func (c *info) get(arg string) (string, error) {
	switch arg {
	case "endpoint":
		if c.readConfig.Endpoint == "" {
			return "", config.ErrEmptyEndpoint
		}
		return fmt.Sprintf("%s secure connect(TLS): %t", c.readConfig.Endpoint, c.readConfig.SecureConnect), nil
	case "wallet":
		return c.readConfig.Wallet, nil
	case "defaultacc":
		if c.readConfig.DefaultAccount.AddressOrAlias == "" {
			return "", config.ErrConfigDefaultAccountNotSet
		}
		return jsonString(c.readConfig.DefaultAccount)
	case "explorer":
		return c.readConfig.Explorer, nil
	case "language":
		return c.readConfig.Language, nil
	case "nsv2height":
		return strconv.FormatUint(c.readConfig.Nsv2height, 10), nil
	case "analyserEndpoint":
		return c.readConfig.AnalyserEndpoint, nil
	case "wsEndpoint":
		return c.readConfig.WsEndpoint, nil
	case "ipfsEndpoint":
		return c.readConfig.IPFSEndpoint, nil
	case "ipfsGateway":
		return c.readConfig.IPFSGateway, nil
	case "wsProjectRegisterContract":
		return c.readConfig.WsProjectRegisterContract, nil
	case "wsProjectStoreContract":
		return c.readConfig.WsProjectStoreContract, nil
	case "wsFleetManagementContract":
		return c.readConfig.WsFleetManagementContract, nil
	case "wsProverStoreContract":
		return c.readConfig.WsProverStoreContract, nil
	case "wsProjectDevicesContract":
		return c.readConfig.WsProjectDevicesContract, nil
	case "wsRouterContract":
		return c.readConfig.WsRouterContract, nil
	case "wsVmTypeContract":
		return c.readConfig.WsVmTypeContract, nil
	case "all":
		return jsonString(c.readConfig)
	default:
		return "", config.ErrConfigNotMatch
	}
}

// isValidEndpoint makes sure the endpoint matches the endpoint match pattern
func isValidEndpoint(endpoint string) bool {
	return _endpointCompile.MatchString(endpoint)
}

// isValidExplorer checks if the explorer is a valid option
func isValidExplorer(arg string) bool {
	for _, exp := range _validExpl {
		if arg == exp {
			return true
		}
	}
	return false
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

// isSupportedLanguage checks if the language is a supported option and returns index when supported
func isSupportedLanguage(arg string) config.Language {
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

// jsonString returns json string for message
func jsonString(input interface{}) (string, error) {
	byteAsJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to JSON marshal config field")
	}
	return fmt.Sprint(string(byteAsJSON)), nil
}
