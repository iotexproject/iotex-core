// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract A {
    uint256[] xs;

    function Apush() internal {
        xs.push(100);
        xs.push(200);
        xs.push(300);
    }

    // can be called from web3
    function foo() public view returns (uint256[] memory) {
        return xs;
    }
}

// trying to call foo from another contract does not work
contract B {
    A a;

    function Bnew() internal {
        a = new A();
    }

    // COMPILATION ERROR
    // Return argument type inaccessible dynamic type is not implicitly convertible
    // to expected type (type of first return variable) uint256[] memory.
    function bar() public view returns (uint256[] memory) {
        return a.foo();
    }
}
