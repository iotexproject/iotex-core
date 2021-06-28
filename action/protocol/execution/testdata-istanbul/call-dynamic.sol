pragma solidity ^0.8.4;

contract MyContract {

    uint x = 0;

    function foo(uint256 _x) public {
        x = 10 + _x;
    }

    function bar() public returns(uint) {
        address(this).call(abi.encodeWithSignature('foo(uint256)', 1));
        return x; // returns 11
    }
}
