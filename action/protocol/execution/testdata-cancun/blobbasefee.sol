// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract BlobBaseFeeTest {
    function getBlobBaseFee() public view returns (uint256) {
        uint256 blobBaseFee;
        assembly {
            blobBaseFee := blobbasefee()
        }
        return blobBaseFee;
    }
}