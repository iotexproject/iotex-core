# ioctl
ioctl is a command-line interface for interacting with IoTeX blockchains.

# Build
`./buildcli.sh`

If you want to build ioctl on Windows, you need to install mingw. Package manager [Chocolatey](https://chocolatey.org/) provides an easy way to intall latest mingw.
`C:\Windows\system32> choco install mingw`

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
      config      Get, set, or reset configuration for ioctl
      help        Help about any command
      node        Deal with nodes of IoTeX blockchain
      stake       Support native staking from ioctl
      update      Update ioctl with latest version
      version     Print the version of ioctl and node
      xrc20       Support ERC20 standard command-line from ioctl
    
    Flags:
      -h, --help                   help for ioctl
      -o, --output-format string   output format
    
    Use "ioctl [command] --help" for more information about a command.
