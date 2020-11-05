#!/bin/bash

# create 10 accounts
# inject one account to the config.yaml for star server
# inject all accounts to the genesis.yaml
# show account addr and private key
# start servcer

# exit code:
# 2: mkdir error
# 3: build image error
# 4: create account error
# 5: copy config error
# 6: set account to the server error
# 7: show all acounts error
# 8: startup error
# 9: setup ioctl endpoint error

# Colour codes
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Commands
IOCTL_CMD="ioctl"
CREATE_10_ACCOUTS="$IOCTL_CMD account create -n 10"
SET_ENDPOINT="$IOCTL_CMD config set endpoint localhost:14014 --insecure"
SED_IS_GNU=0
MKDIR="mkdir -p"

# Dirs and files
USER_DIR="${HOME}/.iotex"
IOTEX_HOME="$HOME/iotex-var"
DOCKER_COMPOSE_HOME="$IOTEX_HOME/docker-compose"
ACCOUNTS_File="$USER_DIR/accounts"
PROJECT_ABS_DIR=$(cd "$(dirname "$0")";pwd)
REPO_ABS_DIR=$(cd "$PROJECT_ABS_DIR/..";pwd)

# Image
IOTEX_IMAGE="iotex-core:local"

pushd () {
    command pushd "$@" > /dev/null
}

popd () {
    command popd "$@" > /dev/null
}

function sedCheck() {
    sed --version > /dev/null 2>&1
    if [ $? -eq 0 ];then
        sed --version|grep 'GNU sed' > /dev/null 2>&1
        if [ $? -eq 0 ];then
            SED_IS_GNU=1
        fi
    fi
}

function checkDockerPermissions() {
    docker ps > /dev/null 2>&1
    if [ $? = 1 ];then
        echo -e "your $RED [$USER] $NC not privilege docker" 
        echo -e "please run $RED [sudo bash] $NC first"
        echo -e "Or docker not install "
        exit 1
    fi
}

function checkDockerCompose() {
    docker-compose --version > /dev/null 2>&1
    if [ $? -eq 127 ];then
        echo -e "$RED docker-compose command not found $NC"
        echo -e "Please install it first"
        exit 1
    fi
}

function installIoctl() {
    if command -v $IOCTL_CMD >/dev/null 2>&1; then
        echo 'Check iotex command success.'
    else
        echo "$IOCTL_CMD is not installed, start installing..."
        bash $REPO_ABS_DIR/install-cli.sh || exit 1
    fi
}

function setIoctlEndpoint() {
    $SET_ENDPOINT
}

function makeDir() {
    $MKDIR $USER_DIR
    $MKDIR $IOTEX_HOME
    $MKDIR $IOTEX_HOME/{log,data,etc,docker-compose}
}

function buildImage() {
    pushd $REPO_ABS_DIR
    docker build . -t $IOTEX_IMAGE
    popd
}

function copyConfigs() {
    \cp -f $PROJECT_ABS_DIR/docker-compose.yml $DOCKER_COMPOSE_HOME/docker-compose.yml
    \cp -f $REPO_ABS_DIR/config/standalone-config.yaml $IOTEX_HOME/etc/config.yaml
    \cp -f $REPO_ABS_DIR/config/standalone-genesis.yaml $IOTEX_HOME/etc/genesis.yaml
    echo "IOTEX_HOME=$IOTEX_HOME
IOTEX_IMAGE=$IOTEX_IMAGE" > $DOCKER_COMPOSE_HOME/.env
}

function createAccounts() {
    if [ ! -f $ACCOUNTS_File ];then
        $CREATE_10_ACCOUTS > $ACCOUNTS_File
    fi
}

function setAllAccountToGenesis() {
    sedCheck
    if [ $SED_IS_GNU -eq 1 ];then
        cat $ACCOUNTS_File  | while read line; do if [[ $line == \"address\"* ]];then account=$(echo $line|awk -F'"' '{print $4 ": \"100000000000000000000000000000000000\""}');sed -i "/^  initBalances:/a\ \ \ \ $account" $IOTEX_HOME/etc/config.yaml;fi;done
    else
        cat $ACCOUNTS_File  | while read line; do if [[ $line == \"address\"* ]];then account=$(echo $line|awk -F'"' '{print $4 ": \"100000000000000000000000000000000000\""}');sed -i '' "/^  initBalances:/a\\
\ \ \ \ $account
" $IOTEX_HOME/etc/config.yaml;fi;done
    fi
}

function showAllAccount() {
    echo 'Available Accounts'
    echo '=================='
    i=0;cat $ACCOUNTS_File | while read line; do if [[ $line == \"address\"* ]];then printf "\t($i) "; echo $line|awk -F'"' '{print $4}';i=$((i+1));fi;done
    echo ''
    echo 'Private Keys'
    echo '=================='
    i=0;cat $ACCOUNTS_File | while read line; do if [[ $line == \"privateKey\"* ]];then printf "\t($i) "; echo $line|awk -F'"' '{print $4}';i=$((i+1));fi;done
    echo ''
}

function startup() {
    echo -e "Start iotex-server."
    pushd $DOCKER_COMPOSE_HOME
    docker-compose up -d
    docker ps | grep iotex|grep -v grep > /dev/null 2>&1
    popd
}

function registSetup() {
    echo "$IOTEX_HOME" > $USER_DIR/workdir
}

function cleanAll() {
    echo -e "${RED} Starting delete all files... ${NC}"
    read -p "Are you sure? Press any key to continue ... [Ctrl + c exit!] " key1

    if [ "${IOTEX_HOME}X" = "X" ] || [ "${IOTEX_HOME}X" = "/X" ];then
        echo -e "${RED} \$IOTEX_HOME: ${IOTEX_HOME} is wrong. ${NC}"
        ## For safe.
        return
    fi

    # Delete docker
    if [ -d $DOCKER_COMPOSE_HOME ];then
        pushd $DOCKER_COMPOSE_HOME
        docker-compose rm -s -f -v
        popd
    fi

    # Delete dir and data
    rm -rf $IOTEX_HOME $USER_DIR
}

function main() {
    # Check and clean
    checkDockerPermissions     # Determine the current user can run docker
    checkDockerCompose         # Determin the docker-compose is installed

    # If clean all
    if [ $# -eq 1 ] && [ "$1" == "-c" ];then
        cleanAll
        exit 0
    fi

    # Install ioctl command
    installIoctl

    # Make work space dir
    makeDir || exit 2

    # Build image
    if [ $# -eq 0 ] || [ "$1" != "-q" ];then
        buildImage || exit 3
    fi

    # Create account
    createAccounts || exit 4

    # Set config
    copyConfigs || exit 5
    setAllAccountToGenesis || exit 6

    # Show accounts
    showAllAccount || exit 7

    # Start server
    startup || exit 8
    echo 'Listening on 127.0.0.1:14014'

    # Set ioctl endpoint
    setIoctlEndpoint || exit 9

    # Register the installation directory to the user directory file for the next startup
    registSetup
}

main $@
