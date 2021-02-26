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

initVersion() {
    PACKAGE_VERSION=$(git describe --tags)
    PACKAGE_COMMIT_ID=$(git rev-parse HEAD)
    GIT_STATUS=$(git status --porcelain)
    if ! [ -z "$GIT_STATUS" ];then
    #if git_status=$(git status --porcelain) && [[ -z ${git_status} ]]; then
        GIT_STATUS="dirty"
    else
    	GIT_STATUS="clean"
    fi
    GO_VERSION=$(go version)
    BUILD_TIME=$(date +%F-%Z/%T)
    VersionImportPath='github.com/iotexproject/iotex-core/pkg/version'
    PackageFlags="-X '${VersionImportPath}.PackageVersion=${PACKAGE_VERSION}' "
    ## Ubuntu PackageFlags+="-X " have fault
    if [ "$OS" = "linux" ]; then
    	PackageFlags=${PackageFlags}"-X '${VersionImportPath}.PackageCommitID=${PACKAGE_COMMIT_ID}' "
    	PackageFlags=${PackageFlags}"-X '${VersionImportPath}.GitStatus=${GIT_STATUS}' "
    	PackageFlags=${PackageFlags}"-X '${VersionImportPath}.GoVersion=${GO_VERSION}' "
    	PackageFlags=${PackageFlags}"-X '${VersionImportPath}.BuildTime=${BUILD_TIME}' "
    	PackageFlags=${PackageFlags}"-s -w -linkmode external -extldflags -static "
    else
    	PackageFlags+="-X '${VersionImportPath}.PackageCommitID=${PACKAGE_COMMIT_ID}' "
    	PackageFlags+="-X '${VersionImportPath}.GitStatus=${GIT_STATUS}' "
    	PackageFlags+="-X '${VersionImportPath}.GoVersion=${GO_VERSION}' "
    	PackageFlags+="-X '${VersionImportPath}.BuildTime=${BUILD_TIME}' "
    	PackageFlags+="-s -w"
   fi
}
initOS
initVersion
project_name="ioctl"

release_dir=./release
rm -rf $release_dir/*
mkdir -p $release_dir

cd  $(dirname $0)

gofmt -w ./

CGO_ENABLED=1 GOARCH=amd64 go build -tags netgo -ldflags "${PackageFlags}" -o $release_dir/$project_name-$OS-amd64 -v .
#CGO_ENABLED=1 GOARCH=386 go build -o $release_dir/$project_name-$OS-386 -v .
