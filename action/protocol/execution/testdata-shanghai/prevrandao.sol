// SPDX-License-Identifier: MIT

pragma solidity >=0.8.18;

contract Prevrandao {

    function random() public view returns (uint256 r) {
        assembly {
            r := prevrandao()
        }
    }
}
