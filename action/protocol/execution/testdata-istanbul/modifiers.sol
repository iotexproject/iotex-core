pragma solidity ^0.8.4;
contract MyContract {

  bool locked = false;

  modifier validAddress(address account) {
    if (account == address(0x0)) { revert(); }
    _;
  }

  modifier greaterThan(uint value, uint limit) {
      if(value <= limit) { revert(); }
      _;
  }

  modifier lock() {
    if(locked) {
        locked = true;
        _;
        locked = false;
    }
  }

  function f(address account) public validAddress(account) {}
  function g(uint a) public greaterThan(a, 10) {}
  function refund() public payable {
      payable(msg.sender).transfer(0);
  }
}
