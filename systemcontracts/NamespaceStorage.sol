// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title NamespaceStorage
 * @dev Namespace-aware storage contract for key-value data with flexible value structure
 * This contract extends the GenericStorage concept with namespace support for data isolation
 * - Owner-only access control for modification operations
 */
contract NamespaceStorage is Ownable {

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

    // Track keys for each namespace (parallel arrays)
    mapping(string => bytes[]) private namespaceKeys_;
    mapping(string => GenericValue[]) private namespaceValues_;

    // Mapping to store the index of each key in the namespace's keys array for O(1) removal
    // Note: We store (actualIndex + 1) to distinguish between non-existent keys (0) and keys at index 0 (1)
    mapping(string => mapping(bytes => uint256)) private keyIndex_;

    // Track all namespaces
    string[] private namespaces_;
    mapping(string => bool) private namespaceExists_;

    // Events for tracking operations
    event DataStored(string indexed namespace, bytes indexed key);
    event DataDeleted(string indexed namespace, bytes indexed key);
    event BatchDataRetrieved(string indexed namespace, uint256 keyCount);
    event NamespaceCleared(string indexed namespace);
    event AllDataCleared();

    /**
     * @dev Constructor that sets the deployer as the owner
     */
    constructor() Ownable(msg.sender) {}

    /**
     * @dev Internal function to check if a key exists in a namespace
     * @param namespace The namespace to check
     * @param key The storage key to check
     * @return Whether the key exists in the namespace
     */
    function _keyExists(string memory namespace, bytes memory key) private view returns (bool) {
        return keyIndex_[namespace][key] != 0;
    }

    /**
     * @dev Store data with a given namespace and key
     * @param namespace The namespace for data isolation
     * @param key The storage key within the namespace
     * @param value The GenericValue struct to store
     */
    function put(
        string memory namespace,
        bytes memory key, 
        GenericValue memory value
    ) external onlyOwner {
        require(bytes(namespace).length > 0, "Namespace cannot be empty");
        require(key.length > 0, "Key cannot be empty");

        // Add namespace if it doesn't exist
        if (!namespaceExists_[namespace]) {
            namespaces_.push(namespace);
            namespaceExists_[namespace] = true;
        }

        // If key doesn't exist in this namespace, add it to both arrays
        if (!_keyExists(namespace, key)) {
            keyIndex_[namespace][key] = namespaceKeys_[namespace].length + 1;  // Store (index + 1)
            namespaceKeys_[namespace].push(key);
            namespaceValues_[namespace].push(value);
        } else {
            // Update existing value
            namespaceValues_[namespace][keyIndex_[namespace][key] - 1] = value;
        }

        emit DataStored(namespace, key);
    }

    /**
     * @dev Get data by namespace and key
     * @param namespace The namespace containing the key
     * @param key The storage key
     * @return value The stored GenericValue struct
     * @return keyExists Whether the key exists in the namespace
     */
    function get(string memory namespace, bytes memory key) external view returns (
        GenericValue memory value, 
        bool keyExists
    ) {
        keyExists = _keyExists(namespace, key);
        if (keyExists) {
            value = namespaceValues_[namespace][keyIndex_[namespace][key] - 1];
        }
        return (value, keyExists);
    }

    /**
     * @dev Delete data by namespace and key
     * @param namespace The namespace containing the key
     * @param key The storage key to delete
     */
    function remove(string memory namespace, bytes memory key) external onlyOwner {
        require(namespaceExists_[namespace], "Namespace does not exist");
        require(_keyExists(namespace, key), "Key does not exist in namespace");

        // Get the index of the key to remove (subtract 1 since we stored index + 1)
        uint256 indexToRemove = keyIndex_[namespace][key] - 1;
        uint256 lastIndex = namespaceKeys_[namespace].length - 1;

        // If it's not the last element, move the last element to the removed position
        if (indexToRemove != lastIndex) {
            namespaceKeys_[namespace][indexToRemove] = namespaceKeys_[namespace][lastIndex];
            namespaceValues_[namespace][indexToRemove] = namespaceValues_[namespace][lastIndex];
            keyIndex_[namespace][namespaceKeys_[namespace][lastIndex]] = indexToRemove + 1;  // Update the moved key's index (add 1)
        }

        // Remove the last elements from both arrays
        namespaceKeys_[namespace].pop();
        namespaceValues_[namespace].pop();
        delete keyIndex_[namespace][key];

        emit DataDeleted(namespace, key);
    }

    /**
     * @dev Batch get data by multiple keys within a namespace
     * @param namespace The namespace to query
     * @param keyList Array of keys to retrieve
     * @return values Array of GenericValue structs
     * @return existsFlags Array indicating which keys exist
     */
    function batchGet(string memory namespace, bytes[] memory keyList) external view returns (
        GenericValue[] memory values, 
        bool[] memory existsFlags
    ) {
        values = new GenericValue[](keyList.length);
        existsFlags = new bool[](keyList.length);

        for (uint256 i = 0; i < keyList.length; i++) {
            existsFlags[i] = _keyExists(namespace, keyList[i]);
            if (existsFlags[i]) {
                values[i] = namespaceValues_[namespace][keyIndex_[namespace][keyList[i]] - 1];
            }
        }

        return (values, existsFlags);
    }

    /**
     * @dev Batch put multiple key-value pairs in the same namespace
     * @param namespace The namespace for all data
     * @param keys Array of keys to store
     * @param values Array of values to store
     */
    function batchPut(
        string memory namespace,
        bytes[] memory keys,
        GenericValue[] memory values
    ) external onlyOwner {
        require(keys.length == values.length, "Keys and values arrays must have same length");
        require(bytes(namespace).length > 0, "Namespace cannot be empty");

        // Add namespace if it doesn't exist
        if (!namespaceExists_[namespace]) {
            namespaces_.push(namespace);
            namespaceExists_[namespace] = true;
        }

        for (uint256 i = 0; i < keys.length; i++) {
            require(keys[i].length > 0, "Key cannot be empty");

            // If key doesn't exist in this namespace, add it to both arrays
            if (!_keyExists(namespace, keys[i])) {
                keyIndex_[namespace][keys[i]] = namespaceKeys_[namespace].length + 1;
                namespaceKeys_[namespace].push(keys[i]);
                namespaceValues_[namespace].push(values[i]);
            } else {
                // Update existing value
                namespaceValues_[namespace][keyIndex_[namespace][keys[i]] - 1] = values[i];
            }

            emit DataStored(namespace, keys[i]);
        }
    }

    /**
     * @dev List all stored data in a namespace with pagination
     * @param namespace The namespace to list
     * @param offset Starting index for pagination
     * @param limit Maximum number of items to return
     * @return keyList Array of keys
     * @return values Array of corresponding GenericValue structs
     * @return total Total number of stored items in the namespace
     */
    function list(string memory namespace, uint256 offset, uint256 limit) external view returns (
        bytes[] memory keyList,
        GenericValue[] memory values,
        uint256 total
    ) {
        bytes[] storage keys = namespaceKeys_[namespace];
        total = keys.length;

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
            keyList[i] = keys[arrayIndex];
            values[i] = namespaceValues_[namespace][arrayIndex];  // Direct array access, no mapping lookup
        }

        return (keyList, values, total);
    }

    /**
     * @dev List all keys in a namespace (lightweight version)
     * @param namespace The namespace to list
     * @param offset Starting index for pagination  
     * @param limit Maximum number of keys to return
     * @return keyList Array of keys
     * @return total Total number of stored items in the namespace
     */
    function listKeys(string memory namespace, uint256 offset, uint256 limit) external view returns (
        bytes[] memory keyList,
        uint256 total
    ) {
        bytes[] storage keys = namespaceKeys_[namespace];
        total = keys.length;

        if (offset >= total) {
            keyList = new bytes[](0);
            return (keyList, total);
        }

        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        keyList = new bytes[](actualLimit);

        for (uint256 i = 0; i < actualLimit; i++) {
            keyList[i] = keys[offset + i];
        }

        return (keyList, total);
    }

    /**
     * @dev List all namespaces with pagination
     * @param offset Starting index for pagination
     * @param limit Maximum number of namespaces to return
     * @return namespaceList Array of namespace names
     * @return counts Array of item counts for each namespace
     * @return total Total number of namespaces
     */
    function listNamespaces(uint256 offset, uint256 limit) external view returns (
        string[] memory namespaceList,
        uint256[] memory counts,
        uint256 total
    ) {
        total = namespaces_.length;

        if (offset >= total) {
            namespaceList = new string[](0);
            counts = new uint256[](0);
            return (namespaceList, counts, total);
        }

        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        namespaceList = new string[](actualLimit);
        counts = new uint256[](actualLimit);

        for (uint256 i = 0; i < actualLimit; i++) {
            namespaceList[i] = namespaces_[offset + i];
            counts[i] = namespaceKeys_[namespaces_[offset + i]].length;
        }

        return (namespaceList, counts, total);
    }

    /**
     * @dev Check if a key exists in a namespace
     * @param namespace The namespace to check
     * @param key The storage key to check
     * @return keyExists Whether the key exists in the namespace
     */
    function exists(string memory namespace, bytes memory key) external view returns (bool keyExists) {
        return _keyExists(namespace, key);
    }

    /**
     * @dev Check if a namespace exists
     * @param namespace The namespace to check
     * @return nsExists Whether the namespace exists
     */
    function hasNamespace(string memory namespace) external view returns (bool nsExists) {
        return namespaceExists_[namespace];
    }

    /**
     * @dev Get total number of stored items in a namespace
     * @param namespace The namespace to count
     * @return itemCount Total number of items in the namespace
     */
    function countInNamespace(string memory namespace) external view returns (uint256 itemCount) {
        return namespaceKeys_[namespace].length;
    }

    /**
     * @dev Get total number of namespaces
     * @return totalNamespaces Total number of namespaces
     */
    function namespaceCount() external view returns (uint256 totalNamespaces) {
        return namespaces_.length;
    }

    /**
     * @dev Get total number of items across all namespaces
     * @return totalItems Total number of items across all namespaces
     */
    function totalCount() external view returns (uint256 totalItems) {
        for (uint256 i = 0; i < namespaces_.length; i++) {
            totalItems += namespaceKeys_[namespaces_[i]].length;
        }
        return totalItems;
    }

    /**
     * @dev Clear all data in a specific namespace
     * @param namespace The namespace to clear
     */
    function clearNamespace(string memory namespace) external onlyOwner {
        require(namespaceExists_[namespace], "Namespace does not exist");

        bytes[] storage keys = namespaceKeys_[namespace];

        // Clear all key indices in the namespace
        for (uint256 i = 0; i < keys.length; i++) {
            delete keyIndex_[namespace][keys[i]];
        }

        // Clear both arrays for the namespace
        delete namespaceKeys_[namespace];
        delete namespaceValues_[namespace];

        emit NamespaceCleared(namespace);
    }

    /**
     * @dev Clear all stored data across all namespaces (emergency function)
     * Note: This function should be carefully protected in production
     */
    function clearAll() external onlyOwner {
        // Clear all namespaces
        for (uint256 i = 0; i < namespaces_.length; i++) {
            string memory namespace = namespaces_[i];
            bytes[] storage keys = namespaceKeys_[namespace];

            // Clear all key indices in this namespace
            for (uint256 j = 0; j < keys.length; j++) {
                delete keyIndex_[namespace][keys[j]];
            }

            // Clear both arrays for this namespace
            delete namespaceKeys_[namespace];
            delete namespaceValues_[namespace];
            namespaceExists_[namespace] = false;
        }

        // Clear namespaces array
        delete namespaces_;

        emit AllDataCleared();
    }
}