package barrister

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

var zeroVal reflect.Value

// randHex generates a random array of bytes and
// returns the value as a hex encoded string
func randHex(bytes int) string {
	buf := make([]byte, bytes)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

//////////////////////////////////////////////////
// IDL //
/////////

// ParseIdlJsonFile loads the IDL JSON from the given filename
func ParseIdlJsonFile(filename string) (*Idl, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseIdlJson(b)
}

// ParseIdlJson parses the given IDL JSON
func ParseIdlJson(jsonData []byte) (*Idl, error) {

	elems := []IdlJsonElem{}
	err := json.Unmarshal(jsonData, &elems)
	if err != nil {
		return nil, err
	}

	return NewIdl(elems), nil
}

// MustParseIdlJson calls ParseIdlJson and panics if an error is returned
func MustParseIdlJson(jsonData []byte) *Idl {
	idl, err := ParseIdlJson(jsonData)
	if err != nil {
		panic(err)
	}
	return idl
}

// NewIdl creates a new Idl struct based on the slice of elements
// parsed from the IDL JSON document
func NewIdl(elems []IdlJsonElem) *Idl {
	idl := &Idl{
		elems:      elems,
		interfaces: map[string][]Function{},
		methods:    map[string]Function{},
		structs:    map[string]*Struct{},
		enums:      map[string][]EnumValue{},
	}

	for _, el := range elems {
		if el.Type == "meta" {
			idl.Meta = Meta{el.BarristerVersion, el.DateGenerated * 1000000, el.Checksum}
		} else if el.Type == "interface" {
			funcs := []Function{}
			for _, f := range el.Functions {
				meth := fmt.Sprintf("%s.%s", el.Name, f.Name)
				idl.methods[meth] = f
				funcs = append(funcs, f)
			}
			idl.interfaces[el.Name] = funcs
		} else if el.Type == "struct" {
			idl.structs[el.Name] = &Struct{Name: el.Name, Extends: el.Extends, Fields: el.Fields}
		} else if el.Type == "enum" {
			idl.enums[el.Name] = el.Values
		}
	}

	idl.computeAllStructFields()

	return idl
}

// A single element in the IDL JSON.  This struct is the union of
// all possible fields that may occur on an element.  The "type" field
// determines what fields are relevant for a given element.
type IdlJsonElem struct {
	// common fields
	Type    string `json:"type"`
	Name    string `json:"name"`
	Comment string `json:"comment"`

	// type=comment
	Value string `json:"value"`

	// type=struct
	Extends string  `json:"extends"`
	Fields  []Field `json:"fields"`

	// type=enum
	Values []EnumValue `json:"values"`

	// type=interface
	Functions []Function `json:"functions"`

	// type=meta
	BarristerVersion string `json:"barrister_version"`
	DateGenerated    int64  `json:"date_generated"`
	Checksum         string `json:"checksum"`
}

// Represents a function on an IDL interface
type Function struct {
	Name    string  `json:"name"`
	Comment string  `json:"comment"`
	Params  []Field `json:"params"`
	Returns Field   `json:"returns"`
}

// Represents an IDL struct
type Struct struct {
	Name    string
	Extends string
	Fields  []Field

	// fields in this struct, and its parents
	allFields []Field
}

// Represents a single Field on a struct or Function param
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional"`
	IsArray  bool   `json:"is_array"`
	Comment  string `json:"comment"`
}

func (f Field) goType(idl *Idl, optionalToPtr bool, pkgToStrip string) string {
	if f.IsArray {
		f2 := Field{f.Name, f.Type, false, false, ""}

		prefix := "[]"
		if f.Optional && optionalToPtr {
			prefix = "*[]"
		}
		return prefix + f2.goType(idl, optionalToPtr, pkgToStrip)
	}

	_, isStruct := idl.structs[f.Type]

	prefix := ""
	if f.Optional && (optionalToPtr || isStruct) {
		prefix = "*"
	}

	switch f.Type {
	case "string":
		return prefix + "string"
	case "int":
		return prefix + "int64"
	case "float":
		return prefix + "float64"
	case "bool":
		return prefix + "bool"
	}

	return prefix + capitalizeAndStripMatchingPkg(f.Type, pkgToStrip)
}

func (f Field) zeroVal(idl *Idl, optionalToPtr bool, pkgToStrip string) interface{} {

	if f.Optional && optionalToPtr {
		return "nil"
	}

	if f.IsArray {
		return f.goType(idl, false, pkgToStrip) + "{}"
	}

	switch f.Type {
	case "string":
		return `""`
	case "int":
		return "int64(0)"
	case "float":
		return "float64(0)"
	case "bool":
		return "false"
	}

	s, ok := idl.structs[f.Type]
	if ok {
		if f.Optional {
			return "nil"
		} else {
			return capitalizeAndStripMatchingPkg(s.Name, pkgToStrip) + "{}"
		}
	}

	e, ok := idl.enums[f.Type]
	if ok && len(e) > 0 {
		return `""`
	}

	msg := fmt.Sprintf("Unable to create val for field: %s type: %s",
		f.Name, f.Type)
	panic(msg)
}

