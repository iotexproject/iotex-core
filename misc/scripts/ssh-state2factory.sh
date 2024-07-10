#!/bin/bash

start=$1
end=$2

nostats="Account,Contract,Rewarding"

for i in $(seq $start $end)
do
   echo "\n"
   # re-run state2factory
   # ssh chenchen@archive-factory-${i}m 'sudo rm -r /data/mainnet-factory/'${i}'m/trie.db && rm -rf /data/workspace/iotex-core/log.migrate.progress.'${i}' && nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -p --v2 --nostats '${nostats}' --progress /data/workspace/iotex-core/log.migrate.progress.'${i}' &> /data/workspace/iotex-core/log.migrate.'${i}' & exit'

   # run state2factory from progress at ns
   # namespaces="Account,Candidate,CandsMap,Code,Contract,Rewarding,Staking,System"
   # namespaces="Contract"
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -p --v2 -m 200000 -n '${namespaces}' --nostats '${nostats}' --progress /data/workspace/iotex-core/log.migrate.progress.'${i}' &>> /data/workspace/iotex-core/log.migrate.'${i}' & exit'

   # run state2factory from progress
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -p --v2 -m 200000 --nostats '${nostats}' --progress /data/workspace/iotex-core/log.migrate.progress.'${i}' &>> /data/workspace/iotex-core/log.migrate.'${i}' & exit'
   
   # check state2factory progress
   # ssh chenchen@archive-factory-${i}m 'tail -n20 /data/workspace/iotex-core/log.migrate.'${i}' && echo "==========" && cat /data/workspace/iotex-core/log.migrate.progress.'${i}''

   # kill state2factory progress
   # ssh chenchen@archive-factory-${i}m 'pkill -x iomigrater'

   # run progress upgrade
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f fake --progress /data/workspace/iotex-core/log.migrate.progress.'${i}' --upgrade --v2 &>> /data/workspace/iotex-core/log.migrate.'${i}' & exit'

   # correctness check
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater dbcompare -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -m 200000 --nostats "Account,Contract,Rewarding" &> /data/workspace/iotex-core/log.compare.'${i}' & exit'

   # check the correctness progress
   # ssh chenchen@archive-factory-${i}m 'cat /data/workspace/iotex-core/log.compare.'${i}''

   # apply diff
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -p --v2 -m 200000 --nostats "Contract,Rewarding" --diff /data/workspace/iotex-core/log.compare.'${i}' &> /data/workspace/iotex-core/log.apply.'${i}' & exit'

   # check the diff apply
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/iomigrater dbcompare -s /data/mainnet/'${i}'m/trie.db -f /data/mainnet-factory/'${i}'m/trie.db -m 200000 --nostats "Contract,Rewarding" --diff /data/workspace/iotex-core/log.compare.'${i}' &> /data/workspace/iotex-core/log.compare.apply.'${i}' & exit'

   # copy && compact factory db
   # ssh chenchen@archive-factory-${i}m 'rm -rf /data/workspace/archive/data/trie.db && cp -r /data/mainnet-factory/'${i}'m/trie.db /data/workspace/archive/data/ && nohup /data/workspace/iotex-core/bin/iomigrater pebble-compact -f /data/workspace/archive/data/trie.db & exit'

   # show database size
   # ssh chenchen@archive-factory-${i}m 'du -sh /data/mainnet/'${i}'m/trie.db && du -sh /data/mainnet-factory/'${i}'m/trie.db && du -sh /data/workspace/archive/data/trie.db'

   # run server
   # ssh chenchen@archive-factory-${i}m 'cat /data/workspace/iotex-core/config-snapshot-'${i}'m.yaml'
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/server -config-path=/data/workspace/iotex-core/config-snapshot-'${i}'m.yaml -genesis-path=/data/workspace/iotex-core/genesis.yaml &>> /data/workspace/iotex-core/log.server.'${i}' & exit'

   # kill server
   # ssh chenchen@archive-factory-${i}m 'pkill -x server'

   # run gateway server
   # ssh chenchen@archive-factory-${i}m 'nohup /data/workspace/iotex-core/bin/server -config-path=/data/workspace/iotex-core/config-archive.yaml -genesis-path=/data/workspace/iotex-core/genesis.yaml -plugin=readonly_gateway &>> /data/workspace/iotex-core/log.shard.'${i}' & exit'

   # kill server in systemctl
   # ssh chenchen@archive-factory-${i}m 'sudo systemctl stop archive & exit'

   # run gateway server in systemctl
   ssh chenchen@archive-factory-${i}m 'sudo systemctl start archive & exit'

   # check server log
   # ssh chenchen@archive-factory-${i}m 'tail -n3 /data/workspace/iotex-core/log.server.'${i}''

   # copy compact factory snapshot
   # ssh chenchen@archive-factory-${i}m 'sudo mkdir -p /data/mainnet-factory-compact/'${i}'m && sudo chmod -R 777 /data/mainnet-factory-compact/ && cp -r /data/mainnet-factory/'${i}'m/trie.db /data/mainnet-factory-compact/'${i}'m/ && nohup /data/workspace/iotex-core/bin/iomigrater pebble-compact -f /data/mainnet-factory-compact/'${i}'m/trie.db & exit'
   # tar factory snapshot
   # ssh chenchen@archive-factory-${i}m 'cd /data/mainnet-factory-compact/'${i}'m && tar -czvf trie.db.tar.gz trie.db && exit'

   # collect factory snapshots and push to s3
   # scp chenchen@archive-factory-${i}m:/data/mainnet-factory/${i}m/trie.db.tar.gz /data/mainnet-factory-snapshot-all/${i}m/trie.db.tar.gz
   
   # check server live
   # ssh chenchen@archive-factory-${i}m 'ps aux | grep server & exit'
done