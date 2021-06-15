// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Regexp patterns
const (
	ipPattern       = `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`
	domainPattern   = `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`
	urlPattern      = `[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
	localPattern    = "localhost"
	endpointPattern = "(" + ipPattern + "|(" + domainPattern + ")" + "|(" + localPattern + "))" + `(:\d{1,5})?`
)

var (
	supportedLanguage = []string{"English", "中文"}
	validArgs         = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height"}
	validGetArgs      = []string{"endpoint", "wallet", "explorer", "defaultacc", "language", "nsv2height", "all"}
	validExpl         = []string{"iotexscan", "iotxplorer"}
	endpointCompile   = regexp.MustCompile("^" + endpointPattern + "$")
)

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:       "get VARIABLE",
	Short:     "Get config fields from ioctl",
	Long:      "Get config fields from ioctl\nValid Variables: [" + strings.Join(validGetArgs, ", ") + "]",
	ValidArgs: validGetArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d\n"+
				"Valid arg(s): %s", len(args), validGetArgs)
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := Get(args[0])
		return output.PrintError(err)
	},
}

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:       "set VARIABLE VALUE",
	Short:     "Set config fields for ioctl",
	Long:      "Set config fields for ioctl\nValid Variables: [" + strings.Join(validArgs, ", ") + "]",
	ValidArgs: validArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("accepts 2 arg(s), received %d\n"+
				"Valid arg(s): %s", len(args), validArgs)
		}
		return cobra.OnlyValidArgs(cmd, args[:1])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := set(args)
		return output.PrintError(err)
	},
}

// configResetCmd represents the config reset command
var configResetCmd = &cobra.Command{
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
	configSetCmd.Flags().BoolVar(&Insecure, "insecure", false,
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
		return nil
	case "wallet":
		output.PrintResult(ReadConfig.Wallet)
		return nil
	case "defaultacc":
		if ReadConfig.DefaultAccount.AddressOrAlias == "" {
			return output.NewError(output.ConfigError, "default account did not set", nil)
		}
		fmt.Println(ReadConfig.DefaultAccount.String())
		return nil
	case "explorer":
		output.PrintResult(ReadConfig.Explorer)
		return nil
	case "language":
		output.PrintResult(ReadConfig.Language)
		return nil
	case "nsv2height":
		fmt.Println(ReadConfig.Nsv2height)
		return nil
	case "all":
		fmt.Println(ReadConfig.String())
		return nil
	}
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
	return endpointCompile.MatchString(endpoint)
}

// isValidExplorer checks if the explorer is a valid option
func isValidExplorer(arg string) bool {
	for _, exp := range validExpl {
		if arg == exp {
			return true
		}
	}
	return false
}

// isSupportedLanguage checks if the language is a supported option and returns index when supported
func isSupportedLanguage(arg string) Language {
	if index, err := strconv.Atoi(arg); err == nil && index >= 0 && index < len(supportedLanguage) {
		return Language(index)
	}
	for i, lang := range supportedLanguage {
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
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
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
			fmt.Scanln(&link)
			match, err := regexp.MatchString(urlPattern, link)
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
					args[1], append(validExpl, "custom")), nil)
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
					args[1], supportedLanguage), nil)
		}
		ReadConfig.Language = supportedLanguage[language]
	case "nsv2height":
		height, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return output.NewError(output.ValidationError, "invalid height", nil)
		}
		ReadConfig.Nsv2height = height
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

	err := writeConfig()
	if err != nil {
		return err
	}

	output.PrintResult("Config set to default values")
	return nil
}