func (f Field) testVal(idl *Idl) interface{} {
	return f.testValRecur(idl, make(map[string]interface{}))
}

func (f Field) testValRecur(idl *Idl, seenTypes map[string]interface{}) interface{} {

	if f.IsArray {
		key := fmt.Sprintf("array %s", f.Type)
		_, ok := seenTypes[key]
		if ok {
			// we've seen this array type, so return
			// an empty slice now to avoid cycles
			return make([]interface{}, 0)
		}

		f2 := Field{f.Name, f.Type, f.Optional, false, ""}
		arr := make([]interface{}, 1, 1)
		seenTypes[key] = arr
		arr[0] = f2.testValRecur(idl, seenTypes)
		return arr
	}

	switch f.Type {
	case "string":
		return "testval"
	case "int":
		return int64(99)
	case "float":
		return float64(10.3)
	case "bool":
		return true
	}

	s, ok := idl.structs[f.Type]
	if ok {
		key := fmt.Sprintf("struct %s", f.Type)
		seenVal, ok := seenTypes[key]
		if ok {
			return seenVal
		}

		val := map[string]interface{}{}
		seenTypes[key] = val
		for _, f2 := range s.allFields {
			val[f2.Name] = f2.testValRecur(idl, seenTypes)
		}
		return val
	}

	e, ok := idl.enums[f.Type]
	if ok && len(e) > 0 {
		return e[0].Value
	}

	msg := fmt.Sprintf("Unable to create val for field: %s type: %s", f.Name, f.Type)
	panic(msg)
}

// Represents a single element in an IDL enum
type EnumValue struct {
	Value   string `json:"value"`
	Comment string `json:"comment"`
}

type Meta struct {
	// Version number of the Python barrister translator that produced the
	// JSON translation of the IDL
	BarristerVersion string

	// When the IDL was translated to JSON represented as nanoseconds since epoch (UnixNano)
	DateGenerated int64

	// Checksum of the IDL
	Checksum string
}

// Represents a single Barrister IDL file
type Idl struct {
	// raw data from IDL file
	elems []IdlJsonElem

	// meta information about the contract
	Meta Meta

	// hashed elements
	interfaces map[string][]Function
	methods    map[string]Function
	structs    map[string]*Struct
	enums      map[string][]EnumValue
}

func (idl *Idl) computeAllStructFields() {
	for _, s := range idl.structs {
		s.allFields = idl.computeStructFields(s, []Field{})
	}
}

func (idl *Idl) computeStructFields(toAdd *Struct, allFields []Field) []Field {
	if toAdd.Extends != "" {
		parent, ok := idl.structs[toAdd.Extends]
		if ok {
			allFields = idl.computeStructFields(parent, allFields)
		}
	}

	for _, f := range toAdd.Fields {
		allFields = append(allFields, f)
	}

	return allFields
}

// GenerateGo generates Go source code for the given Idl.  A map is returned whose keys are
// the Go package names and values are the source code for that package.
//
// Typically you'll use the idl2go binary as a front end to this method, but this method is exposed
// if you wish to write your own code generation tooling.
//
// defaultPkgName - Go package name to use for non-namespaced elements.  Since interfaces are never
// namespaced in Barrister, all interfaces will be generated into this package.
//
// baseImport - Base Go import path to use for internal imports.  For example, if "myproject" is provided,
// and two packages "usersvc" and "common" are resolved, then "usersvc" will import "myproject/common"
//
// optionalToPtr - If true struct fields marked `[optional]` will be generated as Go pointers.  If false,
// they will be generated as non-pointer types with `omitempty` in the JSON tag.  Note: due to the
// behavior of `encoding/json`, all nested struct fields marked optional will be generated as
// pointers.  Otherwise there is no way to omit those fields from the struct during marshaling.
//
func (idl *Idl) GenerateGo(defaultPkgName string, baseImport string, optionalToPtr bool) map[string][]byte {
	pkgNameToGoCode := make(map[string][]byte)
	for _, nsIdl := range partitionIdlByNamespace(idl, defaultPkgName) {
		g := generateGo{idl,
			nsIdl.idl,
			nsIdl.pkgName,
			optionalToPtr,
			nsIdl.imports,
			baseImport}
		pkgNameToGoCode[nsIdl.pkgName] = g.generate()
	}
	return pkgNameToGoCode
}

