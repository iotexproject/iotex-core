// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

/**
 * @title NamespaceStorage
 * @dev Namespace-aware storage contract for key-value data with flexible value structure
 * This contract extends the GenericStorage concept with namespace support for data isolation
 */
contract NamespaceStorage {

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

    // Nested mapping: namespace => key => value
    mapping(string => mapping(bytes => GenericValue)) private namespaceStorage_;

    // Track keys for each namespace
    mapping(string => bytes[]) private namespaceKeys_;

    // Track if a key exists in a namespace
    mapping(string => mapping(bytes => bool)) private keyExists_;

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
     * @dev Store data with a given namespace and key
     * @param namespace The namespace for data isolation
     * @param key The storage key within the namespace
     * @param value The GenericValue struct to store
     */
    function put(
        string memory namespace,
        bytes memory key, 
        GenericValue memory value
    ) external {
        require(bytes(namespace).length > 0, "Namespace cannot be empty");
        require(key.length > 0, "Key cannot be empty");

        // Add namespace if it doesn't exist
        if (!namespaceExists_[namespace]) {
            namespaces_.push(namespace);
            namespaceExists_[namespace] = true;
        }

        // If key doesn't exist in this namespace, add it to keys array
        if (!keyExists_[namespace][key]) {
            namespaceKeys_[namespace].push(key);
            keyExists_[namespace][key] = true;
        }

        // Store the value
        namespaceStorage_[namespace][key] = value;

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
        keyExists = keyExists_[namespace][key];
        if (keyExists) {
            value = namespaceStorage_[namespace][key];
        }
        return (value, keyExists);
    }

    /**
     * @dev Delete data by namespace and key
     * @param namespace The namespace containing the key
     * @param key The storage key to delete
     */
    function remove(string memory namespace, bytes memory key) external {
        require(namespaceExists_[namespace], "Namespace does not exist");
        require(keyExists_[namespace][key], "Key does not exist in namespace");

        // Remove from storage
        delete namespaceStorage_[namespace][key];
        keyExists_[namespace][key] = false;

        // Remove from keys array
        bytes[] storage keys = namespaceKeys_[namespace];
        for (uint256 i = 0; i < keys.length; i++) {
            if (keccak256(keys[i]) == keccak256(key)) {
                // Move last element to current position and pop
                keys[i] = keys[keys.length - 1];
                keys.pop();
                break;
            }
        }

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
            existsFlags[i] = keyExists_[namespace][keyList[i]];
            if (existsFlags[i]) {
                values[i] = namespaceStorage_[namespace][keyList[i]];
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
    ) external {
        require(keys.length == values.length, "Keys and values arrays must have same length");
        require(bytes(namespace).length > 0, "Namespace cannot be empty");

        // Add namespace if it doesn't exist
        if (!namespaceExists_[namespace]) {
            namespaces_.push(namespace);
            namespaceExists_[namespace] = true;
        }

        for (uint256 i = 0; i < keys.length; i++) {
            require(keys[i].length > 0, "Key cannot be empty");

            // If key doesn't exist in this namespace, add it to keys array
            if (!keyExists_[namespace][keys[i]]) {
                namespaceKeys_[namespace].push(keys[i]);
                keyExists_[namespace][keys[i]] = true;
            }

            // Store the value
            namespaceStorage_[namespace][keys[i]] = values[i];

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

        // Fill result arrays
        for (uint256 i = 0; i < actualLimit; i++) {
            bytes memory key = keys[offset + i];
            keyList[i] = key;
            values[i] = namespaceStorage_[namespace][key];
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
            string memory ns = namespaces_[offset + i];
            namespaceList[i] = ns;
            counts[i] = namespaceKeys_[ns].length;
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
        return keyExists_[namespace][key];
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
    function clearNamespace(string memory namespace) external {
        require(namespaceExists_[namespace], "Namespace does not exist");

        bytes[] storage keys = namespaceKeys_[namespace];

        // Clear all data in the namespace
        for (uint256 i = 0; i < keys.length; i++) {
            bytes memory key = keys[i];
            delete namespaceStorage_[namespace][key];
            keyExists_[namespace][key] = false;
        }

        // Clear keys array for the namespace
        delete namespaceKeys_[namespace];

        emit NamespaceCleared(namespace);
    }

    /**
     * @dev Clear all stored data across all namespaces (emergency function)
     * Note: This function should be carefully protected in production
     */
    function clearAll() external {
        // Clear all namespaces
        for (uint256 i = 0; i < namespaces_.length; i++) {
            string memory namespace = namespaces_[i];
            bytes[] storage keys = namespaceKeys_[namespace];

            // Clear all data in this namespace
            for (uint256 j = 0; j < keys.length; j++) {
                bytes memory key = keys[j];
                delete namespaceStorage_[namespace][key];
                keyExists_[namespace][key] = false;
            }

            // Clear keys array for this namespace
            delete namespaceKeys_[namespace];
            namespaceExists_[namespace] = false;
        }

        // Clear namespaces array
        delete namespaces_;

        emit AllDataCleared();
    }
}