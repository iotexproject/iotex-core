#!/bin/sh

# This install script is intended to download and install the latest available
# release of the ioctl dependency manager for Golang.
#
# It attempts to identify the current platform and an error will be thrown if
# the platform is not supported.
#
# Environment variables:
# - INSTALL_DIRECTORY (optional): defaults to $GOPATH/bin (if $GOPATH exists) 
#   or /usr/local/bin (else)
# - CLI_RELEASE_TAG (optional): defaults to fetching the latest release
#
# You can install using this script:
# $ curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh

set -e

RELEASES_URL="https://github.com/iotexproject/iotex-core/releases"
S3URL="https://s3-ap-southeast-1.amazonaws.com/ioctl"
INSTALL_DIRECTORY='/usr/local/bin'

BREW_UPDATE_CMD="brew update"
BREW_INSTALL_CMD="brew install ioctl"
BREW_UNSTABLE_TAP_CMD="brew tap iotexproject/ioctl-unstable"
BREW_UNSTABLE_INSTALL_CMD="brew install --HEAD iotexproject/ioctl-unstable/ioctl-unstable"

downloadJSON() {
    url="$2"

    echo "Fetching $url.."
    if test -x "$(command -v curl)"; then
        response=$(curl -s -L -w 'HTTPSTATUS:%{http_code}' -H 'Accept: application/json' "$url")
        body=$(echo "$response" | sed -e 's/HTTPSTATUS\:.*//g')
        code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
    elif test -x "$(command -v wget)"; then
        temp=$(mktemp)
        body=$(wget -q --header='Accept: application/json' -O - --server-response "$url" 2> "$temp")
        code=$(awk '/^  HTTP/{print $2}' < "$temp" | tail -1)
        rm "$temp"
    else
        echo "Neither curl nor wget was available to perform http requests."
        exit 1
    fi
    if [ "$code" != 200 ]; then
        echo "Request failed with code $code"
        exit 1
    fi

    eval "$1='$body'"
}

downloadFile() {
    url="$1"
    destination="$2"

    echo "Fetching $url.."
    if test -x "$(command -v curl)"; then
        code=$(curl -s -w '%{http_code}' -L "$url" -o "$destination")
    elif test -x "$(command -v wget)"; then
        code=$(wget -q -O "$destination" --server-response "$url" 2>&1 | awk '/^  HTTP/{print $2}' | tail -1)
    else
        echo "Neither curl nor wget was available to perform http requests."
        exit 1
    fi

    if [ "$code" != 200 ]; then
        echo "Request failed with code $code"
        exit 1
    fi
}

initArch() {
    ARCH=$(uname -m)
    case $ARCH in
        amd64) ARCH="amd64";;
        x86_64) ARCH="amd64";;
        i386) ARCH="386";;
        ppc64) ARCH="ppc64";;
        ppc64le) ARCH="ppc64le";;
        s390x) ARCH="s390x";;
        armv6*) ARCH="arm";;
        armv7*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        *) echo "Architecture ${ARCH} is not supported by this installation script"; exit 1;;
    esac
    echo "ARCH = $ARCH"
}

initOS() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    OS_CYGWIN=0
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

# identify platform based on uname output
initArch
initOS

# assemble expected release artifact name
if [ "${OS}" != "linux" ] && { [ "${ARCH}" = "ppc64" ] || [ "${ARCH}" = "ppc64le" ];}; then
    # ppc64 and ppc64le are only supported on Linux.
    echo "${OS}-${ARCH} is not supported by this instalation script"
else
    BINARY="ioctl-${OS}-${ARCH}"
fi

if [ "${OS}" = "darwin" ];then
    if test -x "$(command -v brew)"; then
        if [ "$1" = "unstable" ]; then
            $BREW_UNSTABLE_TAP_CMD && $BREW_UNSTABLE_INSTALL_CMD
            echo "Command-line tools is installed to `command -v ioctl-unstable`"
        else
            $BREW_UPDATE_CMD && $BREW_INSTALL_CMD
            echo "Command-line tools is installed to `command -v ioctl`"
        fi
        exit 0
    else
        echo "None command 'brew' is available to perform installed."
        exit 1
    fi
fi

# add .exe if on windows
if [ "$OS" = "windows" ]; then
    BINARY="$BINARY.exe"
fi

if [ -z "$CLI_RELEASE_TAG" ]; then
    downloadJSON LATEST_RELEASE "$RELEASES_URL/latest"
    CLI_RELEASE_TAG=$(echo "${LATEST_RELEASE}" | tr -s '\n' ' ' | sed 's/.*"tag_name":"//' | sed 's/".*//' )
fi

if [ "$1" = "unstable" ]; then
    BINARY_URL="$S3URL/$BINARY"

else
    # fetch the real release data to make sure it exists before we attempt a download
    downloadJSON RELEASE_DATA "$RELEASES_URL/tag/$CLI_RELEASE_TAG"
    BINARY_URL="$RELEASES_URL/download/$CLI_RELEASE_TAG/$BINARY"
fi

DOWNLOAD_FILE=$(mktemp)

downloadFile "$BINARY_URL" "$DOWNLOAD_FILE"

echo "Setting executable permissions."
chmod +x "$DOWNLOAD_FILE"

INSTALL_NAME="ioctl"

if [ "$OS" = "windows" ]; then
    INSTALL_NAME="$INSTALL_NAME.exe"
    echo "Moving executable to $HOME/$INSTALL_NAME"
    mv "$DOWNLOAD_FILE" "$HOME/$INSTALL_NAME"
else
    echo "Moving executable to $INSTALL_DIRECTORY/$INSTALL_NAME"
    sudo mv "$DOWNLOAD_FILE" "$INSTALL_DIRECTORY/$INSTALL_NAME"
fi