// Method returns the Function related to the given method.
// The method must be fully qualified with the interface name.
// For example: "UserService.save"
func (idl *Idl) Method(name string) Function {
	return idl.methods[name]
}

type namespacedIdl struct {
	idl     *Idl
	pkgName string
	imports []string
}

func partitionIdlByNamespace(idl *Idl, defaultPkgName string) []namespacedIdl {
	var metaElem IdlJsonElem
	pkgNameToIdlElems := make(map[string][]IdlJsonElem)
	for _, elem := range idl.elems {
		if elem.Type == "meta" {
			metaElem = elem
		} else {
			pkg, _ := splitNs(elem.Name)
			if pkg == "" {
				pkg = defaultPkgName
			}

			elems, ok := pkgNameToIdlElems[pkg]
			if !ok {
				elems = make([]IdlJsonElem, 0)
			}
			pkgNameToIdlElems[pkg] = append(elems, elem)
		}
	}

	nsIdl := make([]namespacedIdl, 0)
	for pkg, elems := range pkgNameToIdlElems {
		elems = append(elems, metaElem)
		idl := NewIdl(elems)
		imports := findAllImports(pkg, elems)
		nsIdl = append(nsIdl, namespacedIdl{idl, pkg, imports})
	}
	return nsIdl
}

// splits name at the first period and returns the two parts
// e.g. "hello.World" returns: `"hello", "World"`
//
// If name does not contain a period then: "", name is returned
// e.g. "hello" returns: `"", "hello"`
func splitNs(name string) (string, string) {
	i := strings.Index(name, ".")
	if i > -1 && i < (len(name)-1) {
		return name[0:i], name[i+1:]
	}
	return "", name
}

// finds all imported IDL namespaces in the given elements
func findAllImports(pkgToIgnore string, elems []IdlJsonElem) []string {
	imports := make([]string, 0)
	for _, elem := range elems {
		ns, _ := splitNs(elem.Extends)
		imports = addIfNotInSlice(ns, pkgToIgnore, imports)

		for _, f := range elem.Fields {
			ns, _ := splitNs(f.Type)
			imports = addIfNotInSlice(ns, pkgToIgnore, imports)
		}

		for _, fx := range elem.Functions {
			ns, _ := splitNs(fx.Returns.Type)
			imports = addIfNotInSlice(ns, pkgToIgnore, imports)
			for _, f := range fx.Params {
				ns, _ := splitNs(f.Type)
				imports = addIfNotInSlice(ns, pkgToIgnore, imports)
			}
		}
	}

	return imports
}

// Adds a string to the given slice if it is not empty, not equal to toIgnore,
// and not already in the slice
func addIfNotInSlice(a string, toIgnore string, list []string) []string {
	if a != "" && a != toIgnore && !stringInSlice(a, list) {
		return append(list, a)
	}
	return list
}

// Returns true if a is in the given slice, or false if it is not
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//////////////////////////////////////////////////
// Request / Response //
////////////////////////

// JsonRpcError represents a JSON-RPC 2.0 Request
type JsonRpcRequest struct {
	// Version of the JSON-RPC protocol.  Always "2.0"
	Jsonrpc string `json:"jsonrpc"`

	// An identifier established by the client that uniquely identifies the request
	Id string `json:"id"`

	// Name of the method to be invoked
	Method string `json:"method"`

	// Parameter values to be used during the invocation of the method
	Params interface{} `json:"params"`
}

// JsonRpcError represents a JSON-RPC 2.0 Error
type JsonRpcError struct {
	// Indicates the error type that occurred
	Code int `json:"code"`

	// Short description of the error
	Message string `json:"message"`

	// Optional value that contains additional information about the error
	Data interface{} `json:"data,omitempty"`
}

func (e *JsonRpcError) Error() string {
	return fmt.Sprintf("JsonRpcError: code=%d message=%s", e.Code, e.Message)
}

// JsonRpcResponse represents a JSON-RPC 2.0 Response
type JsonRpcResponse struct {
	// Version of the JSON-RPC protocol.  Always "2.0"
	Jsonrpc string `json:"jsonrpc"`

	// Id will match the related JsonRpcRequest.Id
	Id string `json:"id"`

	// Error will be nil if the request was successful
	Error *JsonRpcError `json:"error,omitempty"`

	// Result from a successful request
	Result interface{} `json:"result,omitempty"`
}

