// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

contract MemoryCopyExample {
    function copyData() public pure returns (bytes memory) {
        bytes memory source = new bytes(5);
        bytes memory dest = new bytes(5);

        source[0] = 0x41; // 'A'
        source[1] = 0x42; // 'B'
        source[2] = 0x43; // 'C'
        source[3] = 0x44; // 'D'
        source[4] = 0x45; // 'E'

        assembly {
            mcopy(add(dest, 0x20), add(source, 0x20), 5)
        }

        return dest;
    }
}
