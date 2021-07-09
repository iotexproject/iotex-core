pragma solidity ^0.8.4;

contract MyContract {

    // naive recursion
    function sum(uint n) public returns(uint) {
        return n == 0 ? 0 :
          n + sum(n-1);
    }

    // loop
    function sumloop(uint n) pure public returns(uint) {
        uint256 total = 0;
        for(uint256 i=1; i<=n; i++) {
          total += i;
        }
        return total;
    }

    // tail-recursion
    function sumtailHelper(uint n, uint acc) private returns(uint) {
        return n == 0 ? acc :
          sumtailHelper(n-1, acc + n);
    }
    function sumtail(uint n) public returns(uint) {
        return sumtailHelper(n, 0);
    }
}