// RequestResponse holds the request method and params and the
// handler instance that the method resolves to.
//
// This struct is used with the Filter interface and allows
// Filter implementations to inspect the request, mutate the
// handler (e.g. set out of band authentication information),
// and set the result/error (e.g. to terminate an unauthorized request)
type RequestResponse struct {
	// from Transport (e.g. HTTP headers)
	Headers Headers

	// from JsonRpcRequest
	Method string
	Params []interface{}

	// handler instance that will process
	// this request method - this is passed
	// by value, so only pointer fields in the handler
	// may be modified by filters
	Handler interface{}

	// to JsonRpcResponse
	Result interface{}
	Err    error
}

// GetFirst returns the first value associated with the given
// key, or an empty string if no value is found with that key
func GetFirst(m map[string][]string, key string) string {
	return GetFirstDefault(m, key, "")
}

// GetFirstDefault returns the first value associated with the given
// key, or defaultVal if no value is found with that key
func GetFirstDefault(m map[string][]string, key string, defaultVal string) string {
	xs, ok := m[key]
	if ok && len(xs) > 0 {
		return xs[0]
	}

	return defaultVal
}

// toJsonRpcError returns a JsonRpcError with code -32000 and
// an empty data field.
func toJsonRpcError(method string, err error) *JsonRpcError {
	if err == nil {
		return nil
	}

	e, ok := err.(*JsonRpcError)
	if ok {
		return e
	}
	msg := fmt.Sprintf("barrister: method '%s' raised unknown error: %v", method, err)
	return &JsonRpcError{Code: -32000, Message: msg}
}

//////////////////////////////////////////////////
// Client //
////////////

// EncodeASCII returns the given byte slice with non-ASCII
// runes encoded as unicode escape sequences (e.g. "\uXX" or "\UXXXX")
func EncodeASCII(b []byte) (*bytes.Buffer, error) {
	in := bytes.NewBuffer(b)
	out := bytes.NewBufferString("")
	for {
		r, size, err := in.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if size == 1 {
			// TODO: we need to check for characters above 127
			out.WriteRune(r)
		} else if size == 2 {
			out.WriteString(fmt.Sprintf("\\u%04x", r))
		} else {
			out.WriteString(fmt.Sprintf("\\U%08x", r))
		}
	}
	return out, nil
}

// Serializers encapsulate marshaling bytes to and from Go types.
type Serializer interface {
	Marshal(in interface{}) ([]byte, error)
	Unmarshal(in []byte, out interface{}) error

	// returns true if b represents a JSON-RPC batch request
	IsBatch(b []byte) bool
	MimeType() string
}

// JsonSerializer implements Serializer using the `encoding/json` package
type JsonSerializer struct {
	// If true values will be encoded with the `EncodeASCII` function
	// when marshaled
	ForceASCII bool
}

func (s *JsonSerializer) Marshal(in interface{}) ([]byte, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	if s.ForceASCII {
		buf, err := EncodeASCII(b)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return b, nil
}

func (s *JsonSerializer) Unmarshal(in []byte, out interface{}) error {
	return json.Unmarshal(in, out)
}

// IsBatch scans b looking for '[' or '{' - if '[' occurs
// first then true is returned.
func (s *JsonSerializer) IsBatch(b []byte) bool {
	batch := false
	for i := 0; i < len(b); i++ {
		if b[i] == '{' {
			break
		} else if b[i] == '[' {
			batch = true
			break
		}
	}
	return batch
}

// Returns "application/json"
func (s *JsonSerializer) MimeType() string {
	return "application/json"
}

// The Transport interface abstracts sending a serialized byte slice
type Transport interface {
	Send(in []byte) ([]byte, error)
}

// HttpTransport sends requests via the Go `http` package
type HttpTransport struct {
	// Endpoint of JSON-RPC service to consume
	Url string

	// Optional hook to invoke before/after requests
	Hook HttpHook

	// Optional custom HTTP client to be used instead of the default empty one.
	Client *http.Client

	// Optional CookieJar - useful if endpoint uses session cookies
	// Deprecated by custom Client option. If you need to provide CookieJar, provide a &http.Client{Jar: YourCookie}
	Jar http.CookieJar
}

// HttpHook is an optional callback interface that can be implemented
// if custom headers need to be added to the request, or if other
// operations are desired (e.g. request timing)
type HttpHook interface {
	// Called before the HTTP request is made.
	// Request may be altered, typically to add headers
	Before(req *http.Request, body []byte)

	// Called after the request is made, but before the
	// response is deserialized
	After(req *http.Request, resp *http.Response, body []byte)
}

func (t *HttpTransport) Send(in []byte) ([]byte, error) {

	req, err := http.NewRequest("POST", t.Url, bytes.NewBuffer(in))
	if err != nil {
		return nil, fmt.Errorf("barrister: HttpTransport NewRequest failed: %s", err)
	}

	// TODO: need to make mime type plugable
	req.Header.Add("Content-Type", "application/json")

	if t.Hook != nil {
		t.Hook.Before(req, in)
	}

	client := t.Client
	if client == nil {
		client = &http.Client{}
	}

	if t.Jar != nil {
		client.Jar = t.Jar
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("barrister: HttpTransport POST to %s failed: %s", t.Url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("barrister: HttpTransport POST to %s returned non-2xx status: %d - %s", t.Url, resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("barrister: HttpTransport Unable to read resp.Body: %s", err)
	}

	if t.Hook != nil {
		t.Hook.After(req, resp, body)
	}

	return body, nil
}

// Client abstracts methods for calling JSON-RPC services.  Note that the
// Server type below implements this interface, which allows services to be
// consumed in process without a transport or serializer.
type Client interface {
	// Call represents a single JSON-RPC method invocation
	Call(method string, params ...interface{}) (interface{}, error)

	// CallBatch represents a JSON-RPC batch request
	CallBatch(batch []JsonRpcRequest) []JsonRpcResponse
}

// NewRemoteClient creates a RemoteClient with the given Transport using the JsonSerializer
func NewRemoteClient(trans Transport, forceASCII bool) Client {
	return &RemoteClient{trans, &JsonSerializer{forceASCII}}
}

// RemoteClient implements Client against the given Transport and Serializer.
type RemoteClient struct {
	Trans Transport
	Ser   Serializer
}

func (c *RemoteClient) CallBatch(batch []JsonRpcRequest) []JsonRpcResponse {
	reqBytes, err := c.Ser.Marshal(batch)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch unable to Marshal request: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32600, Message: msg}}}
	}

	respBytes, err := c.Trans.Send(reqBytes)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch Transport error during request: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32603, Message: msg}}}
	}

	var batchResp []JsonRpcResponse
	err = c.Ser.Unmarshal(respBytes, &batchResp)
	if err != nil {
		msg := fmt.Sprintf("barrister: CallBatch unable to Unmarshal response: %s", err)
		return []JsonRpcResponse{
			JsonRpcResponse{Error: &JsonRpcError{Code: -32603, Message: msg}}}
	}

	return batchResp
}

