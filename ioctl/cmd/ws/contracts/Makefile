.DEFAULT_GOAL := update

.PHONY: update
update: clean fetch generate_abi generate_go

.PHONY: fetch
fetch:
	@echo "#### clone sprout develop branch..."
	@mkdir sprout
	@cd sprout && git init --quiet
	@cd sprout && git remote add origin git@github.com:machinefi/sprout.git
	@cd sprout && git config core.sparsecheckout true
	@cd sprout && echo "smartcontracts" >> .git/info/sparse-checkout
	@cd sprout && git pull origin develop --depth=1 --quiet
	@echo DONE


.PHONY: generate_abi
generate_abi:
	@echo "#### generate abis from latest contracts..."
	@cd sprout/smartcontracts && yarn install > /dev/null 2>&1
	@mkdir -p abis && cd sprout/smartcontracts/contracts && for file in 'FleetManagement' 'ProjectRegistrar' 'W3bstreamProject' 'W3bstreamProver' 'ProjectDevice' 'W3bstreamRouter' 'W3bstreamVMType'; \
	do \
		solc --include-path ../node_modules/ --base-path . --optimize --abi --overwrite --pretty-json -o . $$file.sol > /dev/null 2>&1 ; \
		if [ -e $$file.abi ]; then \
			mv $$file.abi ../../../abis/$$file.json; \
		fi \
	done
	@echo DONE

.PHONY: generate_go
generate_go:
	@echo "#### generate go code by abis"
	@go generate .
	@echo DONE

.PHONY: clean
clean:
	@rm -rf sprout

