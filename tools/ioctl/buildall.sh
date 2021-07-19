apt-get install gcc-mingw-w64
apt-get install gcc-multilib
GOOS=windows GOARCH=amd64   CGO_ENABLED=1  CXX_FOR_TARGET=x86_64-w64-mingw64-g++ CC_FOR_TARGET=x86_64-w64-mingw64-gcc  go build -o ioctl-windows-amd64.exe
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags netgo -o ioctl-darwin-amd64
GOOS=linux GOARCH=amd64 go build -o ioctl-linux-amd64

