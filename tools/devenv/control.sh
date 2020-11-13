#!/bin/bash

# Colour codes
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Dirs and files
USER_DIR="${HOME}/.iotex"
PROJECT_ABS_DIR=$(cd "$(dirname "$0")";pwd)
REPO_ABS_DIR=$(cd "$PROJECT_ABS_DIR/..";pwd)

pushd () {
    command pushd "$@" > /dev/null
}

popd () {
    command popd "$@" > /dev/null
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

function startup() {
    echo -e "Start iotex-server..."
    pushd $DOCKER_COMPOSE_HOME
    docker-compose up -d
    docker ps | grep iotex|grep -v grep > /dev/null 2>&1
    popd
}

function stop() {
    echo -e "Stopping iotex-server..."
    pushd $DOCKER_COMPOSE_HOME
    docker-compose stop
    popd
}

function status() {
    pushd $DOCKER_COMPOSE_HOME
    docker-compose ps iotex
    popd
}

function main() {
    # Check and clean
    checkDockerPermissions     # Determine the current user can run docker
    checkDockerCompose         # Determin the docker-compose is installed

    DOCKER_COMPOSE_HOME="$(cat $USER_DIR/workdir)/docker-compose"
    
    # Start server
    if [ "$1" == "start" ];then
        startup || exit 2
        echo 'Listening on 127.0.0.1:14014'
    elif [ "$1" == "stop" ];then
        stop
    elif [ "$1" == "status" ];then
        status
    fi
}

main $@
