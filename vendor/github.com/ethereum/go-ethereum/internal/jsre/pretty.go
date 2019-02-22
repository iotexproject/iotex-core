// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package jsre

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/robertkrimen/otto"
)

const (
	maxPrettyPrintLevel = 3
	indentString        = "  "
)

var (
	FunctionColor = color.New(color.FgMagenta).SprintfFunc()
	SpecialColor  = color.New(color.Bold).SprintfFunc()
	NumberColor   = color.New(color.FgRed).SprintfFunc()
	StringColor   = color.New(color.FgGreen).SprintfFunc()
	ErrorColor    = color.New(color.FgHiRed).SprintfFunc()
)

// these fields are hidden when printing objects.
var boringKeys = map[string]bool{
	"valueOf":              true,
	"toString":             true,
	"toLocaleString":       true,
	"hasOwnProperty":       true,
	"isPrototypeOf":        true,
	"propertyIsEnumerable": true,
	"constructor":          true,
}

// prettyPrint writes value to standard output.
func prettyPrint(vm *otto.Otto, value otto.Value, w io.Writer) {
	ppctx{vm: vm, w: w}.printValue(value, 0, false)
}

// prettyError writes err to standard output.
func prettyError(vm *otto.Otto, err error, w io.Writer) {
	failure := err.Error()
	if ottoErr, ok := err.(*otto.Error); ok {
		failure = ottoErr.String()
	}
	fmt.Fprint(w, ErrorColor("%s", failure))
}

func (re *JSRE) prettyPrintJS(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		prettyPrint(call.Otto, v, re.output)
		fmt.Fprintln(re.output)
	}
	return otto.UndefinedValue()
}

type ppctx struct {
	vm *otto.Otto
	w  io.Writer
}

func (ctx ppctx) indent(level int) string {
	return strings.Repeat(indentString, level)
}

func (ctx ppctx) printValue(v otto.Value, level int, inArray bool) {
	switch {
	case v.IsObject():
		ctx.printObject(v.Object(), level, inArray)
	case v.IsNull():
		fmt.Fprint(ctx.w, SpecialColor("null"))
	case v.IsUndefined():
		fmt.Fprint(ctx.w, SpecialColor("undefined"))
	case v.IsString():
		s, _ := v.ToString()
		fmt.Fprint(ctx.w, StringColor("%q", s))
	case v.IsBoolean():
		b, _ := v.ToBoolean()
		fmt.Fprint(ctx.w, SpecialColor("%t", b))
	case v.IsNaN():
		fmt.Fprint(ctx.w, NumberColor("NaN"))
	case v.IsNumber():
		s, _ := v.ToString()
		fmt.Fprint(ctx.w, NumberColor("%s", s))
	default:
		fmt.Fprint(ctx.w, "<unprintable>")
	}
}

