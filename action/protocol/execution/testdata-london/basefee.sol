// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract One {
    function getBaseFee() public view returns (uint256) {
        return block.basefee;
    }
}
