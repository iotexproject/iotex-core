pragma solidity ^0.4.24;
contract MyContract {

  bool locked = false;

  modifier validAddress(address account) {
    if (account == 0x0) { throw; }
    _;
  }

  modifier greaterThan(uint value, uint limit) {
      if(value <= limit) { throw; }
      _;
  }

  modifier lock() {
    if(locked) {
        locked = true;
        _;
        locked = false;
    }
  }

  function f(address account) validAddress(account) {}
  function g(uint a) greaterThan(a, 10) {}
  function refund() lock {
      msg.sender.send(0);
  }
}