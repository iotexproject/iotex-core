// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract A {
    bytes8[] stuff;

    function get() public view returns (bytes8[] memory) {
        return stuff;
    }
}

contract B {
    A a;
    bytes8[] mystuff;

    function assign(address _a) public {
        a = A(_a);
    }

    function copyToMemory() public view {
        // VM does not support variably-sized return types from external function calls
        // (ERROR: Type inaccessible dynamic type is not implicitly convertible...)
        bytes8[] memory stuff = a.get();
    }

    function copyToStorage() public {
        // ERROR
        mystuff = a.get();
    }
}
