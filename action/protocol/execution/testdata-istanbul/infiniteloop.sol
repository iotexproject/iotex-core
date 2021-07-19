pragma solidity ^0.8.4;

contract SimpleStorage {
   uint storedData;

   event Set(uint256);
   event Get(address, uint256);
   event Deadlock();

   function set(uint x) public {
       storedData = x;
       emit Set(x);
   }

   function get(address _to) public returns (uint) {
       require(_to != address(0));
       emit Get(_to, storedData);
       return storedData;
   }

   function infinite() public {
       while(true)
       {
           storedData++;
       }
       emit Deadlock();
   }
}
