// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

/**
 * @title GenericStorage
 * @dev Generic storage contract for key-value data with flexible value structure
 * This contract provides a universal storage solution with support for:
 * - Basic CRUD operations (put, get, delete)
 * - Batch operations (batchGet)
 * - Listing all stored data
 * - Flexible value structure with immutable and mutable fields
 */
contract GenericStorage {

    /**
     * @dev Generic value structure with multiple fields for flexibility
     * @param primaryData Primary data field for main content
     * @param secondaryData Secondary data field for additional content
     * @param auxiliaryData Additional data field for extended use cases
     */
    struct GenericValue {
        bytes primaryData;      // Primary data field
        bytes secondaryData;    // Secondary data field
        bytes auxiliaryData;    // Additional data field for flexibility
    }

    // Main storage mapping
    mapping(bytes => GenericValue) private storage_;

    // Array to keep track of all keys for listing functionality
    bytes[] private keys_;

    // Mapping to check if a key exists (for efficient existence checks)
    mapping(bytes => bool) private keyExists_;

    // Events
    event DataStored(bytes indexed key);
    event DataDeleted(bytes indexed key);
    event BatchDataRetrieved(uint256 keyCount);
    event StorageCleared();

    /**
     * @dev Store data with a given key
     * @param key The storage key
     * @param value The GenericValue struct to store
     */
    function put(
        bytes memory key, 
        GenericValue memory value
    ) external {
        require(key.length > 0, "Key cannot be empty");

        // If key doesn't exist, add it to keys array
        if (!keyExists_[key]) {
            keys_.push(key);
            keyExists_[key] = true;
        }

        // Store the value
        storage_[key] = value;

        emit DataStored(key);
    }

    /**
     * @dev Delete data by key
     * @param key The storage key to delete
     */
    function remove(bytes memory key) external {
        require(keyExists_[key], "Key does not exist");

        // Remove from storage
        delete storage_[key];
        keyExists_[key] = false;

        // Remove from keys array
        for (uint256 i = 0; i < keys_.length; i++) {
            if (keccak256(keys_[i]) == keccak256(key)) {
                // Move last element to current position and pop
                keys_[i] = keys_[keys_.length - 1];
                keys_.pop();
                break;
            }
        }

        emit DataDeleted(key);
    }

    /**
     * @dev Get data by key
     * @param key The storage key
     * @return value The stored GenericValue struct
     * @return keyExists Whether the key exists
     */
    function get(bytes memory key) external view returns (GenericValue memory value, bool keyExists) {
        keyExists = keyExists_[key];
        if (keyExists) {
            value = storage_[key];
        }
        return (value, keyExists);
    }

    /**
     * @dev Batch get data by multiple keys
     * @param keyList Array of keys to retrieve
     * @return values Array of GenericValue structs
     * @return existsFlags Array indicating which keys exist
     */
    function batchGet(bytes[] memory keyList) external view returns (
        GenericValue[] memory values, 
        bool[] memory existsFlags
    ) {
        values = new GenericValue[](keyList.length);
        existsFlags = new bool[](keyList.length);

        for (uint256 i = 0; i < keyList.length; i++) {
            existsFlags[i] = keyExists_[keyList[i]];
            if (existsFlags[i]) {
                values[i] = storage_[keyList[i]];
            }
        }

        return (values, existsFlags);
    }

    /**
     * @dev List all stored data with pagination
     * @param offset Starting index for pagination
     * @param limit Maximum number of items to return
     * @return keyList Array of keys
     * @return values Array of corresponding GenericValue structs
     * @return total Total number of stored items
     */
    function list(uint256 offset, uint256 limit) external view returns (
        bytes[] memory keyList,
        GenericValue[] memory values,
        uint256 total
    ) {
        total = keys_.length;

        // Handle edge cases for pagination
        if (offset >= total) {
            keyList = new bytes[](0);
            values = new GenericValue[](0);
            return (keyList, values, total);
        }

        // Calculate actual limit
        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        // Create result arrays
        keyList = new bytes[](actualLimit);
        values = new GenericValue[](actualLimit);

        // Fill result arrays
        for (uint256 i = 0; i < actualLimit; i++) {
            bytes memory key = keys_[offset + i];
            keyList[i] = key;
            values[i] = storage_[key];
        }

        return (keyList, values, total);
    }

    /**
     * @dev List all keys only (lightweight version)
     * @param offset Starting index for pagination  
     * @param limit Maximum number of keys to return
     * @return keyList Array of keys
     * @return total Total number of stored items
     */
    function listKeys(uint256 offset, uint256 limit) external view returns (
        bytes[] memory keyList,
        uint256 total
    ) {
        total = keys_.length;

        if (offset >= total) {
            keyList = new bytes[](0);
            return (keyList, total);
        }

        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        keyList = new bytes[](actualLimit);

        for (uint256 i = 0; i < actualLimit; i++) {
            keyList[i] = keys_[offset + i];
        }

        return (keyList, total);
    }

    /**
     * @dev Check if a key exists
     * @param key The storage key to check
     * @return keyExists Whether the key exists
     */
    function exists(bytes memory key) external view returns (bool keyExists) {
        return keyExists_[key];
    }

    /**
     * @dev Get total number of stored items
     * @return totalCount Total number of items
     */
    function count() external view returns (uint256 totalCount) {
        return keys_.length;
    }

    /**
     * @dev Clear all stored data (emergency function)
     * Note: This function should be carefully protected in production
     */
    function clear() external {
        // Clear all mappings and arrays
        for (uint256 i = 0; i < keys_.length; i++) {
            bytes memory key = keys_[i];
            delete storage_[key];
            keyExists_[key] = false;
        }

        // Clear keys array
        delete keys_;

        emit StorageCleared();
    }
}