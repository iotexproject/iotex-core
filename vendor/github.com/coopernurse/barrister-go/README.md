# Barrister for Go

This project contains Go bindings for the Barrister RPC system.

[![build status](https://secure.travis-ci.org/coopernurse/barrister-go.png)](http://travis-ci.org/coopernurse/barrister-go)

For information on how to write a Barrister IDL, please visit:

http://barrister.bitmechanic.com/

## Installation

```sh
# Install the barrister translator (IDL -> JSON)
# you need to be root (or use sudo)
pip install barrister

# Install barrister-go
go get github.com/coopernurse/barrister-go
go install github.com/coopernurse/barrister-go/idl2go
```

## Run tests

To run the included unit tests:

```sh
go get github.com/couchbaselabs/go.assert
go test
```

## Run example

### HTTP transport

```sh
# Generate Go code from calc.idl
cd $GOPATH/src/github.com/coopernurse/barrister-go/example
barrister calc.idl | $GOPATH/bin/idl2go -i -p calc

# Compile and run server in background
go run server.go &

# Compile and run client
go run client.go
```

### Iris transport

Iris is a messaging system that maps very cleanly to JSON-RPC request/response semantics.
Iris provides a clusterable message relay that eliminates the need to manually wire 
endpoints together, which in a micro service architecture is quite helpful.

Barrister Iris support is very preliminary, but you can run a simple example by:

```sh
# Fetch Iris Go client
go get gopkg.in/project-iris/iris-go.v1

# Generate Go code from calc.idl
cd $GOPATH/src/github.com/coopernurse/barrister-go/example
barrister calc.idl | $GOPATH/bin/idl2go -i -p calc

# Compile and run server in background
go run iris-calc-server.go &

# Compile and run client
go run iris-calc-client.go

```

## API documentation

http://godoc.org/github.com/coopernurse/barrister-go

## idl2go usage

idl2go generates a .go file based on the IDL JSON.  If the IDL contains namespaced 
enums or structs, the namespaced elements will be written to separate .go files.

The IDL JSON file is embedded in the generated .go file, so it is not needed
at runtime.

Usage info: `idl2go -h`

Examples:

```sh
# Loads auth.json and generates ./auth/auth.go
idl2go -p auth auth.json

# Reads IDL JSON from STDIN and generates /tmp/designsvc/designsvc.go
idl2go -p designsvc -i -d /tmp
```
## Writing clients

To write a Barrister client in Go:

* Run idl2go to generate a Go package based on the Barrister IDL .json file
* Use the generated Proxy struct in your program

Example:

```go
package main

import (
	"fmt"
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
)

// Accepts an URL to the endpoint of the Calculator server
// and returns a proxy that implements the Calculator interface
func NewCalculatorProxy(url string) calc.Calculator {
    // HttpTransport is provided, but you could write your
    // own transport (e.g. websockets, zeromq, etc)
	trans := &barrister.HttpTransport{Url: url}

	client := barrister.NewRemoteClient(trans, true)

	// calc.NewCalculatorProxy() is provided by the idl2go
	// generated calc.go file
	return calc.NewCalculatorProxy(client)
}

func main() {
    // create the proxy
	calc := NewCalculatorProxy("http://localhost:9233")

    // call methods on the proxy as if they were local functions
	res, err := calc.Add(51, 22.3)
	if err == nil {
		fmt.Println("Success!  51+22.3 =", res)
	} else {
		fmt.Println("ERROR! ", err)
	}

}
```

## Writing servers

To write a Barrister server in Go:

* Run idl2go to generate a Go package based (same step as writing a client)
* Write structs that implement the interfaces in the IDL
* Expose the service via HTTP (or other transport)

See `example/server.go` for a basic example.

### Thread safety

By default interface implementations (aka "services") must be thread safe.
A single shared instance will be used for all RPC requests.

If your struct implements `barrister.Cloneable` then `CloneForReq()` will
be called per request, and your service interface methods can safely
mutate state on the service struct for the lifespan of a single method
invocation.

### Security / Transport Headers

Often security is implemented via transport headers (e.g. HTTP Auth headers
or session cookies).  `Cloneable.CloneForReq()` is passed a map of the
transport headers for the request.  Consequently, you can copy this map
to your struct and use these headers in the body of your methods to 
implement whatever rules you like.  For example:

```go
type MySecureService struct {
	userId   string
}

// implements Cloneable interface
func (s MySecureService) CloneForReq(headers Headers) interface{} {
	userId := ""
	cookie := barrister.GetFirst(headers.Request, "MySecureCookie")

	if cookie != "" {
		// decode cookie somehow, perhaps returning empty string on failure
        userId = decodeUserId(cookie)
	}

	return MySecureService{userId}
}

// fictitious method from the IDL
func (s MySecureService) SaveSecretStuff(req SaveReq) (SaveResp, error) {
	err := s.authorize()
	if err != nil {
	    return SaveResp{}, err
	}

	// do real work

	return SaveResp{}, nil
}

func (s MySecureService) authorize() error {
	if s.userId == "" {
        return &barrister.JsonRpcError{Code: 1000, Message: "Unauthorized"}
	}

    // OK
	return nil
}
```

### Filters

Filters may be added to the Server instance.  Filter are separate from interface
implementations and run on all requests handled by the server, regardless of 
method.

Filters may handle the request and terminate request processing.  They may also 
perform some passive action and allow request processing to continue normally.

Example from `example/server.go`

```go
type LogFilter struct{}

func (f LogFilter) PreInvoke(r *barrister.RequestResponse) bool {
	fmt.Println("LogFilter: PreInvoke of method:", r.Method)

	// returning true continues request processing normally
	return true
}

func (f LogFilter) PostInvoke(r *barrister.RequestResponse) bool {
	fmt.Println("LogFilter: PostInvoke of method:", r.Method)
	return true
}

func main() {
	idl := barrister.MustParseIdlJson([]byte(calc.IdlJsonRaw))

	// create barrister.Server instance
	svr := calc.NewJSONServer(idl, true, CalculatorImpl{})

	// register filter
	svr.AddFilter(LogFilter{})

	http.Handle("/", &svr)
	fmt.Println("Starting Calculator server on localhost:9233")
	err := http.ListenAndServe(":9233", nil)
	if err != nil {
		panic(err)
	}
}
```