func (c *RemoteClient) Call(method string, params ...interface{}) (interface{}, error) {
	rpcReq := JsonRpcRequest{Jsonrpc: "2.0", Id: randHex(20), Method: method, Params: params}

	reqBytes, err := c.Ser.Marshal(rpcReq)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Call unable to Marshal request: %s", method, err)
		return nil, &JsonRpcError{Code: -32600, Message: msg}
	}

	respBytes, err := c.Trans.Send(reqBytes)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Transport error during request: %s", method, err)
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	var rpcResp JsonRpcResponse
	err = c.Ser.Unmarshal(respBytes, &rpcResp)
	if err != nil {
		msg := fmt.Sprintf("barrister: %s: Call unable to Unmarshal response: %s", method, err)
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}

//////////////////////////////////////////////////
// Server //
////////////

// If a server handler implements Cloneable, it will
// be cloned per JSON-RPC call.  This allows you to initialize out
// of band context that your service implementation may need such
// as auth headers.
//
// In addition, by implementing Cloneable
// your services no longer need to be threadsafe, and can safely store
// state locally for the lifetime of the service method invocation.
//
type Cloneable interface {
	// CloneForReq is called after the JSON-RPC request is
	// decoded, but before the RPC method call is invoked.
	//
	// A copy of the implementing struct should be returned.
	// This copy will be used for a single RPC method call and
	// then discarded.
	CloneForReq(headers Headers) interface{}
}

// Represents transport request headers/cookies.
// Handler may mutate Response to send headers back to the caller.
type Headers struct {
	// Headers from the request. Handlers should not modify
	Request map[string][]string

	// Convenience property - only set if transport is HTTP
	// For other transports this may be nil
	// Read-only.
	Cookies []*http.Cookie

	// Writeable map of headers to return on the response
	// Transport implementations are responsible for
	// sending values in this map back to the caller.
	Response map[string][]string
}

// GetCookie returns the cookie associated with the given
// name, or nil if no cookie is found with that name.
func (me *Headers) GetCookie(name string) *http.Cookie {
	if me.Cookies != nil && len(me.Cookies) > 0 {
		for _, c := range me.Cookies {
			if c.Name == name {
				return c
			}
		}
	}
	return nil
}

