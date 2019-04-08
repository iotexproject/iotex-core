pragma solidity 0.4.24;
contract A {
    
    uint[] xs;
    
    function A() {
        xs.push(100);
        xs.push(200);
        xs.push(300);
    }
    
    // can be called from web3
    function foo() constant returns(uint[]) {
        return xs;
    }
}

// trying to call foo from another contract does not work
contract B {
    
    A a;
    
    function B() {
        a = new A();
    }
    
    // COMPILATION ERROR
    // Return argument type inaccessible dynamic type is not implicitly convertible 
    // to expected type (type of first return variable) uint256[] memory.
    function bar() constant returns(uint[]) {
        return a.foo();
    }
}
