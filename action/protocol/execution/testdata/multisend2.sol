pragma solidity ^0.4.24;

import './math/SafeMath.sol';
import './ownership/Ownable.sol';

contract ERC20Basic {
    function totalSupply() public view returns (uint256);
    function balanceOf(address who) public view returns (uint256);
    function transfer(address to, uint256 value) public returns (bool);
    event Transfer(address indexed from, address indexed to, uint256 value);
}

contract ERC20 is ERC20Basic {
    function allowance(address owner, address spender) public view returns (uint256);
    function transferFrom(address from, address to, uint256 value) public returns (bool);
    function approve(address spender, uint256 value) public returns (bool);
    event Approval(address indexed owner, address indexed spender, uint256 value);
}

contract Multisend is Ownable {
    using SafeMath for uint256;

    uint256 public minTips;
    uint256 public limit;
    
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Receipt(address _token, uint256 _totalAmount, uint256 _tips, string _payload);
    event Withdraw(address _owner, uint256 _balance);

    constructor(uint256 _minTips, uint256 _limit) public {
        minTips = _minTips;
        limit = _limit;
    }
    
    function sendCoin(address[] recipients, uint256[] amounts, string payload) public payable {
        require(recipients.length == amounts.length);
        require(recipients.length <= limit);
        uint256 totalAmount = minTips;
        for (uint256 i = 0; i < recipients.length; i++) {
            totalAmount = SafeMath.add(totalAmount, amounts[i]);
            require(msg.value >= totalAmount);
            recipients[i].transfer(amounts[i]);
            emit Transfer(msg.sender, recipients[i], amounts[i]);
        }
        emit Receipt(address(this), SafeMath.sub(totalAmount, minTips), SafeMath.add(minTips, SafeMath.sub(msg.value, totalAmount)), payload);
    }

    function sendToken(address token, address[] recipients, uint256[] amounts, string payload) public payable {
        require(msg.value >= minTips);
        require(recipients.length == amounts.length);
        require(recipients.length <= limit);
        uint256 totalAmount = 0;
        ERC20 erc20token = ERC20(token);
        for (uint256 i = 0; i < recipients.length; i++) {
            totalAmount = SafeMath.add(totalAmount, amounts[i]);
            erc20token.transferFrom(msg.sender, recipients[i], amounts[i]);
        }
        emit Receipt(token, totalAmount, msg.value, payload);
    }

    function withdraw() public onlyOwner {
        uint256 balance = address(this).balance;
        owner.transfer(balance);
        emit Withdraw(owner, balance);
    }

    function setMinTips(uint256 _minTips) public onlyOwner {
        minTips = _minTips;
    }

    function setLimit(uint256 _limit) public onlyOwner {
        limit = _limit;
    }
}
