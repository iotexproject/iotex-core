#!/bin/sh

# This install script is intended to install solc release supported.
#
# It attempts to identify the current platform and an error will be thrown if
# the platform is not supported.
#
# Environment variables:
# - INSTALL_DIRECTORY (optional): defaults to /usr/local/bin or $HOME (windows)
#
# You can install using this script:
# $ curl https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-solc.sh | sh

set -e

LINUX_RELEASES_URL="https://github.com/ethereum/solidity/releases/download/v0.4.25/solidity-ubuntu-trusty.zip"
WINDOWS_RELEASES_URL="https://github.com/ethereum/solidity/releases/download/v0.4.25/solidity-windows.zip"
INSTALL_DIRECTORY='/usr/local/bin'

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
if [ "$OS" = "darwin" ]; then
    sudo brew install solidity@4
else
    if [ "${OS}" != "linux" ] && { [ "${ARCH}" = "ppc64" ] || [ "${ARCH}" = "ppc64le" ];}; then
        # ppc64 and ppc64le are only supported on Linux.
        echo "${OS}-${ARCH} is not supported by this installation script"
    fi

    # add .exe if on windows
    INSTALL_NAME="solc"
    if [ "$OS" = "windows" ]; then
        INSTALL_NAME="$INSTALL_NAME.exe"
        BINARY_URL="$WINDOWS_RELEASES_URL"
    else
        BINARY_URL="$LINUX_RELEASES_URL"
    fi

    DOWNLOAD_FILE=$(mktemp)

    downloadFile "$BINARY_URL" "$DOWNLOAD_FILE"
    unzip -o "$DOWNLOAD_FILE" -d /tmp

    echo "Setting executable permissions."
    chmod +x /tmp/"$INSTALL_NAME"

    if [ "$OS" = "windows" ]; then
        echo "Moving executable to $HOME/$INSTALL_NAME"
        mv /tmp/"$INSTALL_NAME" "$HOME/$INSTALL_NAME"
    else
        echo "Moving executable to $INSTALL_DIRECTORY/$INSTALL_NAME"
        sudo mv /tmp/"$INSTALL_NAME" "$INSTALL_DIRECTORY/$INSTALL_NAME"
    fi
fi

