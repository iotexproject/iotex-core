pragma solidity ^0.4.24;

contract MyContract {

    uint x = 0;

    function foo(uint _x) {
        x = 10 + _x;
    }

    function bar() constant returns(uint) {
        this.call(bytes4(sha3('foo(uint256)')), 1);
        return x; // returns 11
    }
}
