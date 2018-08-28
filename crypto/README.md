## CGO Binding Guide
### Software Requirement
* CMAKE (Version at least 3.10.2)
  * Install on MacOS:
  ```
    brew install cmake
   ```
  * Install on Linux:
  ```
    wget http://www.cmake.org/files/v3.11/cmake-3.11.0-Linux-x86_64.tar.gz
    tar -xvf cmake-3.11.0-Linux-x86_64.tar.gz
    cd cmake-3.11.0-Linux-x86_64
    sudo  cp -r bin /usr/
    sudo  cp -r share /usr/
    sudo  cp -r doc /usr/share/
    sudo  cp -r man /usr/share/
    ```



### CGO Development Guide
##### 1. Declare the use of CGO and files binding

``` Go
 //#include "lib/ecckey.h"
 //#include "lib/ecckey.c"
 import “C”
```
  * Just use import “C” to start using CGO. It comes with Go. No need to install any other library.
  * “//” is not a regular comment sign in CGO declaration. It starts a directive and will not be ignored by compiler.
  * import “C” should be immediately preceded by //#include XXX or //#cgo.
  * //#include XXX indicates the use of c header files and c source code files
  * //#cgo is used to indicate the use of flags like CFLAGS, CPPFLAGS, CXXFLAGS, FFLAGS and LDFLAGS etc. to tweak the behavior of C, C++ compiler. The //#cgo directive can also include a list of build constraints that limit the os and architecture. It can also indicates the C library, so that you do not have to include the source code files.
``` Go
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib -lsect283k1_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib -lsect283k1_ubuntu
```
This indicates the library sect283k1_macos will be used in Mac platform,           
sect283k1_ubuntu in Linux platform. 
Both of them are located in directory ${SRCDIR}/lib.
-L specifies the location of the library.
``` Go
package crypto

//#include "lib/ecckey.h"
//#include "lib/ecdsa.h"
//#include "lib/sect283k1.h"
//#cgo darwin LDFLAGS: -L${SRCDIR}/lib -lsect283k1_macos
//#cgo linux LDFLAGS: -L${SRCDIR}/lib -lsect283k1_ubuntu
import "C"
import (
    "bytes"
    "encoding/binary"
    "errors"

    "github.com/iotexproject/iotex-core/pkg/enc"
    "github.com/iotexproject/iotex-core/pkg/keypair"
)
```

##### 2. Go references C
  * Go calls C functions: Just call C.function_name_in_C(…)
  * Go uses any type in C: C.type_in_C, like C.uint32_t, C.ec_point_aff_twist
  * Pointer pass in CGO is the same as in go
  * Go can access any fields of C struct in the same way in Go struct
    ``` Go
    // C header file
    
    typedef struct
    {
        uint32_t d[9];         
        ec283_point_lambda_aff Q;
    }ec283_key_pair;
    
    void keypair_generation(ec283_key_pair *key);
    
    // Go file
    func (c *ec283) NewKeyPair() (keypair.PublicKey, keypair.PrivateKey, error) {
        var kp C.ec283_key_pair
        C.keypair_generation(&kp)
        pubKey := kp.Q
        privKey := kp.d
        ...
    }
    ```
  * Primitive type conversion: Just cast the type from C type to Go or from Go to C
    ``` Go
    // privkey has go type elements, privkeySer has C type elements
    for i := 0; i < privkeySize; i++ {
        privkeySer[i] = (C.uint32_t)(privkey[i])
    }
    
    // sigSer has C type elements, sig has go type elements
    for i, x := range sigSer {
        sig[i] = (uint32)(x)
    }
    ```
##### 3. Go passes an array to C

Just pass the first element of the slice as a pointer, and cast the pointer to C corresponding      
type. The slice in Go is a struct including header and elements, so &slice won’t give you the     
first element.
``` Go
// C file
// d and sig are arrays

uint32_t BLS_sign(uint32_t *d, const uint8_t *msg, uint64_t mlen, uint32_t *sig);

// Go file
// Sign signs a message given a private key

func (b *bls) Sign(privkey []uint32, msg []byte) (bool, []byte, error) {
    var sigSer [sigSize]C.uint32_t
    msgString := string(msg[:])

    if ok := C.BLS_sign((*C.uint32_t)(unsafe.Pointer(&privkey[0])), (*C.uint8_t)(&msg[0]), (C.uint64_t)(len(msgString)), &sigSer[0]); ok == 1 {
        sig, err := b.signatureSerialization(sigSer)
        if err != nil {
            return false, []byte{}, err
        }
        return true, sig, nil
    }
    return false, []byte{}, ErrSignError
}

```
##### 4. Use struct defined in C
  * Construct a corresponding struct in Go that mimics the same struct in C.
  * (Recommended) Write function to serialize and deserialize the the struct, so the information can be exposed outside independently.
    ``` Go
    // C header file and C struct definition
    typedef struct
    {
        uint32_t x[9];     
        uint32_t l[9];     
    }ec283_point_lambda_aff;
    
    // Go file
    
    func (*ec283) publicKeySerialization(pubKey C.ec283_point_lambda_aff) (keypair.PublicKey, error) {
        var xl [18]uint32
        for i := 0; i < 9; i++ {
            xl[i] = (uint32)(pubKey.x[i])
            xl[i+9] = (uint32)(pubKey.l[i])
        }
        buf := new(bytes.Buffer)
        err := binary.Write(buf, enc.MachineEndian, xl)
        if err != nil {
            return keypair.ZeroPublicKey, err
        }
        return keypair.BytesToPublicKey(buf.Bytes())
    }
    
    func (*ec283) publicKeyDeserialization(pubKey keypair.PublicKey) (C.ec283_point_lambda_aff, error) {
        var xl [18]uint32
        var pub C.ec283_point_lambda_aff
        rbuf := bytes.NewReader(pubKey[:])
        err := binary.Read(rbuf, enc.MachineEndian, &xl)
        if err != nil {
            return pub, err
        }
        for i := 0; i < 9; i++ {
            pub.x[i] = (C.uint32_t)(xl[i])
            pub.l[i] = (C.uint32_t)(xl[i+9])
        }
        return pub, nil
    }
    ```

