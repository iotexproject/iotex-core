# injector
injector is a command-line interface for interacting with IoTeX blockchains.

# Build
`./buildcli.sh`

After this command, target bin files will be placed in ./release/ folder, upload them to
specific release so install-cli.sh can download them.

# Intall
## Install released build
    curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-injector.sh | sh

## Install latest build
    curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-injector.sh | sh -s "unstable"

# Usage
    injector [command]

    Available Commands:
      help        Help about any command
      random      inject random actions

    Flags:
      -h, --help   help for injector

    Use "injector [command] --help" for more information about a command.
