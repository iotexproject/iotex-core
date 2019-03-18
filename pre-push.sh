#!/bin/sh

check_result() {
    if [ $? != 0 ]; then
        echo "$1 failed!!"
        echo "push will not execute"
        echo "$?"
        exit 1
    else
        echo "$1 passed."
        echo "push will execute"
        echo "$?"
    fi
}

cd ../../
./go.test.sh
bash <(curl -s https://codecov.io/bash)
go test -run=XXX -bench=. $(go list ./crypto)
make minicluster

check_result local_debug

