# iotex-bot

Bot is a server for testing with IoTeX blockchains.
we could set execute interval and timeout.In every execute interval,three steps will be execute and check timeout sequentially.

There's three steps to execute every interval:
1. Make a token transfer,from signer to signer,but this will consume a little gas,about 0.01 IOTX.
2. Make a xrc20 token transfer,from signer to signer,this will consume a little gas,about 0.03 IOTX.
3. Make a multisend execution,from signer to any address that you seted,if we send 300 addresses,gas consumes is about 4.74 IOTX.
The execute data will be set in config.yaml,data can be make through https://remix.ethereum.org/#optimize=false&evmVersion=null&appVersion=0.7.7&version=soljson-v0.4.24+commit.e67f0147.js

Please make sure there's enough balance for the signer.

We will check if results returned right,if it's not right we will alert.
Alert's way is still pending,now we just log it.

# Build
make build

After this command, target bin files will be placed in this folder.

# Usage
   bot -config-path=[string]
     -config-path string
       	Config path (default "config.yaml")

# pre run
First we need to deploy two contracts,one for xrc20,one for multisend,those two contracts is in server/bot/contract

# modify config.yaml
We need to add two contracts's address and a signer with iotx to config.yaml.

# run
bot -config-path=/etc/iotex/config.yaml

# docker run
docker build -t iotex-bot:latest .

#put config.yaml and keystore in /etc/config.yaml(default path),set the right path to config.yaml

docker run -d -P --name bot -v /etc/iotex:/etc/iotex  iotex-bot bot -config-path=/etc/iotex/config.yaml

## License
This project is licensed under the [Apache License 2.0](LICENSE).
