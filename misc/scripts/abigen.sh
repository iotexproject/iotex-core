#!/bin/bash

abigen --abi election/staking.abi --pkg election --type StakingContract --out election/stakingcontract.go.tmp

sed 's/github.com\/ethereum/github.com\/iotexproject/g' election/stakingcontract.go.tmp > election/stakingcontract.go

rm election/stakingcontract.go.tmp