func (ctx ppctx) printObject(obj *otto.Object, level int, inArray bool) {
	switch obj.Class() {
	case "Array", "GoArray":
		lv, _ := obj.Get("length")
		len, _ := lv.ToInteger()
		if len == 0 {
			fmt.Fprintf(ctx.w, "[]")
			return
		}
		if level > maxPrettyPrintLevel {
			fmt.Fprint(ctx.w, "[...]")
			return
		}
		fmt.Fprint(ctx.w, "[")
		for i := int64(0); i < len; i++ {
			el, err := obj.Get(strconv.FormatInt(i, 10))
			if err == nil {
				ctx.printValue(el, level+1, true)
			}
			if i < len-1 {
				fmt.Fprintf(ctx.w, ", ")
			}
		}
		fmt.Fprint(ctx.w, "]")

	case "Object":
		// Print values from bignumber.js as regular numbers.
		if ctx.isBigNumber(obj) {
			fmt.Fprint(ctx.w, NumberColor("%s", toString(obj)))
			return
		}
		// Otherwise, print all fields indented, but stop if we're too deep.
		keys := ctx.fields(obj)
		if len(keys) == 0 {
			fmt.Fprint(ctx.w, "{}")
			return
		}
		if level > maxPrettyPrintLevel {
			fmt.Fprint(ctx.w, "{...}")
			return
		}
		fmt.Fprintln(ctx.w, "{")
		for i, k := range keys {
			v, _ := obj.Get(k)
			fmt.Fprintf(ctx.w, "%s%s: ", ctx.indent(level+1), k)
			ctx.printValue(v, level+1, false)
			if i < len(keys)-1 {
				fmt.Fprintf(ctx.w, ",")
			}
			fmt.Fprintln(ctx.w)
		}
		if inArray {
			level--
		}
		fmt.Fprintf(ctx.w, "%s}", ctx.indent(level))

	case "Function":
		// Use toString() to display the argument list if possible.
		if robj, err := obj.Call("toString"); err != nil {
			fmt.Fprint(ctx.w, FunctionColor("function()"))
		} else {
			desc := strings.Trim(strings.Split(robj.String(), "{")[0], " \t\n")
			desc = strings.Replace(desc, " (", "(", 1)
			fmt.Fprint(ctx.w, FunctionColor("%s", desc))
		}

	case "RegExp":
		fmt.Fprint(ctx.w, StringColor("%s", toString(obj)))

	default:
		if v, _ := obj.Get("toString"); v.IsFunction() && level <= maxPrettyPrintLevel {
			s, _ := obj.Call("toString")
			fmt.Fprintf(ctx.w, "<%s %s>", obj.Class(), s.String())
		} else {
			fmt.Fprintf(ctx.w, "<%s>", obj.Class())
		}
	}
}

func (ctx ppctx) fields(obj *otto.Object) []string {
	var (
		vals, methods []string
		seen          = make(map[string]bool)
	)
	add := func(k string) {
		if seen[k] || boringKeys[k] || strings.HasPrefix(k, "_") {
			return
		}
		seen[k] = true
		if v, _ := obj.Get(k); v.IsFunction() {
			methods = append(methods, k)
		} else {
			vals = append(vals, k)
		}
	}
	iterOwnAndConstructorKeys(ctx.vm, obj, add)
	sort.Strings(vals)
	sort.Strings(methods)
	return append(vals, methods...)
}

func iterOwnAndConstructorKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	seen := make(map[string]bool)
	iterOwnKeys(vm, obj, func(prop string) {
		seen[prop] = true
		f(prop)
	})
	if cp := constructorPrototype(obj); cp != nil {
		iterOwnKeys(vm, cp, func(prop string) {
			if !seen[prop] {
				f(prop)
			}
		})
	}
}

func iterOwnKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	Object, _ := vm.Object("Object")
	rv, _ := Object.Call("getOwnPropertyNames", obj.Value())
	gv, _ := rv.Export()
	switch gv := gv.(type) {
	case []interface{}:
		for _, v := range gv {
			f(v.(string))
		}
	case []string:
		for _, v := range gv {
			f(v)
		}
	default:
		panic(fmt.Errorf("Object.getOwnPropertyNames returned unexpected type %T", gv))
	}
}

func (ctx ppctx) isBigNumber(v *otto.Object) bool {
	// Handle numbers with custom constructor.
	if v, _ := v.Get("constructor"); v.Object() != nil {
		if strings.HasPrefix(toString(v.Object()), "function BigNumber") {
			return true
		}
	}
	// Handle default constructor.
	BigNumber, _ := ctx.vm.Object("BigNumber.prototype")
	if BigNumber == nil {
		return false
	}
	bv, _ := BigNumber.Call("isPrototypeOf", v)
	b, _ := bv.ToBoolean()
	return b
}

func toString(obj *otto.Object) string {
	s, _ := obj.Call("toString")
	return s.String()
}

func constructorPrototype(obj *otto.Object) *otto.Object {
	if v, _ := obj.Get("constructor"); v.Object() != nil {
		if v, _ = v.Object().Get("prototype"); v.Object() != nil {
			return v.Object()
		}
	}
	return nil
}
