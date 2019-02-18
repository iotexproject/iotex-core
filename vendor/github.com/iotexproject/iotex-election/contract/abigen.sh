#!/bin/bash

abigen --abi iotx.abi --pkg contract --type IOTX --out iotx.go
abigen --abi register.abi --pkg contract --type Register --out register.go
abigen --abi staking.abi --pkg contract --type Staking --out staking.go
