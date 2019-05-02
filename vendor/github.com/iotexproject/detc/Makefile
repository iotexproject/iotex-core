# Go parameters
GOCMD=go

# Solidity and EVM parameters
ABIGENCMD=abigen

.PHONY: abigen
abigen:
	$(GOCMD) run tools/abigen.go -path=build/contracts/DetcImpl.json > detc.abi
	$(ABIGENCMD) --abi detc.abi --pkg detc --type DetcGen --out detc_gen.go