// ReadCookies is taken from the net/http cookie.go standard library
// and populates Headers.Cookies from Headers.Request["Cookie"]
func (me *Headers) ReadCookies() {
	cookies := []*http.Cookie{}
	lines, ok := me.Request["Cookie"]
	if ok {
		for _, line := range lines {
			parts := strings.Split(strings.TrimSpace(line), ";")
			if len(parts) == 1 && parts[0] == "" {
				continue
			}
			// Per-line attributes
			parsedPairs := 0
			for i := 0; i < len(parts); i++ {
				parts[i] = strings.TrimSpace(parts[i])
				if len(parts[i]) == 0 {
					continue
				}
				name, val := parts[i], ""
				if j := strings.Index(name, "="); j >= 0 {
					name, val = name[:j], name[j+1:]
				}
				if !isCookieNameValid(name) {
					continue
				}
				val, success := parseCookieValue(val)
				if !success {
					continue
				}
				cookies = append(cookies, &http.Cookie{Name: name, Value: val})
				parsedPairs++
			}
		}
	}
	me.Cookies = cookies
}

// Filters allow you to intercept requests before and after the handler method
// is invoked.  Filters are useful for implementing cross cutting concerns
// such as authentication, performance measurement, logging, etc.
type Filter interface {

	// PreInvoke is called after the handler has been resolved, but prior
	// to handler method invocation.
	//
	// Return value of false terminates the filter chain, and r.result, r.err
	// will be used as the response.
	// Return value of true continues filter chain execution.
	//
	PreInvoke(r *RequestResponse) bool

	// PostInvoke is called after the handler method has been invoked and
	// returns a bool that indicates whether later filters should be called.
	//
	// Implementations may alter the ReturnVal, which will be later marshaled
	// into the JSON-RPC response.
	//
	// Return value of false terminates the filter chain, and r.result, r.err
	// will be used.  Return value of true continues filter chain execution.
	//
	PostInvoke(r *RequestResponse) bool
}

// NewJSONServer creates a Server for the given IDL that uses the JsonSerializer.
// If forceASCII is true, then unicode characters will be escaped
func NewJSONServer(idl *Idl, forceASCII bool) Server {
	return NewServer(idl, &JsonSerializer{forceASCII})
}

// NewServer creates a Server for the given IDL and Serializer
func NewServer(idl *Idl, ser Serializer) Server {
	return Server{idl, ser, map[string]interface{}{}, make([]Filter, 0)}
}

// Server represents a handler for Barrister IDL file.
// Each Server has one or more handlers (one per interface in the IDL) and
// zero or more Filters.
type Server struct {
	idl      *Idl
	ser      Serializer
	handlers map[string]interface{}
	filters  []Filter
}

// AddFilter registers a Filter implementation with the Server.
// Filter.PreInvoke is called in the order of registration.
// Filter.PostInvoke is called in reverse order of registration.
//
func (s *Server) AddFilter(f Filter) {
	s.filters = append(s.filters, f)
}

// AddHandler associates the given impl with the IDL interface.
// Typically this method is used with idl2go generated interfaces, so
// any validation issues indicate a programming bug.  Consequently this
// method panics instead of returning na error if any IDL mismatches are
// found.
func (s *Server) AddHandler(iface string, impl interface{}) {
	ifaceFuncs, ok := s.idl.interfaces[iface]

	if !ok {
		msg := fmt.Sprintf("barrister: IDL has no interface: %s", iface)
		panic(msg)
	}

	var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

	elem := reflect.ValueOf(impl)
	for _, idlFunc := range ifaceFuncs {
		fname := capitalize(idlFunc.Name)
		fn := elem.MethodByName(fname)
		if fn == zeroVal {
			msg := fmt.Sprintf("barrister: %s impl has no method named: %s",
				iface, fname)
			panic(msg)
		}

		fnType := fn.Type()
		if fnType.NumIn() != len(idlFunc.Params) {
			msg := fmt.Sprintf("barrister: %s impl method: %s accepts %d params but IDL specifies %d", iface, fname, fnType.NumIn(), len(idlFunc.Params))
			panic(msg)
		}

		if fnType.NumOut() != 2 {
			msg := fmt.Sprintf("barrister: %s impl method: %s returns %d params but must be 2", iface, fname, fnType.NumOut())
			panic(msg)
		}

		for x, param := range idlFunc.Params {
			path := fmt.Sprintf("%s.%s param[%d]", iface, fname, x)
			s.validate(param, fnType.In(x), path)
		}

		path := fmt.Sprintf("%s.%s return value[0]", iface, fname)
		s.validate(idlFunc.Returns, fnType.Out(0), path)

		errType := fnType.Out(1)
		if !errType.Implements(typeOfError) {
			msg := fmt.Sprintf("%s.%s return value[1] has invalid type: %s (expected: error)", iface, fname, errType)
			panic(msg)
		}
	}

	s.handlers[iface] = impl
}

