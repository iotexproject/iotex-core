
# 1 - configure ssh key
pubkey="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDKtQNJdhE0DAabrDVMXMiwFBE0PxRIQAHKC8QMbHm/W3kgJFT39ygpAzid1Npz4osluFWpma2rX2nFJhrQ0mJ3ZT8EUkjnvwDRC3gjCVG47aLjpQy/F0hr6Ed+9v3XHZGJfn4xwh5GgdkNZczzl20kqqNnouclZxkQrpD7wf5ZSyl5eCwXoOZUYlvzF/a0m9aGz9pbbMc3s8kxEmO0+jEP6bClMF+b9U1Yn9B80ZRCQgvfFPAOEOL8mPZ8XGszi4KXNKy6yyoAIGyVhYcwblMMbQcPb2a1H3f/3Elu/WsuWKORw/g5T/X22phokkQAe3mtF4JSYL3tw8l6z9cTXg7trpJzpE2nHkoO48m0CYyCJnjbB6kxrnYmGupx3deF1sB+VxyIrIKw3TEt2pd3izKhmQ6h/gMJ3M+cXlpcBkYxebQX+Nlz628u33iLtqVnEGoyAJQC6yyXaqPjcta+GzX9icmugSUU/ZPgKRX/AAy+RRdqcqqsI4SRt8Hrlaxm+Y0= chenchen@shard-dev"
echo $pubkey >> /home/chenchen/.ssh/authorized_keys
pubkey="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC5pbLbe/Cqm8PQpVATD4Ya7/yPq4s+9bkV8ZWmY477ptIqYUvx+8igqKQ9+ugrBO0yVpTp4twzmmyD0nEeElFjHSVyY5WCi/1Kr8IdqBGI3YP7nWt+/NVLbs0eDSBTbgwZdFB+o8xtBROWA4YcdtOdAsNHU58JqG+7yZDTpqkithUHTgCiRUBvUTsjDhKe8BZ4HSCfsBGytUAtDEh1gwLenha5UnJOyj+42G+Uw5+gWG78qmXAMOYavk3TBVT4KVhNQYIlJ4e3eV0DUx4YB1ogzAsP1uotYryqDDgX3F5zb27soP66xQowZEkFpd+YhRcDMYWcg6dVOaUbPtmYFyM97cQEfkfOC7m2h0j+JgK60a1wEOWa6Gube43rgnW6/4zczxr4XSR8EhyAMysVu0YnOrLoMU2W+LVC6Tg1NaFxi0MX29/bo7P1IlgXOChFmhY6rRvHAhxSzuMIdXKQ10L980QhVan6IiOeDk4M0mDGVoOX6Txkco6GELVfbqSRk20= chenchen@archive-factory-15m"
echo $pubkey >> /home/chenchen/.ssh/authorized_keys
pubkey="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDCMiXNQDNo7hnq8UD5qIPWDaIRYpSxqdI2+eHon8yiDUVaMXTcb7lBuhAFZGXJsbABRmVJK+QOJRG2GSEA6wLdD1YuJTL91Obr348HIu+FBnSz/kzTDv7oxTISZ4PNLJ2AloROuqVpQhRZ+UA1tRN3V2IXMS7i+FVg0DG0CKILzTOLlGFUmIW0oe02/E+CeSiJMJmZ+LUM/pcxEkA/FNZFPef3WZd6aY1vRC3Yny7CJFkA6qUSE+RF2udI7zTT9cgew8xi/YNlBr5QQk+EI0NyG6tyk9Ukf6EIKlhAEF7SOD9PHC+JccC/SSjCRDpzEOqkYB0yg1PumKGdQ9telNIybxZ1hi2yiJxlBJzzMx11HFtoipcvDDdp8QbcGtR2qwhiSh9NHTUWlZ6qHfi72t2kiVSZeXY/og1K9/a2kSc/9VXKd6u7O7ErRm7h1SOSGXPPV/xvJgwSinLIKF6OyEngYUMV7QZIn2LqSbv405o0h+vBy9O5haMYduNTc+Z8bOs= chenchen@archive-factory-16m"
echo $pubkey >> /home/chenchen/.ssh/authorized_keys

sudo mkdir -p /data/mainnet
sudo chmod -R 777 /data/mainnet
sudo mkdir -p /data/workspace/iotex-core/
sudo chmod -R 777 /data/workspace/iotex-core/
sudo mkdir -p /data/workspace/archive/log
sudo mkdir -p /data/workspace/archive/data
sudo chmod -R 777 /data/workspace/archive
# 2 - scp programs && statedb snapshot (run on dev1)
scp -r /data/mainnet/13m archive-factory-13m:/data/mainnet/
scp -r /data/workspace/iotex-core/bin archive-factory-13m:/data/workspace/iotex-core/
scp /data/workspace/iotex-core/config-snapshot-13m.yaml archive-factory-13m:/data/workspace/iotex-core
scp /data/workspace/iotex-core/genesis.yaml archive-factory-13m:/data/workspace/iotex-core

# 3 - run state2factory
/data/workspace/iotex-core/bin/iomigrater state2factory -s /data/mainnet/13m/trie.db -f /data/mainnet-factory/13m/trie.db -p --v2

# 4 - run pebble compact
cp -r /data/mainnet-factory/13m/trie.db  /data/mainnet-factory/13m/trie.db.compact
/data/workspace/iotex-core/bin/iomigrater pebble-compact -f /data/mainnet-factory/13m/trie.db.compact

# 4 - run server stop at height 14m255
/data/workspace/iotex-core/bin/server -config-path=/data/workspace/iotex-core/config-snapshot-13m.yaml -genesis-path=/data/workspace/iotex-core/genesis.yaml