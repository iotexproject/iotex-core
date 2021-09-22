pragma solidity ^0.8.3;

contract Datacopy {
    bytes store;
    function dataCopy() public {
        bytes memory arr = new bytes(3);
        arr[0] = 0x11;
        arr[1] = 0x22;
        arr[2] = 0x33;
        uint length = arr.length;
        bytes memory result = new bytes(3);
        bool ret;
        assembly {
            // Call precompiled contract to copy data
            ret := staticcall(0x10000, 0x04, add(arr, 0x20), length, add(arr, 0x21), length)
            returndatacopy(add(result, 0x20), 0x00, length)
        }
        updateStore(result);
    }
    
    function updateStore(bytes memory ret) internal {
        store = ret;
    }
}
