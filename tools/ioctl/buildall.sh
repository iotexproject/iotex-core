#!/usr/bin/env bash

apt install -y gcc-mingw-w64
apt install -y gcc-multilib


#windows
GOOS=windows GOARCH=amd64   CGO_ENABLED=1  CXX_FOR_TARGET=x86_64-w64-mingw64-g++ CC_FOR_TARGET=x86_64-w64-mingw64-gcc  go build -o ioctl-windows-amd64.exe

#mac
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o ioctl-darwin-amd64

#linux
GOOS=linux GOARCH=amd64 go build -o ioctl-linux-amd64
GOOS=linux GOARCH=arm64 go build -o ioctl-linux-arm64
