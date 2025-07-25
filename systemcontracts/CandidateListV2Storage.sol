// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

/**
 * @title CandidateListV2Storage
 * @dev Contract for storing and retrieving CandidateListV2 information
 * This contract implements storage functionality for CandidateListV2 with paginated access
 * supporting PutCandidates and GetCandidates operations
 */
contract CandidateListV2Storage {
    struct CandidateV2 {
        string ownerAddress;
        string operatorAddress;
        string rewardAddress;
        string name;
        string totalWeightedVotes;
        uint64 selfStakeBucketIdx;
        string selfStakingTokens;
        string id;
    }

    // Storage for candidate list
    CandidateV2[] private candidates;

    // Events
    event CandidatesStored(uint256 count);
    event CandidateListCleared();

    /**
     * @dev Store a complete candidate list (PutCandidates functionality)
     * @param candidateList Array of CandidateV2 to store
     */
    function putCandidates(CandidateV2[] memory candidateList) external {
        // Clear existing candidates
        delete candidates;

        // Store new candidates
        for (uint256 i = 0; i < candidateList.length; i++) {
            candidates.push(candidateList[i]);
        }

        emit CandidatesStored(candidateList.length);
    }

    /**
     * @dev Get candidates with pagination (GetCandidates functionality)
     * @param offset Starting index for pagination
     * @param limit Maximum number of candidates to return
     * @return candidateList Array of CandidateV2 within the specified range
     * @return total Total number of candidates in the list
     */
    function getCandidates(
        uint256 offset,
        uint256 limit
    ) external view returns (CandidateV2[] memory candidateList, uint256 total) {
        total = candidates.length;

        // Handle edge cases for pagination
        if (offset >= total) {
            // Return empty array if offset is beyond the list
            candidateList = new CandidateV2[](0);
            return (candidateList, total);
        }

        // Calculate actual limit considering remaining items
        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        // Create result array with actual size
        candidateList = new CandidateV2[](actualLimit);

        // Copy candidates from storage to result array
        for (uint256 i = 0; i < actualLimit; i++) {
            candidateList[i] = candidates[offset + i];
        }

        return (candidateList, total);
    }
}