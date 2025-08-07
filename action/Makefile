# Makefile for generating ABI from Solidity interface files

# Variables
SOLC := solc
SOL_FILE := native_staking_contract_interface.sol
ABI_FILE := native_staking_contract_abi.json
TEMP_DIR := .temp

# Default target
.PHONY: all
all: abi

# Generate ABI from Solidity interface
.PHONY: abi
abi: $(ABI_FILE)

$(ABI_FILE): $(SOL_FILE)
	@echo "Generating ABI from $(SOL_FILE)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(SOL_FILE) -o $(TEMP_DIR) --overwrite
	@# Extract the ABI from the generated file and format it
	@if [ -f "$(TEMP_DIR)/INativeStakingContract.abi" ]; then \
		cat "$(TEMP_DIR)/INativeStakingContract.abi" | jq '.' > $(ABI_FILE); \
		echo "ABI generated successfully: $(ABI_FILE)"; \
	else \
		echo "Error: ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)

# Clean generated files
.PHONY: clean
clean:
	@echo "Cleaning generated files..."
	@rm -f $(ABI_FILE)
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
	@echo "Watching $(SOL_FILE) for changes..."
	@while true; do \
		inotifywait -e modify $(SOL_FILE) 2>/dev/null || \
		(echo "inotifywait not available, using sleep instead"; sleep 5); \
		make abi; \
		echo "Waiting for changes..."; \
	done

# Validate generated ABI
.PHONY: validate
validate: $(ABI_FILE)
	@echo "Validating ABI file..."
	@if jq empty $(ABI_FILE) 2>/dev/null; then \
		echo "✅ ABI file is valid JSON"; \
		echo "Functions found: $$(jq '[.[] | select(.type=="function")] | length' $(ABI_FILE))"; \
		echo "Events found: $$(jq '[.[] | select(.type=="event")] | length' $(ABI_FILE))"; \
	else \
		echo "❌ ABI file is not valid JSON"; \
		exit 1; \
	fi

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  abi           - Generate ABI from Solidity interface (default)"
	@echo "  clean         - Remove generated files"
	@echo "  install-deps  - Install solc and jq dependencies"
	@echo "  check-deps    - Check if dependencies are installed"
	@echo "  watch         - Watch for changes and regenerate ABI"
	@echo "  validate      - Validate generated ABI file"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Files:"
	@echo "  Input:  $(SOL_FILE)"
	@echo "  Output: $(ABI_FILE)"
