// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract Contract {
    uint256[] public values;

    function find(uint256 value) internal view returns (uint256) {
        uint256 i = 0;
        while (values[i] != value) {
            i++;
        }
        return i;
    }

    function removeByValue(uint256 value) public {
        uint256 i = find(value);
        removeByIndex(i);
    }

    function removeByIndex(uint256 i) public {
        while (i < values.length - 1) {
            values[i] = values[i + 1];
            i++;
        }
        values.pop();
    }

    function getValues() internal view returns (uint256[] storage) {
        return values;
    }

    function test() public returns (uint256[] memory) {
        values.push(10);
        values.push(20);
        values.push(30);
        values.push(40);
        values.push(50);
        removeByValue(30);
        uint256 i = find(40);
        removeByIndex(i);
        return getValues();
    }
}
