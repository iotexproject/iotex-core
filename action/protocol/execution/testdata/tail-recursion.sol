pragma solidity 0.4.24;

contract MyContract {

    // naive recursion
    function sum(uint n) constant returns(uint) {
        return n == 0 ? 0 :
          n + sum(n-1);
    }

    // loop
    function sumloop(uint n) constant returns(uint) {
        uint256 total = 0;
        for(uint256 i=1; i<=n; i++) {
          total += i;
        }
        return total;
    }

    // tail-recursion
    function sumtailHelper(uint n, uint acc) private constant returns(uint) {
        return n == 0 ? acc :
          sumtailHelper(n-1, acc + n);
    }
    function sumtail(uint n) constant returns(uint) {
        return sumtailHelper(n, 0);
    }
}
