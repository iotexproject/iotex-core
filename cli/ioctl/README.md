    ioctl is a command-line interface for interacting with IoTeX blockchains.

    Build:
    ./buildcli.sh
    after this command, target bin files will be placed in ./release/ folder, upload them to
    specific release so install-cli.sh can download them.

    Intall:
    curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh
    
    Usage:
      ioctl [command]
    
    Available Commands:
      account     Deal with accounts of IoTeX blockchain
      action      Deal with actions of IoTeX blockchain
      bc          Deal with block chain of IoTeX blockchain
      help        Help about any command
      node        Deal with nodes of IoTeX blockchain
      version     Print the version number of ioctl
      wallet      Manage accounts of IoTeX blockchain
    
    Flags:
      -h, --help   help for ioctl
    
    Use "ioctl [command] --help" for more information about a command.
