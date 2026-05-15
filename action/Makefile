# Makefile for generating ABI from Solidity interface files

# Variables
SOLC := solc
NATIVE_SOL_FILE := native_staking_contract_interface.sol
NATIVE_ABI_FILE := native_staking_contract_abi.json
ACCOUNT_SOL_FILE := account_contract_interface.sol
ACCOUNT_ABI_FILE := account_contract_abi.json
TEMP_DIR := .temp

# Default target
.PHONY: all
all: abi

# Generate all ABI files
.PHONY: abi
abi: native-abi account-abi

# Generate ABI from native staking contract interface
.PHONY: native-abi
native-abi: $(NATIVE_ABI_FILE)

$(NATIVE_ABI_FILE): $(NATIVE_SOL_FILE)
	@echo "Generating ABI from $(NATIVE_SOL_FILE)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(NATIVE_SOL_FILE) -o $(TEMP_DIR) --overwrite
	@# Extract the ABI from the generated file and format it
	@if [ -f "$(TEMP_DIR)/INativeStakingContract.abi" ]; then \
		cat "$(TEMP_DIR)/INativeStakingContract.abi" | jq '.' > $(NATIVE_ABI_FILE); \
		echo "ABI generated successfully: $(NATIVE_ABI_FILE)"; \
	else \
		echo "Error: Native staking ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)

# Generate ABI from account contract interface
.PHONY: account-abi
account-abi: $(ACCOUNT_ABI_FILE)

$(ACCOUNT_ABI_FILE): $(ACCOUNT_SOL_FILE)
	@echo "Generating ABI from $(ACCOUNT_SOL_FILE)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(ACCOUNT_SOL_FILE) -o $(TEMP_DIR) --overwrite
	@# Extract the ABI from the generated file and format it
	@if [ -f "$(TEMP_DIR)/IAccountProtocolContract.abi" ]; then \
		cat "$(TEMP_DIR)/IAccountProtocolContract.abi" | jq '.' > $(ACCOUNT_ABI_FILE); \
		echo "ABI generated successfully: $(ACCOUNT_ABI_FILE)"; \
	else \
		echo "Error: Account contract ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)

# Clean generated files
.PHONY: clean
clean:
	@echo "Cleaning generated files..."
	@rm -f $(NATIVE_ABI_FILE) $(ACCOUNT_ABI_FILE)
	@rm -rf $(TEMP_DIR)

# Install dependencies (solc and jq)
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

# Watch for changes and regenerate ABI
.PHONY: watch
watch: check-deps
	@echo "Watching $(NATIVE_SOL_FILE) and $(ACCOUNT_SOL_FILE) for changes..."
	@while true; do \
		inotifywait -e modify $(NATIVE_SOL_FILE) $(ACCOUNT_SOL_FILE) 2>/dev/null || \
		(echo "inotifywait not available, using sleep instead"; sleep 5); \
		make abi; \
		echo "Waiting for changes..."; \
	done

# Validate generated ABI files
.PHONY: validate
validate: $(NATIVE_ABI_FILE) $(ACCOUNT_ABI_FILE)
	@echo "Validating ABI files..."
	@echo "Validating $(NATIVE_ABI_FILE)..."
	@if jq empty $(NATIVE_ABI_FILE) 2>/dev/null; then \
		echo "✅ Native staking ABI file is valid JSON"; \
		echo "Functions found: $$(jq '[.[] | select(.type=="function")] | length' $(NATIVE_ABI_FILE))"; \
		echo "Events found: $$(jq '[.[] | select(.type=="event")] | length' $(NATIVE_ABI_FILE))"; \
	else \
		echo "❌ Native staking ABI file is not valid JSON"; \
		exit 1; \
	fi
	@echo "Validating $(ACCOUNT_ABI_FILE)..."
	@if jq empty $(ACCOUNT_ABI_FILE) 2>/dev/null; then \
		echo "✅ Account contract ABI file is valid JSON"; \
		echo "Functions found: $$(jq '[.[] | select(.type=="function")] | length' $(ACCOUNT_ABI_FILE))"; \
		echo "Events found: $$(jq '[.[] | select(.type=="event")] | length' $(ACCOUNT_ABI_FILE))"; \
	else \
		echo "❌ Account contract ABI file is not valid JSON"; \
		exit 1; \
	fi

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  abi           - Generate all ABI files from Solidity interfaces (default)"
	@echo "  native-abi    - Generate ABI from native staking contract interface"
	@echo "  account-abi   - Generate ABI from account contract interface"
	@echo "  clean         - Remove generated files"
	@echo "  install-deps  - Install solc and jq dependencies"
	@echo "  check-deps    - Check if dependencies are installed"
	@echo "  watch         - Watch for changes and regenerate ABI"
	@echo "  validate      - Validate generated ABI files"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Files:"
	@echo "  Input:  $(NATIVE_SOL_FILE) -> Output: $(NATIVE_ABI_FILE)"
	@echo "  Input:  $(ACCOUNT_SOL_FILE) -> Output: $(ACCOUNT_ABI_FILE)"
