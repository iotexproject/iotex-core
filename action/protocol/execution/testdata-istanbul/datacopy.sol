pragma solidity ^0.8.3;

contract Datacopy {
    function dataCopy() public view returns (bool) {
        bytes memory arr = new bytes(3);
        arr[0] = 0x11;
        arr[1] = 0x22;
        arr[2] = 0x00;
        uint length = arr.length;
        bytes memory result = new bytes(length);
        bool ret;
        assembly {
            // Call precompiled contract to copy data
            ret :=staticcall(0x10000, 0x04, add(arr, 0x20), length, add(arr, 0x21), length)
        }
        return ret;
    }
}