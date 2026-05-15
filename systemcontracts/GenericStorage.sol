// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title GenericStorage
 * @dev Generic storage contract for key-value data with flexible value structure
 * This contract provides a universal storage solution with support for:
 * - Basic CRUD operations (put, get, delete)
 * - Batch operations (batchGet)
 * - Listing all stored data
 * - Flexible value structure with immutable and mutable fields
 * - Owner-only access control for modification operations
 */
contract GenericStorage is Ownable {

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

    // Array to keep track of all keys for listing functionality
    bytes[] private keys_;

    // Array to store values corresponding to keys (parallel arrays)
    GenericValue[] private values_;

    // Mapping to store the index of each key in the keys_ array for O(1) removal
    // Note: We store (actualIndex + 1) to distinguish between non-existent keys (0) and keys at index 0 (1)
    mapping(bytes => uint256) private keyIndex_;

    // Events
    event DataStored(bytes indexed key);
    event DataDeleted(bytes indexed key);
    event BatchDataRetrieved(uint256 keyCount);
    event StorageCleared();

    /**
     * @dev Constructor that sets the deployer as the owner
     */
    constructor() Ownable(msg.sender) {}

    /**
     * @dev Internal function to check if a key exists
     * @param key The storage key to check
     * @return Whether the key exists
     */
    function _keyExists(bytes memory key) private view returns (bool) {
        return keyIndex_[key] != 0;
    }

    /**
     * @dev Store data with a given key
     * @param key The storage key
     * @param value The GenericValue struct to store
     */
    function put(
        bytes memory key, 
        GenericValue memory value
    ) external onlyOwner {
        require(key.length > 0, "Key cannot be empty");

        // If key doesn't exist, add it to keys array
        if (!_keyExists(key)) {
            keyIndex_[key] = keys_.length + 1;  // Store (index + 1) to distinguish from non-existent keys
            keys_.push(key);
            values_.push(value);  // Add corresponding value to values array
        } else {
            // Update existing value
            values_[keyIndex_[key] - 1] = value;
        }

        emit DataStored(key);
    }

    /**
     * @dev Delete data by key
     * @param key The storage key to delete
     */
    function remove(bytes memory key) external onlyOwner {
        require(_keyExists(key), "Key does not exist");

        // Get the index of the key to remove (subtract 1 since we stored index + 1)
        uint256 indexToRemove = keyIndex_[key] - 1;
        uint256 lastIndex = keys_.length - 1;

        // If it's not the last element, move the last element to the removed position
        if (indexToRemove != lastIndex) {
            keys_[indexToRemove] = keys_[lastIndex];
            values_[indexToRemove] = values_[lastIndex];
            keyIndex_[keys_[lastIndex]] = indexToRemove + 1;  // Update the moved key's index (add 1)
        }

        // Remove the last elements from both arrays
        keys_.pop();
        values_.pop();
        delete keyIndex_[key];

        emit DataDeleted(key);
    }

    /**
     * @dev Get data by key
     * @param key The storage key
     * @return value The stored GenericValue struct
     * @return keyExists Whether the key exists
     */
    function get(bytes memory key) external view returns (GenericValue memory value, bool keyExists) {
        keyExists = _keyExists(key);
        if (keyExists) {
            value = values_[keyIndex_[key] - 1];  // Get value from values array
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
            existsFlags[i] = _keyExists(keyList[i]);
            if (existsFlags[i]) {
                values[i] = values_[keyIndex_[keyList[i]] - 1];  // Get value from values array
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

        // Fill result arrays - much more efficient with parallel arrays
        for (uint256 i = 0; i < actualLimit; i++) {
            uint256 arrayIndex = offset + i;
            keyList[i] = keys_[arrayIndex];
            values[i] = values_[arrayIndex];  // Direct array access, no mapping lookup needed
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
        return _keyExists(key);
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
    function clear() external onlyOwner {
        // Clear all mappings and arrays
        for (uint256 i = 0; i < keys_.length; i++) {
            bytes memory key = keys_[i];
            delete keyIndex_[key];
        }

        // Clear both arrays
        delete keys_;
        delete values_;

        emit StorageCleared();
    }
}