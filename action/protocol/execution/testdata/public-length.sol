pragma solidity 0.4.24;
contract A {
    uint[] public nums;
    function getNumLength() returns(uint) {
        return nums.length;
    }
}

contract B {
    A a;
    function test() constant returns (uint) {
        a=new A();
        // length is not accessible on public array from other contract
        //return a.nums.length();
        return a.getNumLength();
    }
}