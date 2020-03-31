package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd"
	"github.com/iotexproject/iotex-core/ioctl/doc"
)

var xctlPath string

func main() {
	toolName := "xctl"
	corePath, err := filepath.Abs(".")
	if err != nil {
		log.Fatal(err)
	}
	xctlPath = filepath.Join(corePath, "tools", "xctl")

	// TODO: update 'Install' part
	preString := `# xctl
xctl is a command-line interface for interacting with consortium blockchain.

# Build
` + "`./buildcli.sh`\n" + `

If you want to build xctl on Windows, you need to install mingw. Package manager [Chocolatey](https://chocolatey.org/) provides an easy way to intall latest mingw.
` + "`C:\\Windows\\system32> choco install mingw`\n" + `

After this command, target bin files will be placed in ./release/ folder, upload them to
specific release so install-cli.sh can download them.

# Install
## Install released build
    curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh

## Install latest build
    curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh -s "unstable"
`
	rootCmd := cmd.NewXctl()

	linkHandler := func(c *cobra.Command, s string) string {
		if c == rootCmd {
			return "readme/" + s
		}
		if strings.Contains(s, "xctl.md") {
			return "../README.md"
		}
		return s
	}

	filePrepender := func(s string) string {
		if strings.Contains(s, "README.md") {
			return preString
		}
		return ""
	}

	path := filepath.Join(xctlPath, "readme")
	err = doc.GenMarkdownTreeCustom(rootCmd, path, toolName, xctlPath, filePrepender, linkHandler)
	if err != nil {
		log.Fatal(err)
	}
}
