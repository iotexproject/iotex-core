pragma solidity ^0.4.24;

import "./Ownable.sol";

contract Cashier is Ownable {
    address public safe;
    uint256 public depositFee;
    uint256 public minAmount;
    uint256 public maxAmount;
    uint256 public gasLimit;
    address[] public customers;
    uint256[] public amounts;
    bool paused;

    event Receipt(address indexed customer, uint256 amount, uint256 fee);

    function Cashier(address _safe, uint256 _fee, uint256 _minAmount, uint256 _maxAmount) public {
        require(_safe != address(0));
        require(_minAmount > 0);
        require(_maxAmount >= _minAmount);
        safe = _safe;
        depositFee = _fee;
        minAmount = _minAmount;
        maxAmount = _maxAmount;
        gasLimit = 4000000;
    }

    function () external payable {
        deposit();
    }

    function deposit() public payable {
        require(!paused);
        require(msg.value >= minAmount + depositFee);
        uint256 amount = msg.value - depositFee;
        require(amount <= maxAmount);

        if (safe.call.value(amount).gas(gasLimit)()) {
            customers.push(msg.sender);
            amounts.push(amount);

            emit Receipt(msg.sender, amount, depositFee);
        }
    }

    function withdraw(uint256 amount) external onlyOwner {
        require(address(this).balance >= amount);
        msg.sender.transfer(amount);
    }

    function setSafe(address _safe) external onlyOwner {
        require(_safe != address(0));
        safe = _safe;
    }

    function setMinAmount(uint256 _minAmount) external onlyOwner {
        require(maxAmount >= _minAmount);
        minAmount = _minAmount;
    }

    function setMaxAmount(uint256 _maxAmount) external onlyOwner {
        require(_maxAmount >= minAmount);
        maxAmount = _maxAmount;
    }

    function setDepositFee(uint256 _fee) external onlyOwner {
        depositFee = _fee;
    }

    function setGasLimit(uint256 _gasLimit) external onlyOwner {
        gasLimit = _gasLimit;
    }

    function pause() external onlyOwner {
        require(!paused);
        paused = true;
    }

    function resume() external onlyOwner {
        require(paused);
        paused = false;
    }

}