// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

/**
 * @title CandidateStorage
 * @dev Contract for storing and retrieving candidate information
 * This contract implements the storage functionality for the CandidateList
 * as currently implemented in the Go code.
 */
contract CandidateStorage {
    struct Candidate {
        address candidateAddress;
        uint256 votes;
        address rewardAddress;
        bytes32 canName; // Limited to 32 bytes as per Go implementation
    }

    // Storage for candidate list
    mapping(bytes32 => mapping(uint256 => Candidate)) private candidatesByNsKey;
    mapping(bytes32 => uint256) private candidateCountByNsKey;
    
    // Events
    event CandidateStored(bytes32 indexed nsKey, uint256 indexed index, address candidateAddress);
    event CandidateListCleared(bytes32 indexed nsKey);

    /**
     * @dev Store a complete candidate list
     * @param ns The namespace string
     * @param key The key bytes
     * @param candidates Array of candidates to store
     */
    function storeCandidateList(
        string memory ns,
        bytes memory key,
        Candidate[] memory candidates
    ) external {
        bytes32 nsKey = keccak256(abi.encodePacked(ns, key));
        
        // Clear existing candidates for this nsKey
        _clearCandidateList(nsKey);
        
        // Store new candidates
        uint256 validCount = 0;
        for (uint256 i = 0; i < candidates.length; i++) {
            if (candidates[i].candidateAddress != address(0)) {
                candidatesByNsKey[nsKey][validCount] = candidates[i];
                emit CandidateStored(nsKey, validCount, candidates[i].candidateAddress);
                validCount++;
            }
        }
        
        candidateCountByNsKey[nsKey] = validCount;
    }

    /**
     * @dev Get all candidates for a given namespace and key
     * @param ns The namespace string
     * @param key The key bytes
     * @return Array of all candidates
     */
    function getCandidateList(
        string memory ns,
        bytes memory key
    ) external view returns (Candidate[] memory) {
        bytes32 nsKey = keccak256(abi.encodePacked(ns, key));
        uint256 count = candidateCountByNsKey[nsKey];
        
        Candidate[] memory candidates = new Candidate[](count);
        for (uint256 i = 0; i < count; i++) {
            candidates[i] = candidatesByNsKey[nsKey][i];
        }
        
        return candidates;
    }

    /**
     * @dev Internal function to clear candidate list
     * @param nsKey The hashed namespace key
     */
    function _clearCandidateList(bytes32 nsKey) private {
        uint256 count = candidateCountByNsKey[nsKey];
        
        for (uint256 i = 0; i < count; i++) {
            delete candidatesByNsKey[nsKey][i];
        }
        
        candidateCountByNsKey[nsKey] = 0;
        emit CandidateListCleared(nsKey);
    }
}
