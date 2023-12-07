// SPDX-License-Identifier: MIT

pragma solidity =0.8.17;

contract Difficulty {

    function diffi() public view returns (uint256 r) {
        assembly {
            r := difficulty()
        }
    }
}
