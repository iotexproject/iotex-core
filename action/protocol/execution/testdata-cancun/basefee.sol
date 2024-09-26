// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract BaseFeeTest {
    function getBaseFee() public view returns (uint256) {
        uint256 baseFee;
        assembly {
            baseFee := basefee()
        }
        return baseFee;
    }
}