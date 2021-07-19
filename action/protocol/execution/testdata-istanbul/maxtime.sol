pragma solidity >=0.4.24;

contract SimpleStorage {
   uint[32] storedData;

   event Set(uint256 indexed);
   event Get(address, uint256);

   function set(uint x) public {
       for (uint i = 0; i < 32; i++) {
           storedData[i] = x;
       }
       emit Set(x);
   }

   function test1(uint loop) public returns (uint) {
       uint x = 0;
       for (uint j = 0; j < loop; j++) {
           for (uint i = 0; i < 32; i++) {
               x += storedData[i];
               x = x >> 8;
               x = x * 257;
           }
       }
       emit Set(x);
       return x;
   }

   function store1(uint loop) public returns (uint) {
       uint x = 0;
       for (uint j = 0; j < loop; j++) {
           for (uint i = 0; i < 32; i++) {
               x += storedData[i];
               x = x >> 8;
               x = x * 257;
           }
           storedData[j%32] = x;
       }
       emit Set(x);
       return x;
   }

   function test2(uint loop) public returns (uint) {
       uint x = 0;
       for (uint j = 0; j < loop; j++) {
           for (uint i = 0; i < 32; i++) {
               x += storedData[i];
               x = x >> 8;
           }
       }
       emit Set(x);
       return x;
   }
}
