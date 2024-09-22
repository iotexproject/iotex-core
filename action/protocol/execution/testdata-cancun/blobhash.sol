// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract BlobHashTest {
    function getBlobHash() public view returns (bytes32) {
        bytes32 hash;
        assembly {
            hash := blobhash(0)
        }
        return hash;
    }
}