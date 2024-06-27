#!/bin/bash

start=$1
end=$2

while true
do
    for i in $(seq $start $end)
    do
        echo "\n"
        # kill state2factory progress
        ssh chenchen@archive-factory-${i}m 'pkill -x iomigrater'
    done

    for i in $(seq $start $end)
    do
        echo "\n"
        # run state2factory from progress
        ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -p --v2 -m 200000 --nostats '${nostats}' --progress /data/workspace/iotex-core/log.migrate.progress.'${i}' &>> /data/workspace/iotex-core/log.migrate.'${i}' & exit'
    done

    wait

    # Wait for 1 hour
    sleep 3600
done