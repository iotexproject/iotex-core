# ioctl
ioctl is a command-line interface for interacting with IoTeX blockchains.

# Build
`./buildcli.sh`

After this command, target bin files will be placed in ./release/ folder, upload them to
specific release so install-cli.sh can download them.

# Intall
## Install released build
    curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh

## Install latest build
    curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh -s "unstable"

# Usage
    ioctl [command]

    Available Commands:
      account     Manage accounts of IoTeX blockchain
      action      Manage actions of IoTeX blockchain
      alias       Manage aliases of IoTeX addresses
      bc          Deal with block chain of IoTeX blockchain
      help        Help about any command
      node        Deal with nodes of IoTeX blockchain
      update      Update ioctl with latest version
      version     Print the version of ioctl and node

    Flags:
      -h, --help   help for ioctl

    Use "ioctl [command] --help" for more information about a command.
