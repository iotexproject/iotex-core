pragma solidity 0.4.24;

contract A {
  bytes8[] stuff;
  function get() constant returns(bytes8[]) {
    return stuff;
  }
}

contract B {
  A a;
  bytes8[] mystuff;
  function assign(address _a) {
      a = A(_a);
  }

  function copyToMemory() {
    // VM does not support variably-sized return types from external function calls
    // (ERROR: Type inaccessible dynamic type is not implicitly convertible...)
    bytes8[] memory stuff = a.get();
  }

  function copyToStorage() {
    // ERROR
    mystuff = a.get();
  }
}