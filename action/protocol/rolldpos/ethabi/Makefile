# Variables
SOLC := solc
ROLLDPOS_SOL_FILE := rolldpos.sol
ROLLDPOS_ABI_FILE := rolldpos.abi.json
TEMP_DIR := .temp

# Default target
all: $(ROLLDPOS_ABI_FILE)

# Generate ABI file
$(ROLLDPOS_ABI_FILE): $(ROLLDPOS_SOL_FILE)
	@echo "Generating ABI from $(ROLLDPOS_SOL_FILE)..."
	@mkdir -p $(TEMP_DIR)
	@$(SOLC) --abi $(ROLLDPOS_SOL_FILE) -o $(TEMP_DIR) --overwrite
	@if [ -f "$(TEMP_DIR)/IRollDPoSProtocolContract.abi" ]; then \
		jq '.' "$(TEMP_DIR)/IRollDPoSProtocolContract.abi" > $(ROLLDPOS_ABI_FILE); \
		echo "ABI generated and formatted successfully: $(ROLLDPOS_ABI_FILE)"; \
	else \
		echo "Error: RollDPoS contract ABI file not generated"; \
		exit 1; \
	fi
	@rm -rf $(TEMP_DIR)

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f $(ROLLDPOS_ABI_FILE)
	@rm -rf $(TEMP_DIR)

.PHONY: all clean