// validate ensurse that the given implType matches the expected IDL type.
// If the type does not match, validate panics.
//
// This method is called by AddHandler to ensure that registered implementations
// comply with the IDL.
func (s *Server) validate(idlField Field, implType reflect.Type, path string) {
	testVal := idlField.testVal(s.idl)
	conv := newConvert(s.idl, &idlField, implType, testVal, "")
	_, err := conv.run()
	if err != nil {
		msg := fmt.Sprintf("barrister: %s has invalid type: %s reason: %s", path, implType, err)
		panic(msg)
	}
}

// InvokeBytes handles a raw request.  It unmarhals the request based on the
// registered Serializer (e.g. the JsonSerializer) and then determines if the
// request is a single or batch call.
//
// InvokeBytess delegates to InvokeOne and then marshals the result using the
// Serializer and returns the serialized byte slice.
func (s *Server) InvokeBytes(headers Headers, req []byte) []byte {

	// determine if batch or single
	batch := s.ser.IsBatch(req)

	// batch execution
	if batch {
		var batchReq []JsonRpcRequest
		batchResp := []JsonRpcResponse{}
		err := s.ser.Unmarshal(req, &batchReq)
		if err != nil {
			return jsonParseErr("", true, err)
		}

		for _, req := range batchReq {
			resp := s.InvokeOne(headers, &req)
			batchResp = append(batchResp, *resp)
		}

		b, err := s.ser.Marshal(batchResp)
		if err != nil {
			panic(err)
		}
		return b
	}

	// single request execution
	rpcReq := JsonRpcRequest{}
	err := s.ser.Unmarshal(req, &rpcReq)
	if err != nil {
		return jsonParseErr("", false, err)
	}

	resp := s.InvokeOne(headers, &rpcReq)

	b, err := s.ser.Marshal(resp)
	if err != nil {
		panic(err)
	}
	return b
}

// InvokeOne handles a single JSON-RPC request, delegating to Call.  If the special "barrister-idl"
// method is handled, InvokeOne will return the IDL associated with this Server.
func (s *Server) InvokeOne(headers Headers, rpcReq *JsonRpcRequest) *JsonRpcResponse {
	if rpcReq.Method == "barrister-idl" {
		// handle 'barrister-idl' method
		return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: s.idl.elems}
	}

	// handle normal RPC method executions
	var result interface{}
	var err error
	arr, ok := rpcReq.Params.([]interface{})
	if ok {
		result, err = s.Call(headers, rpcReq.Method, arr...)
	} else {
		result, err = s.Call(headers, rpcReq.Method)
	}

	if err == nil {
		// successful Call
		return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Result: result}
	}

	return &JsonRpcResponse{Jsonrpc: "2.0", Id: rpcReq.Id, Error: toJsonRpcError(rpcReq.Method, err)}
}

// CallBatch handles a JSON-RPC batch request.  All requests in the batch must target methods that this
// Server can handle (i.e. no additional message routing is performed).  Elements in the returned
// batch will match the order of the requests.
//
func (s *Server) CallBatch(headers Headers, batch []JsonRpcRequest) []JsonRpcResponse {
	batchResp := make([]JsonRpcResponse, len(batch))

	for _, req := range batch {
		result, err := s.Call(headers, req.Method, req.Params)
		resp := JsonRpcResponse{Jsonrpc: "2.0", Id: req.Id}
		if err == nil {
			resp.Result = result
		} else {
			resp.Error = toJsonRpcError(req.Method, err)
		}
		batchResp = append(batchResp, resp)
	}

	return batchResp
}

