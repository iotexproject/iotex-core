pragma solidity ^0.4.24;

contract BasicToken {

  mapping(address => uint256) balances;

  function transfer(address recipient, uint256 value) public {
    balances[msg.sender] -= value;
    balances[recipient] += value;
  }

  function balanceOf(address account) public constant returns (uint256) {
    return balances[account];
  }

}
