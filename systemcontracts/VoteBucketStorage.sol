// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

/**
 * @title VoteBucketStorage
 * @dev Contract for storing and retrieving VoteBucketList information
 * This contract implements storage functionality for VoteBucketList with paginated access
 * supporting PutBuckets and GetBuckets operations
 */
contract VoteBucketStorage {
    struct VoteBucket {
        uint64 index;
        string candidateAddress;
        string stakedAmount;
        uint32 stakedDuration;
        uint64 createTime;           // Timestamp as uint64 (Unix timestamp)
        uint64 stakeStartTime;       // Timestamp as uint64 (Unix timestamp)
        uint64 unstakeStartTime;     // Timestamp as uint64 (Unix timestamp)
        bool autoStake;
        string owner;
        string contractAddress;
        uint64 stakedDurationBlockNumber;
        uint64 createBlockHeight;
        uint64 stakeStartBlockHeight;
        uint64 unstakeStartBlockHeight;
        uint64 endorsementExpireBlockHeight;
    }

    // Storage for vote bucket list
    VoteBucket[] private buckets;

    // Events
    event BucketsStored(uint256 count);
    event BucketListCleared();

    /**
     * @dev Store a complete vote bucket list (PutBuckets functionality)
     * @param bucketList Array of VoteBucket to store
     */
    function putBuckets(VoteBucket[] memory bucketList) external {
        // Clear existing buckets
        delete buckets;

        // Store new buckets
        for (uint256 i = 0; i < bucketList.length; i++) {
            buckets.push(bucketList[i]);
        }

        emit BucketsStored(bucketList.length);
    }

    /**
     * @dev Get vote buckets with pagination (GetBuckets functionality)
     * @param offset Starting index for pagination
     * @param limit Maximum number of buckets to return
     * @return bucketList Array of VoteBucket within the specified range
     * @return total Total number of buckets in the list
     */
    function getBuckets(
        uint256 offset,
        uint256 limit
    ) external view returns (VoteBucket[] memory bucketList, uint256 total) {
        total = buckets.length;

        // Handle edge cases for pagination
        if (offset >= total) {
            // Return empty array if offset is beyond the list
            bucketList = new VoteBucket[](0);
            return (bucketList, total);
        }

        // Calculate actual limit considering remaining items
        uint256 remainingItems = total - offset;
        uint256 actualLimit = limit > remainingItems ? remainingItems : limit;

        // Create result array with actual size
        bucketList = new VoteBucket[](actualLimit);

        // Copy buckets from storage to result array
        for (uint256 i = 0; i < actualLimit; i++) {
            bucketList[i] = buckets[offset + i];
        }

        return (bucketList, total);
    }
}