// Call handles a single JSON-RPC request.  The JSON-RPC method is parsed and the appropriate
// handler for the given interface is resolved.  The execution order is:
//
// 1) The method is checked against the IDL.  If the IDL does not define this method an error is returned.
//
// 2) The handler associated with this method is resolved. If no handler has been registered then an error
// is returned.
//
// 3) If the handler implements Cloneable, it will be cloned and passed the headers for this request.
//
// 4) If the Server has one or more Filters registered, PreInvoke() will be called on each Filter.  Filters are
// called in the order registered.  If any Filter returns false, the response returned by the Filter is returned.
//
// 5) Request parameters are validated against the IDL.  If the request violates the IDL an error is returned.
//
// 6) The handler function is invoked
//
// 7) If the Server has one or more Filters registered, PostInvoke() will be called on each Filter.  Filters are
// called in the reverse order.  If any Filter returns false, filter execution will stop.
//
// 8) The result/error is returned
//
func (s *Server) Call(headers Headers, method string, params ...interface{}) (interface{}, error) {

	idlFunc, ok := s.idl.methods[method]
	if !ok {
		return nil, &JsonRpcError{Code: -32601, Message: fmt.Sprintf("Unsupported method: %s", method)}
	}

	iface, fname := parseMethod(method)

	handler, ok := s.handlers[iface]
	if !ok {
		return nil, &JsonRpcError{Code: -32601,
			Message: fmt.Sprintf("No handler registered for interface: %s", iface)}
	}

	// If handler supports cloning, create a new instance for this request
	c, ok := handler.(Cloneable)
	if ok {
		handler = c.CloneForReq(headers)
	}

	elem := reflect.ValueOf(handler)
	fn := elem.MethodByName(fname)
	if fn == zeroVal {
		return nil, &JsonRpcError{Code: -32601,
			Message: fmt.Sprintf("Function %s not found on handler %s", fname, iface)}
	}

	//fmt.Printf("Call method: %s  params: %v\n", method, params)

	// check params
	fnType := fn.Type()
	if fnType.NumIn() != len(params) {
		return nil, &JsonRpcError{Code: -32602,
			Message: fmt.Sprintf("Method %s expects %d params but was passed %d", method, fnType.NumIn(), len(params))}
	}

	if len(idlFunc.Params) != len(params) {
		return nil, &JsonRpcError{Code: -32602,
			Message: fmt.Sprintf("Method %s expects %d params but was passed %d", method, len(idlFunc.Params), len(params))}
	}

	rr := &RequestResponse{headers, method, params, handler, nil, nil}

	// run filters - PreInvoke
	flen := len(s.filters)
	for i := 0; i < flen; i++ {
		ok := s.filters[i].PreInvoke(rr)
		if !ok {
			return rr.Result, rr.Err
		}
	}

	// convert params
	paramVals := []reflect.Value{}
	for x, param := range params {
		desiredType := fnType.In(x)
		idlField := idlFunc.Params[x]
		path := fmt.Sprintf("param[%d]", x)
		paramConv := newConvert(s.idl, &idlField, desiredType, param, path)
		converted, err := paramConv.run()
		if err != nil {
			return nil, &JsonRpcError{Code: -32602, Message: err.Error()}
		}
		paramVals = append(paramVals, converted)
	}

	// make the call
	ret := fn.Call(paramVals)
	if len(ret) != 2 {
		msg := fmt.Sprintf("Method %s did not return 2 values. len(ret)=%d", method, len(ret))
		return nil, &JsonRpcError{Code: -32603, Message: msg}
	}

	ret0 := ret[0].Interface()
	ret1 := ret[1].Interface()

	rr.Result = ret0
	if ret1 != nil {
		e, ok := ret1.(error)
		if ok {
			rr.Err = e
		}
	}

	// run filters - PostInvoke
	for i := flen - 1; i >= 0; i-- {
		ok := s.filters[i].PostInvoke(rr)
		if !ok {
			break
		}
	}

	return rr.Result, rr.Err
}

// ServeHTTP handles HTTP requests for the server.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		// TODO: log and return JSON-RPC response?
		panic(err)
	}

	headers := Headers{
		Request:  req.Header,
		Cookies:  req.Cookies(),
		Response: make(map[string][]string),
	}

	resp := s.InvokeBytes(headers, buf.Bytes())
	w.Header().Set("Content-Type", s.ser.MimeType())

	for k, v := range headers.Response {
		for _, s := range v {
			w.Header().Add(k, s)
		}
	}

	// TODO: log err?
	_, err = w.Write(resp)
}

// parseMethod takes a JSON-RPC method string and splits it on period, returning
// the part to the left of the period, and capitalizing the part to the right.
//
// For example, "UserService.save" would return: "UserService", "Save"
//
// If the method does not contain a period, the whole method and an empty string
// are returned.  For example, "doFoo" would return: "doFoo", ""
func parseMethod(method string) (string, string) {
	i := strings.Index(method, ".")
	if i > -1 && i < (len(method)-1) {
		iface := method[0:i]
		if i < (len(method) - 2) {
			return iface, strings.ToUpper(method[i+1:i+2]) + method[i+2:]
		} else {
			return iface, strings.ToUpper(method[i+1:])
		}
	}
	return method, ""
}

// jsonParseErr creates a JSON-RPC error and marhals it to a byte slice
// to be returned to the caller.
func jsonParseErr(reqId string, batch bool, err error) []byte {
	rpcerr := &JsonRpcError{Code: -32700, Message: fmt.Sprintf("Unable to parse JSON: %s", err.Error())}
	resp := JsonRpcResponse{Jsonrpc: "2.0"}
	resp.Id = reqId
	resp.Error = rpcerr

	if batch {
		respBatch := []JsonRpcResponse{resp}
		b, err := json.Marshal(respBatch)
		if err != nil {
			panic(err)
		}
		return b
	}

	b, err := json.Marshal(resp)
	if err != nil {
		panic(err)
	}
	return b
}
