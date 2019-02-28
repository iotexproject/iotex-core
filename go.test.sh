#!/usr/bin/env bash

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v 'vender' ); do
    go test -short -v -coverprofile=profile.out -covermode=count "$d" > go_test.txt
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
    LINE=`cat go_test.txt | grep -v "no test" | wc -l`
    if [[ $LINE -gt 0 ]];then
    echo "!!!!!!!!!!!!!!!"
    cat go_test.txt
    echo "#################"
    #cat go_test.txt |  /go/bin/go2xunit > /tmp/test_report_upload/coverage_`basename "$d"`_`date +"%H_%M_%S"`.xml 2>/dev/null | echo "pass"
    cat go_test.txt |  /go/bin/go2xunit > /tmp/test_report_upload/coverage_`basename "$d"`_`date +"%H_%M_%S"`.xml 
    fi
    rm -f go_test.txt
done

