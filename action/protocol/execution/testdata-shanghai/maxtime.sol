// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract SimpleStorage {
    uint256[32] storedData;

    event Set(uint256 indexed);
    event Get(address, uint256);

    function set(uint256 x) public {
        for (uint256 i = 0; i < 32; i++) {
            storedData[i] = x;
        }
        emit Set(x);
    }

    function test1(uint256 loop) public returns (uint256) {
        uint256 x = 0;
        for (uint256 j = 0; j < loop; j++) {
            for (uint256 i = 0; i < 32; i++) {
                x += storedData[i];
                x = x >> 8;
                x = x * 257;
            }
        }
        emit Set(x);
        return x;
    }

    function store1(uint256 loop) public returns (uint256) {
        uint256 x = 0;
        for (uint256 j = 0; j < loop; j++) {
            for (uint256 i = 0; i < 32; i++) {
                x += storedData[i];
                x = x >> 8;
                x = x * 257;
            }
            storedData[j % 32] = x;
        }
        emit Set(x);
        return x;
    }

    function test2(uint256 loop) public returns (uint256) {
        uint256 x = 0;
        for (uint256 j = 0; j < loop; j++) {
            for (uint256 i = 0; i < 32; i++) {
                x += storedData[i];
                x = x >> 8;
            }
        }
        emit Set(x);
        return x;
    }
}
