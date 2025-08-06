# Makefile for generating ABI and Go bindings from Solidity interface files

# Variables
SOLC := solc
TEMP_DIR := .temp

# Contract definitions
ACCOUNT_SOL := account_contract_interface.sol
ACCOUNT_ABI := account_contract_abi.json

# Default target
.PHONY: all
all: account-abi

# Generate all ABIs
.PHONY: abi
abi: account-abi

# Account contract targets
.PHONY: account-abi
account-abi: $(ACCOUNT_ABI)

$(ACCOUNT_ABI): $(ACCOUNT_SOL)
	@echo "Generating ABI from $(ACCOUNT_SOL)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(ACCOUNT_SOL) -o $(TEMP_DIR) --overwrite
	@if [ -f "$(TEMP_DIR)/IAccountProtocolContract.abi" ]; then \
		cat "$(TEMP_DIR)/IAccountProtocolContract.abi" | jq '.' > $(ACCOUNT_ABI); \
		echo "✅ Account ABI generated: $(ACCOUNT_ABI)"; \
	else \
		echo "❌ Error: Account ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)



# Clean generated files
.PHONY: clean
clean:
	@echo "Cleaning generated files..."
	@rm -f $(ACCOUNT_ABI)
	@rm -rf $(TEMP_DIR)

# Install dependencies (solc, jq)
.PHONY: install-deps
install-deps:
	@echo "Installing dependencies..."
	@if ! command -v solc &> /dev/null; then \
		echo "Installing solc..."; \
		if command -v npm &> /dev/null; then \
			npm install -g solc; \
		elif command -v brew &> /dev/null; then \
			brew install solidity; \
		else \
			echo "Please install solc manually"; \
		fi; \
	fi
	@if ! command -v jq &> /dev/null; then \
		echo "Installing jq..."; \
		if command -v brew &> /dev/null; then \
			brew install jq; \
		elif command -v apt-get &> /dev/null; then \
			sudo apt-get update && sudo apt-get install -y jq; \
		else \
			echo "Please install jq manually"; \
		fi; \
	fi

# Check if dependencies are installed
.PHONY: check-deps
check-deps:
	@echo "Checking dependencies..."
	@if ! command -v solc &> /dev/null; then \
		echo "❌ solc is not installed"; \
		echo "Run 'make install-deps' to install dependencies"; \
		exit 1; \
	else \
		echo "✅ solc is installed: $$(solc --version | head -1)"; \
	fi
	@if ! command -v jq &> /dev/null; then \
		echo "❌ jq is not installed"; \
		echo "Run 'make install-deps' to install dependencies"; \
		exit 1; \
	else \
		echo "✅ jq is installed: $$(jq --version)"; \
	fi

# Watch for changes and regenerate
.PHONY: watch
watch: check-deps
	@echo "Watching Solidity files for changes..."
	@while true; do \
		inotifywait -e modify $(ACCOUNT_SOL) 2>/dev/null || \
		(echo "inotifywait not available, using sleep instead"; sleep 5); \
		make all; \
		echo "Waiting for changes..."; \
	done

# Validate generated ABI files
.PHONY: validate
validate: $(ACCOUNT_ABI)
	@echo "Validating ABI files..."
	@if jq empty $(ACCOUNT_ABI) 2>/dev/null; then \
		echo "✅ Account ABI is valid JSON"; \
		echo "  Functions: $$(jq '[.[] | select(.type=="function")] | length' $(ACCOUNT_ABI))"; \
		echo "  Events: $$(jq '[.[] | select(.type=="event")] | length' $(ACCOUNT_ABI))"; \
	else \
		echo "❌ Account ABI is not valid JSON"; \
		exit 1; \
	fi



# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all           - Generate ABI for account contract (default)"
	@echo "  abi           - Generate ABI files for account contract"
	@echo ""
	@echo "Account contract:"
	@echo "  account-abi   - Generate account contract ABI"
	@echo ""
	@echo "Utilities:"
	@echo "  clean         - Remove all generated files"
	@echo "  install-deps  - Install solc and jq dependencies"
	@echo "  check-deps    - Check if dependencies are installed"
	@echo "  watch         - Watch for changes and regenerate ABI"
	@echo "  validate      - Validate generated ABI file"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Files:"
	@echo "  Account:  $(ACCOUNT_SOL) → $(ACCOUNT_ABI)"
