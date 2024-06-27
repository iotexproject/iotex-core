#!/bin/bash

start=$1
end=$2

for i in $(seq $start $end)
do
    scp /Users/chenchen/dev/iotex-core/config-snapshot-${i}m.yaml archive-factory-${i}m:/data/workspace/iotex-core
    scp /Users/chenchen/dev/iotex-core/genesis-snapshot-0m.yaml archive-factory-${i}m:/data/workspace/iotex-core/genesis.yaml
#    scp -r /data/mainnet/${i}m archive-factory-${i}m:/data/mainnet/
#    scp -r /data/workspace/iotex-core/bin archive-factory-${i}m:/data/workspace/iotex-core/
    #scp -r /data/workspace/archive/data archive-factory-${i}m:/data/workspace/archive
done

#scp /data/workspace/iotex-core/config-snapshot-${i}m.yaml archive-factory-${i}m:/data/workspace/iotex-core
#scp /data/workspace/iotex-core/genesis.yaml archive-factory-${i}m:/data/workspace/iotex-core
