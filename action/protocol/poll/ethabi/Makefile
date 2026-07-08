# Variables
SOLC := solc
POLL_SOL_FILE := poll.sol
POLL_ABI_FILE := poll.abi.json
TEMP_DIR := .temp

# Default target
all: $(POLL_ABI_FILE)

# Generate ABI file
$(POLL_ABI_FILE): $(POLL_SOL_FILE)
	@echo "Generating ABI from $(POLL_SOL_FILE)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(POLL_SOL_FILE) -o $(TEMP_DIR) --overwrite
	@if [ -f "$(TEMP_DIR)/IPollProtocolContract.abi" ]; then \
		jq '.' "$(TEMP_DIR)/IPollProtocolContract.abi" > $(POLL_ABI_FILE); \
		echo "ABI generated and formatted successfully: $(POLL_ABI_FILE)"; \
	else \
		echo "Error: Poll contract ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f $(POLL_ABI_FILE)
	@rm -rf $(TEMP_DIR)

.PHONY: all clean
