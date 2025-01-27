// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/validator"
)

// Regexp patterns
const (
	_ipPattern               = `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`
	_domainPattern           = `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`
	_urlPattern              = `[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
	_localPattern            = "localhost"
	_endpointPattern         = "(" + _ipPattern + "|(" + _domainPattern + ")" + "|(" + _localPattern + "))" + `(:\d{1,5})?`
	_defaultEndpoint         = "api.iotex.one:443"
	_defaultAnalyserEndpoint = "https://iotex-analyser-api-mainnet.chainanalytics.org"
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
	// _defaultWsProjectDevicesContract  default project devices contract address
	_defaultWsProjectDevicesContract = "0xEA0B75d277AE1D13BBeAAe4537291319E2d3d1C2"
	// _defaultWsRouterContract default router contract address
	_defaultWsRouterContract = "0x749C1856D7fdF7b4a2BEBDa6c16E335CE6b33bAe"
	// _defaultWsVmTypeContract default vmType contract address
	_defaultWsVmTypeContract = "0x3C296D654d33901F8c2D08386Bf438458c89dFaB"
	// _defaultIoidProjectRegisterContract is the default ioID project register contract address
	_defaultIoidProjectRegisterContract = "0x601B655c0a20FA1465C9a18e39387A33eEe7F777"
	// _defaultIoidProjectStoreContract is the default ioID project store contract address
	_defaultIoidProjectStoreContract = "0xa822Fd390e8eD3FEC80Bd26c77DD036935463b5E"
)

var (
	_supportedLanguage = []string{"English", "中文"}
	_validArgs         = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "wsEndpoint", "ipfsEndpoint", "ipfsGateway", "wsProjectRegisterContract", "wsProjectStoreContract", "wsFleetManagementContract", "wsProverStoreContract", "wsProjectDevicesContract", "wsRouterContract", "wsVmTypeContract"}
	_validGetArgs      = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "analyserEndpoint", "wsEndpoint", "ipfsEndpoint", "ipfsGateway", "wsProjectRegisterContract", "wsProjectStoreContract", "wsFleetManagementContract", "wsProverStoreContract", "wsProjectDevicesContract", "wsRouterContract", "wsVmTypeContract", "all"}
	_validExpl         = []string{"iotexscan", "iotxplorer"}
	_endpointCompile   = regexp.MustCompile("^" + _endpointPattern + "$")
)

// _configGetCmd represents the config get command
var _configGetCmd = &cobra.Command{
	Use:       "get VARIABLE",
	Short:     "Get config fields from ioctl",
	Long:      "Get config fields from ioctl\nValid Variables: [" + strings.Join(_validGetArgs, ", ") + "]",
	ValidArgs: _validGetArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d\n"+
				"Valid arg(s): %s", len(args), _validGetArgs)
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := Get(args[0])
		return output.PrintError(err)
	},
}

// _configSetCmd represents the config set command
var _configSetCmd = &cobra.Command{
	Use:       "set VARIABLE VALUE",
	Short:     "Set config fields for ioctl",
	Long:      "Set config fields for ioctl\nValid Variables: [" + strings.Join(_validArgs, ", ") + "]",
	ValidArgs: _validArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("accepts 2 arg(s), received %d\n"+
				"Valid arg(s): %s", len(args), _validArgs)
		}
		return cobra.OnlyValidArgs(cmd, args[:1])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := set(args)
		return output.PrintError(err)
	},
}

// _configResetCmd represents the config reset command
var _configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset config to default",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := reset()
		return output.PrintError(err)
	},
}

type endpointMessage struct {
	Endpoint      string `json:"endpoint"`
	SecureConnect bool   `json:"secureConnect"`
}

func (m *endpointMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprint(m.Endpoint, "    secure connect(TLS):", m.SecureConnect)
		return message
	}
	return output.FormatString(output.Result, m)
}

func (m *Context) String() string {
	if output.Format == "" {
		message := output.JSONString(m)
		return message
	}
	return output.FormatString(output.Result, m)
}

func (m *Config) String() string {
	if output.Format == "" {
		message := output.JSONString(m)
		return message
	}
	return output.FormatString(output.Result, m)
}

func init() {
	_configSetCmd.Flags().BoolVar(&Insecure, "insecure", false,
		"set insecure connection as default")
}

// Get gets config variable
func Get(arg string) error {
	switch arg {
	default:
		return output.NewError(output.ConfigError, ErrConfigNotMatch.Error(), nil)
	case "endpoint":
		if ReadConfig.Endpoint == "" {
			return output.NewError(output.ConfigError, ErrEmptyEndpoint.Error(), nil)
		}
		message := endpointMessage{Endpoint: ReadConfig.Endpoint, SecureConnect: ReadConfig.SecureConnect}
		fmt.Println(message.String())
	case "wallet":
		output.PrintResult(ReadConfig.Wallet)
	case "defaultacc":
		if ReadConfig.DefaultAccount.AddressOrAlias == "" {
			return output.NewError(output.ConfigError, "default account did not set", nil)
		}
		fmt.Println(ReadConfig.DefaultAccount.String())
	case "explorer":
		output.PrintResult(ReadConfig.Explorer)
	case "language":
		output.PrintResult(ReadConfig.Language)
	case "nsv2height":
		fmt.Println(ReadConfig.Nsv2height)
	case "analyserEndpoint":
		fmt.Println(ReadConfig.AnalyserEndpoint)
	case "wsEndpoint":
		fmt.Println(ReadConfig.WsEndpoint)
	case "ipfsEndpoint":
		fmt.Println(ReadConfig.IPFSEndpoint)
	case "ipfsGateway":
		fmt.Println(ReadConfig.IPFSGateway)
	case "wsProjectRegisterContract":
		fmt.Println(ReadConfig.WsProjectRegisterContract)
	case "wsProjectStoreContract":
		fmt.Println(ReadConfig.WsProjectStoreContract)
	case "wsFleetManagementContract":
		fmt.Println(ReadConfig.WsFleetManagementContract)
	case "wsProverStoreContract":
		fmt.Println(ReadConfig.WsProverStoreContract)
	case "wsProjectDevicesContract":
		fmt.Println(ReadConfig.WsProjectDevicesContract)
	case "wsRouterContract":
		fmt.Println(ReadConfig.WsRouterContract)
	case "wsVmTypeContract":
		fmt.Println(ReadConfig.WsVmTypeContract)
	case "all":
		fmt.Println(ReadConfig.String())
	}
	return nil
}

// GetContextAddressOrAlias gets current context
func GetContextAddressOrAlias() (string, error) {
	defaultAccount := ReadConfig.DefaultAccount
	if strings.EqualFold(defaultAccount.AddressOrAlias, "") {
		return "", output.NewError(output.ConfigError,
			`use "ioctl config set defaultacc ADDRESS|ALIAS" to config default account first`, nil)
	}
	return defaultAccount.AddressOrAlias, nil
}

// GetAddressOrAlias gets address from args or context
func GetAddressOrAlias(in string) (address string, err error) {
	if !strings.EqualFold(in, "") {
		address = in
	} else {
		address, err = GetContextAddressOrAlias()
	}
	return
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

// isSupportedLanguage checks if the language is a supported option and returns index when supported
func isSupportedLanguage(arg string) Language {
	if index, err := strconv.Atoi(arg); err == nil && index >= 0 && index < len(_supportedLanguage) {
		return Language(index)
	}
	for i, lang := range _supportedLanguage {
		if strings.EqualFold(arg, lang) {
			return Language(i)
		}
	}
	return Language(-1)
}

// writeConfig writes to config file
func writeConfig() error {
	out, err := yaml.Marshal(&ReadConfig)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := os.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", DefaultConfigFile), err)
	}
	return nil
}

// set sets config variable
func set(args []string) error {
	switch args[0] {
	default:
		return output.NewError(output.ConfigError, ErrConfigNotMatch.Error(), nil)
	case "endpoint":
		if !isValidEndpoint(args[1]) {
			return output.NewError(output.ConfigError, fmt.Sprintf("endpoint %s is not valid", args[1]), nil)
		}
		ReadConfig.Endpoint = args[1]
		ReadConfig.SecureConnect = !Insecure
	case "analyserEndpoint":
		ReadConfig.AnalyserEndpoint = args[1]
	case "wallet":
		ReadConfig.Wallet = args[1]
	case "explorer":
		lowArg := strings.ToLower(args[1])
		switch {
		case isValidExplorer(lowArg):
			ReadConfig.Explorer = lowArg
		case args[1] == "custom":
			output.PrintQuery(`Please enter a custom link below:("Example: iotexscan.io/action/")`)
			var link string
			if _, err := fmt.Scanln(&link); err != nil {
				return output.NewError(output.InputError, "failed to input link", err)
			}
			match, err := regexp.MatchString(_urlPattern, link)
			if err != nil {
				return output.NewError(output.UndefinedError, "failed to validate link", nil)
			}
			if match {
				ReadConfig.Explorer = link
			} else {
				return output.NewError(output.ValidationError, "invalid link", err)
			}
		default:
			return output.NewError(output.ConfigError,
				fmt.Sprintf("Explorer %s is not valid\nValid explorers: %s",
					args[1], append(_validExpl, "custom")), nil)
		}
	case "defaultacc":
		err1 := validator.ValidateAlias(args[1])
		err2 := validator.ValidateAddress(args[1])
		if err1 != nil && err2 != nil {
			return output.NewError(output.ValidationError, "failed to validate alias or address", nil)
		}
		ReadConfig.DefaultAccount.AddressOrAlias = args[1]
	case "language":
		language := isSupportedLanguage(args[1])
		if language == -1 {
			return output.NewError(output.ConfigError,
				fmt.Sprintf("Language %s is not supported\nSupported languages: %s",
					args[1], _supportedLanguage), nil)
		}
		ReadConfig.Language = _supportedLanguage[language]
	case "nsv2height":
		height, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return output.NewError(output.ValidationError, "invalid height", nil)
		}
		ReadConfig.Nsv2height = height
	case "wsEndpoint":
		ReadConfig.WsEndpoint = args[1]
	case "ipfsEndpoint":
		ReadConfig.IPFSEndpoint = args[1]
	case "ipfsGateway":
		ReadConfig.IPFSGateway = args[1]
	case "wsProjectRegisterContract":
		ReadConfig.WsProjectRegisterContract = args[1]
	case "wsProjectStoreContract":
		ReadConfig.WsProjectStoreContract = args[1]
	case "wsFleetManagementContract":
		ReadConfig.WsFleetManagementContract = args[1]
	case "wsProverStoreContract":
		ReadConfig.WsProverStoreContract = args[1]
	case "wsProjectDevicesContract":
		ReadConfig.WsProjectDevicesContract = args[1]
	case "wsRouterContract":
		ReadConfig.WsRouterContract = args[1]
	case "wsVmTypeContract":
		ReadConfig.WsVmTypeContract = args[1]
	}
	err := writeConfig()
	if err != nil {
		return err
	}
	output.PrintResult(strings.Title(args[0]) + " is set to " + args[1])
	return nil
}

// reset resets all values of config
func reset() error {
	ReadConfig.Wallet = ConfigDir
	ReadConfig.Endpoint = ""
	ReadConfig.SecureConnect = true
	ReadConfig.DefaultAccount = *new(Context)
	ReadConfig.Explorer = "iotexscan"
	ReadConfig.Language = "English"
	ReadConfig.AnalyserEndpoint = _defaultAnalyserEndpoint
	ReadConfig.WsEndpoint = _defaultWsEndpoint
	ReadConfig.IPFSEndpoint = _defaultIPFSEndpoint
	ReadConfig.IPFSGateway = _defaultIPFSGateway
	ReadConfig.WsProjectRegisterContract = _defaultWsProjectRegisterContract
	ReadConfig.WsProjectStoreContract = _defaultWsProjectStoreContract
	ReadConfig.WsFleetManagementContract = _defaultWsFleetManagementContract
	ReadConfig.WsProverStoreContract = _defaultWsProverStoreContract
	ReadConfig.WsProjectDevicesContract = _defaultWsProjectDevicesContract
	ReadConfig.WsRouterContract = _defaultWsRouterContract
	ReadConfig.WsVmTypeContract = _defaultWsVmTypeContract

	err := writeConfig()
	if err != nil {
		return err
	}

	output.PrintResult("Config set to default values")
	return nil
}
