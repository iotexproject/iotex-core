package barrister

import (
	"fmt"
	"reflect"
	"strings"
)

type typeError struct {
	// string that describes location of value in the
	// param or return value graph.  e.g. param[0].Addresses[0].Street1
	path string

	msg string
}

func (e *typeError) Error() string {
	return fmt.Sprintf("barrister: %s: %s", e.path, e.msg)
}

func Convert(idl *Idl, field *Field, desired reflect.Type, actual interface{}, path string) (interface{}, error) {
	c := newConvert(idl, field, desired, actual, path)
	conv, err := c.run()
	if err != nil {
		return nil, err
	}

	return conv.Interface(), nil
}

type convert struct {
	idl       *Idl
	field     *Field
	desired   reflect.Type
	desirePtr bool
	actual    interface{}
	converted reflect.Value
	path      string
}

func newConvert(idl *Idl, field *Field, desired reflect.Type, actual interface{}, path string) *convert {
	return &convert{idl, field, desired, false, actual, zeroVal, path}
}

func (c *convert) run() (reflect.Value, error) {
	desiredKind := c.desired.Kind()

	actType := reflect.TypeOf(c.actual)

	if actType == c.desired {
		// return value without checking IDL
		return reflect.ValueOf(c.actual), nil
	}

	if actType == nil {
		if c.field.Optional && (desiredKind == reflect.Ptr || desiredKind == reflect.Slice) {
			return reflect.ValueOf(c.actual), nil
		} else if c.field.Optional {
			return reflect.Zero(c.desired), nil
		} else {
			return zeroVal, &typeError{c.path, fmt.Sprintf("%v null not allowed", c.field)}
		}
	}

	if desiredKind == reflect.Ptr {
		c.desirePtr = true
		c.desired = c.desired.Elem()
		desiredKind = c.desired.Kind()
	}

	c.converted = reflect.New(c.desired)

	actVal := reflect.ValueOf(c.actual)

	//fmt.Printf("convert: idl: %s go: %s actual: %s\n", c.field.Type, desiredKind, actType)

	if actType.Kind() == desiredKind {
		switch desiredKind {
		case reflect.String:
			if c.field.Type == "string" {
				c.converted.Elem().Set(actVal)
				return c.returnVal("string")
			} else {
				enum, ok := c.idl.enums[c.field.Type]
				if ok {
					s := actVal.String()
					for _, enumVal := range enum {
						if enumVal.Value == s {
							c.converted.Elem().SetString(s)
							return c.convertedVal()
						}
					}

					msg := fmt.Sprintf("Value '%s' not in enum values: ", s)
					for x, enumVal := range enum {
						if x > 0 {
							msg += ", "
						}
						msg += "'" + enumVal.Value + "'"
					}
					return zeroVal, &typeError{path: c.path, msg: msg}
				}
			}
		}
	} else {
		//fmt.Printf("%v is NOT assignable to %v\n", actType, c.desired)
	}

	switch desiredKind {
	case reflect.String:
		_, ok := c.actual.(string)
		if ok {
			return c.returnVal("string")
		}
	case reflect.Int:
		s, ok := c.actual.(int)
		if ok {
			c.converted.Elem().SetInt(int64(s))
			return c.returnVal("int")
		}
		s2, ok := c.actual.(int64)
		if ok {
			c.converted.Elem().SetInt(s2)
			return c.returnVal("int")
		}
	case reflect.Int64:
		s, ok := c.actual.(int64)
		if ok {
			c.converted.Elem().SetInt(s)
			return c.returnVal("int")
		}
		s2, ok := c.actual.(int)
		if ok {
			c.converted.Elem().SetInt(int64(s2))
			return c.returnVal("int")
		}
		s3, ok := c.actual.(float64)
		if ok {
			s4 := int64(s3)
			if float64(s4) == s3 {
				c.converted.Elem().SetInt(s4)
				return c.returnVal("int")
			}
		}
	case reflect.Float32:
		s, ok := c.actual.(float32)
		if ok {
			c.converted.Elem().SetFloat(float64(s))
			return c.returnVal("float")
		}
	case reflect.Float64:
		s, ok := c.actual.(float64)
		if ok {
			c.converted.Elem().SetFloat(s)
			return c.returnVal("float")
		}
		s3, ok := c.actual.(float32)
		if ok {
			c.converted.Elem().SetFloat(float64(s3))
			return c.returnVal("float")
		}
		s4, ok := c.actual.(int)
		if ok {
			c.converted.Elem().SetFloat(float64(s4))
			return c.returnVal("float")
		}
		s5, ok := c.actual.(int64)
		if ok {
			c.converted.Elem().SetFloat(float64(s5))
			return c.returnVal("float")
		}
		s6, ok := c.actual.(int32)
		if ok {
			c.converted.Elem().SetFloat(float64(s6))
			return c.returnVal("float")
		}
	case reflect.Bool:
		b, ok := c.actual.(bool)
		if ok {
			c.converted.Elem().SetBool(b)
			return c.returnVal("bool")
		}
	case reflect.Slice:
		actType := actVal.Type()
		if actType.Kind() == reflect.Slice {
			return c.convertSlice(actVal)
		}
	case reflect.Struct:
		m, ok := c.actual.(map[string]interface{})
		if ok {
			return c.convertStruct(m)
		}
	}

	msg := fmt.Sprintf("Unable to convert: %v - %v to %v", c.path,
		actType.Kind().String(), c.desired)
	return zeroVal, &typeError{c.path, msg}
}

