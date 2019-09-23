pragma solidity >=0.4.24;

contract SimpleStorage {
   uint[32] storedData;

   event Set(uint256 indexed);
   event Get(address, uint256);
   event Deadlock();

   function set(uint x) public {
       for (uint i = 0; i < 32; i++) {
           storedData[i] = x;
       }
       emit Set(x);
   }

   function test1() public returns (uint) {
       uint x = 0;
       for (uint j = 0; j < 2; j++) {
           for (uint i = 0; i < 32; i++) {
               x += storedData[i];
               x = x >> 8;
               x = x * 301;
               x = x * 12;
           }
       }
       emit Set(x);
       return x;
   }
   function test2() public returns (uint) {
       uint x = 0;
       for (uint j = 0; j < 1; j++) {
           for (uint i = 0; i < 16; i++) {
               x += storedData[i];
               x = x >> 8;
               x = x << 12; 
           }
       }
       emit Set(x);
       return x;
   }
}
