// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

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
	validArgs       = []string{"endpoint", "wallet", "explorer", "defaultacc"}
	validGetArgs    = []string{"endpoint", "wallet", "explorer", "defaultacc", "all"}
	validExpl       = []string{"iotexscan", "iotxplorer"}
	urlCompile      = regexp.MustCompile(urlPattern)
	endpointCompile = regexp.MustCompile("^" + endpointPattern + "$")
)

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:       "get VARIABLE",
	Short:     "Get config fields from ioctl",
	Long:      "Get config fields from ioctl\nValid Variables: [" + strings.Join(validGetArgs, ", ") + "]",
	ValidArgs: validGetArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("Accepts 1 arg(s), received %d\n"+
				"Valid arg(s): %s\n", len(args), validGetArgs)
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := Get(args[0])
		if err == nil {
			fmt.Println(output)
		}
		return err
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
			return fmt.Errorf("Accepts 2 arg(s), received %d\n"+
				"Valid arg(s): %s\n", len(args), validArgs)
		}
		return cobra.OnlyValidArgs(cmd, args[:1])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := set(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// configResetCmd represents the config reset command
var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset config to default",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := reset()
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func init() {
	configSetCmd.Flags().BoolVar(&Insecure, "insecure", false,
		"set insecure connection as default")
}

// Get gets config variable
func Get(arg string) (string, error) {
	switch arg {
	default:
		return "", ErrConfigNotMatch
	case "endpoint":
		if ReadConfig.Endpoint == "" {
			return "", ErrEmptyEndpoint
		}
		return fmt.Sprint(ReadConfig.Endpoint, "    secure connect(TLS):",
			ReadConfig.SecureConnect), nil
	case "wallet":
		return ReadConfig.Wallet, nil
	case "defaultacc":
		return fmt.Sprint(ReadConfig.DefaultAccount), nil
	case "explorer":
		return ReadConfig.Explorer, nil
	case "all":
		all, err := ioutil.ReadFile(DefaultConfigFile)
		return string(all), err
	}
}

// GetContextAddressOrAlias gets current context
func GetContextAddressOrAlias() (string, error) {
	defaultAccount := ReadConfig.DefaultAccount
	if strings.EqualFold(defaultAccount.AddressOrAlias, "") {
		return "", fmt.Errorf(`use "ioctl config set defaultacc ADDRESS|ALIAS" to config default account first`)
	}
	return defaultAccount.AddressOrAlias, nil
}

// GetAddressOrAlias gets address from args or context
func GetAddressOrAlias(args []string) (address string, err error) {
	if len(args) == 1 && !strings.EqualFold(args[0], "") {
		address = args[0]
	} else {
		address, err = GetContextAddressOrAlias()
	}
	return
}

// isMatch makes sure the endpoint matches the endpoint match pattern
func isMatch(endpoint string) bool {
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

// writeConfig writes to config file
func writeConfig() error {
	out, err := yaml.Marshal(&ReadConfig)
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return fmt.Errorf("failed to write to config file %s", DefaultConfigFile)
	}
	return err
}

// set sets config variable
func set(args []string) (string, error) {
	switch args[0] {
	default:
		return "", ErrConfigNotMatch
	case "endpoint":
		if !isMatch(args[1]) {
			return "", fmt.Errorf("Endpoint %s is not valid", args[1])
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
			fmt.Println("Please enter a custom link below:")
			fmt.Println("Example: iotexscan.io/action/")
			fmt.Print("Link: ")
			var link string
			fmt.Scanln(&link)
			match, err := regexp.MatchString(urlPattern, link)
			if err != nil {
				return "", fmt.Errorf("")
			}
			if match {
				ReadConfig.Explorer = link
				writeConfig()
				return strings.Title(args[0]) + " is set to " + link, nil
			}
			return "", fmt.Errorf("Invalid link")
		default:
			return "", fmt.Errorf("Explorer %s is not valid\nValid Explorers: %s", args[1], append(validExpl, "custom"))
		}
	case "defaultacc":
		err1 := validator.ValidateAlias(args[1])
		err2 := validator.ValidateAddress(args[1])
		if err1 != nil && err2 != nil {
			return "", fmt.Errorf("failed to validate alias or address:%s %s", err1, err2)
		}
		ReadConfig.DefaultAccount.AddressOrAlias = args[1]
	}
	err := writeConfig()
	if err != nil {
		return "", err
	}
	return strings.Title(args[0]) + " is set to " + args[1], nil
}

// reset resets all values of config
func reset() (string, error) {
	ReadConfig.Wallet = ConfigDir
	ReadConfig.Endpoint = ""
	ReadConfig.SecureConnect = true
	ReadConfig.DefaultAccount = *new(Context)
	ReadConfig.Explorer = "iotexscan"
	out, err := yaml.Marshal(&ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", DefaultConfigFile)
	}
	return "Config reset to default values", nil
}