func (c *convert) convertSlice(actVal reflect.Value) (reflect.Value, error) {
	length := actVal.Len()
	slice := reflect.MakeSlice(c.desired, length, length)

	elemField := &Field{Name: c.field.Name, Type: c.field.Type,
		Optional: c.field.Optional, IsArray: false}

	sliceType := c.desired.Elem()

	elemConv := newConvert(c.idl, elemField, sliceType, nil, "")

	for x := 0; x < length; x++ {

		el := actVal.Index(x)
		elemConv.actual = el.Interface()

		elemConv.path = c.path + "[" + string(x) + "]"

		conv, err := elemConv.run()
		if err != nil {
			return zeroVal, err
		}

		slice.Index(x).Set(conv)
	}

	c.converted = slice
	return c.convertedVal()
}

func (c *convert) convertStruct(m map[string]interface{}) (reflect.Value, error) {

	idlStruct, ok := c.idl.structs[c.field.Type]

	if !ok {
		msg := fmt.Sprintf("Struct not found in IDL: %s", c.field.Type)
		return zeroVal, &typeError{path: c.path, msg: msg}
	}

	val := reflect.New(c.desired)

	for _, sField := range idlStruct.allFields {
		fname := sField.Name
		goName := fname
		structField, ok := c.desired.FieldByName(fname)
		if !ok {
			goName = capitalize(fname)
			structField, ok = c.desired.FieldByName(goName)
			if !ok {
				msg := fmt.Sprintf("Struct: %v is missing required field: %s",
					c.desired, goName)
				return zeroVal, &typeError{path: c.path, msg: msg}
			}
		}

		mval, ok := m[fname]

		if !ok && !sField.Optional {
			msg := fmt.Sprintf("Input value: %v is missing required field: %s",
				m, fname)
			return zeroVal, &typeError{path: c.path, msg: msg}
		}

		if ok {

			fieldConv := newConvert(c.idl, &sField, structField.Type, mval,
				c.path+"."+fname)
			conv, err := fieldConv.run()
			if err != nil {
				return zeroVal, err
			}

			f := val.Elem().FieldByName(goName)
			if f == zeroVal {
				msg := fmt.Sprintf("Instance: %s is missing required field: %s",
					c.field.Type, goName)
				return zeroVal, &typeError{path: c.path, msg: msg}
			}

			if f.Kind() == reflect.Ptr {
				if conv.Kind() == reflect.Ptr {
					f.Set(conv)
				} else if conv.CanAddr() {
					f.Set(conv.Addr())
				}
			} else {
				if conv.Kind() == reflect.Ptr {
					f.Set(conv.Elem())
				} else {
					f.Set(conv)
				}
			}
		}
	}

	c.converted = val
	return c.convertedVal()
}

func (c *convert) returnVal(convertedType string) (reflect.Value, error) {
	if c.field.Type != convertedType {
		msg := fmt.Sprintf("Type mismatch for '%s' - Expected: %s Got: %v",
			c.path, c.field.Type, convertedType)
		return zeroVal, &typeError{path: c.path, msg: msg}
	}

	return c.convertedVal()
}

func (c *convert) convertedVal() (reflect.Value, error) {
	if c.desirePtr || c.converted.Kind() != reflect.Ptr {
		return c.converted, nil
	}
	return c.converted.Elem(), nil
}

func capitalize(s string) string {
	switch len(s) {
	case 0:
		return s
	case 1:
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func capitalizeAndStripMatchingPkg(s string, pkgToStrip string) string {
	ns, name := splitNs(s)
	if ns == "" {
		return capitalize(s)
	} else if ns == pkgToStrip {
		return capitalize(name)
	}

	return ns + "." + capitalize(name)
}
