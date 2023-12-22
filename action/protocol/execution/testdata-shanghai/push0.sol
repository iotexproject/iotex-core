// SPDX-License-Identifier: MIT

pragma solidity >=0.8.20;

contract SimpleStorage {

    uint256 favoriteNumber;

    function retrieve() public view returns (uint256){
        return favoriteNumber;
    }

}