#!/bin/sh

initOS() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    OS_CYGWIN=0
    if [ -n "$DEP_OS" ]; then
        echo "Using DEP_OS"
        OS="$DEP_OS"
    fi
    case "$OS" in
        darwin) OS='darwin';;
        linux) OS='linux';;
        freebsd) OS='freebsd';;
        mingw*) OS='windows';;
        msys*) OS='windows';;
	cygwin*)
	    OS='windows'
	    OS_CYGWIN=1
	    ;;
        *) echo "OS ${OS} is not supported by this installation script"; exit 1;;
    esac
    echo "OS = $OS"
}

initOS

project_name="ioctl"
release_version="0.0.1"

release_dir=./release
rm -rf $release_dir/*
mkdir -p $release_dir

cd  $(dirname $0)

gofmt -w ./

CGO_ENABLED=1 GOARCH=amd64 go build -o $release_dir/$project_name-$OS-amd64 -v .
#CGO_ENABLED=1 GOARCH=386 go build -o $release_dir/$project_name-$OS-386 -v .
