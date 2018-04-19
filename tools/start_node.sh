#!/bin/bash

file=$GOPATH/src/github.com/iotexproject/iotex-core/sampleconfig/stonevan/config_$1.yaml
echo "Starting node with config file:" $file
$GOPATH/src/github.com/iotexproject/iotex-core/bin/server -stderrthreshold=WARNING -log_dir=./log -config=$file

exit 0