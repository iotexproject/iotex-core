// Copyright (c) 2017-2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"golang.org/x/text/transform"
)

const (
	_envSeparator = ":"
	_emptyDefault = `""`
)

// A LookupFunc behaves like os.LookupEnv: it uses the supplied string as a
// key into some key-value store and returns the value and whether the key was
// present.
type LookupFunc = func(string) (string, bool)

func expandVariables(f LookupFunc, buf *bytes.Buffer) (*bytes.Buffer, error) {
	if f == nil {
		return buf, nil
	}
	exp, err := ioutil.ReadAll(transform.NewReader(buf, newExpandTransformer(f)))
	if err != nil {
		return nil, fmt.Errorf("couldn't expand environment: %v", err)
	}
	return bytes.NewBuffer(exp), nil
}

// Given a function with the same signature as os.LookupEnv, return a function
// that expands expressions of the form ${ENV_VAR:default_value}.
func replace(lookUp LookupFunc) func(in string) (string, error) {
	return func(in string) (string, error) {
		sep := strings.Index(in, _envSeparator)
		var key string
		var def string

		if sep == -1 {
			// separator missing - everything is the key ${KEY}
			key = in
		} else {
			// ${KEY:DEFAULT}
			key = in[:sep]
			def = in[sep+1:]
		}

		if envVal, ok := lookUp(key); ok {
			return envVal, nil
		}

		if def == "" {
			return "", fmt.Errorf(`default is empty for %q (use "" for empty string)`, key)
		} else if def == _emptyDefault {
			return "", nil
		}

		return def, nil
	}
}

// expandTransformer implements transform.Transformer
type expandTransformer struct {
	transform.NopResetter

	expand func(string) (string, error)
}

func newExpandTransformer(lookup LookupFunc) *expandTransformer {
	return &expandTransformer{expand: replace(lookup)}
}

// First char of shell variable may be [a-zA-Z_]
func isShellNameFirstChar(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z')
}

// Char's after the first of shell variable may be [a-zA-Z0-9_]
func isShellNameChar(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// bytesIndexCFunc returns the index of the byte for which the
// complement of the supplied function is true
func bytesIndexCFunc(buf []byte, f func(b byte) bool) int {
	for i, b := range buf {
		if !f(b) {
			return i
		}
	}
	return -1
}

// Transform expands shell-like sequences like $foo and ${foo} using
// the configured expand function.  The sequence '$$' is replaced with
// a literal '$'.
func (e *expandTransformer) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var srcPos int
	var dstPos int

	for srcPos < len(src) {

		if dstPos == len(dst) {
			return dstPos, srcPos, transform.ErrShortDst
		}

		end := bytes.IndexByte(src[srcPos:], '$')

		if end == -1 {
			// src does not contain '$', copy into dst
			cnt := copy(dst[dstPos:], src[srcPos:])
			srcPos += cnt
			dstPos += cnt
			continue
		} else if end > 0 {
			// copy chars preceding '$' from src to dst
			cnt := copy(dst[dstPos:], src[srcPos:srcPos+end])
			srcPos += cnt
			dstPos += cnt

			if dstPos == len(dst) {
				return dstPos, srcPos, transform.ErrShortDst
			}
		}

		// src[srcPos] now points to '$', dstPos < len(dst)

		// If we're at the end of src, but we found a starting
		// token, return ErrShortSrc, unless we're also at EOF,
		// in which case just copy it dst.
		if srcPos+1 == len(src) {
			if atEOF {
				dst[dstPos] = src[srcPos]
				srcPos++
				dstPos++
				continue
			}
			return dstPos, srcPos, transform.ErrShortSrc
		}

		// At this point we know that src[srcPos+1] is populated.

		// If this token sequence represents the special '$$'
		// sequence, emit a '$' into dst.
		if src[srcPos+1] == '$' {
			dst[dstPos] = src[srcPos]
			srcPos += 2
			dstPos++
			continue
		}

		var token []byte
		var tokenEnd int

		// Start of bracketed token ${foo}
		if src[srcPos+1] == '{' {
			end := bytes.IndexByte(src[srcPos+2:], '}')
			if end == -1 {
				if atEOF {
					// No closing bracket and we're at
					// EOF, so it's not a valid bracket
					// expression.
					if len(dst[dstPos:]) <
						len(src[srcPos:]) {
						return dstPos, srcPos,
							transform.ErrShortDst
					}

					cnt := copy(dst[dstPos:], src[srcPos:])
					srcPos += cnt
					dstPos += cnt
					continue
				}

				// Otherwise, we need more bytes in src
				return dstPos, srcPos, transform.ErrShortSrc
			}

			// Set tokenEnd so it points to the byte
			// immediately after the closing '}'
			tokenEnd = end + srcPos + 3

			token = src[srcPos+2 : tokenEnd-1]
		} else { // Else start of non-bracketed token $foo
			if !isShellNameFirstChar(src[srcPos+1]) {
				// If it doesn't conform to the naming
				// rules for shell variables, do not
				// try to expand, just copy to dst.
				dst[dstPos] = src[srcPos]
				srcPos++
				dstPos++
				continue
			}

			end := bytesIndexCFunc(src[srcPos+2:], isShellNameChar)

			if end == -1 {
				// Reached the end of src without finding
				// end of shell variable
				if !atEOF {
					// We need more bytes in src
					return dstPos, srcPos,
						transform.ErrShortSrc
				}
				tokenEnd = len(src)
			} else {
				// Set tokenEnd so it points to the byte
				// immediately after the token
				tokenEnd = end + srcPos + 2
			}

			token = src[srcPos+1 : tokenEnd]
		}

		replacement, err := e.expand(string(token))
		if err != nil {
			return dstPos, srcPos, err
		}

		if len(dst[dstPos:]) < len(replacement) {
			return dstPos, srcPos, transform.ErrShortDst
		}

		cnt := copy(dst[dstPos:], replacement)
		srcPos = tokenEnd
		dstPos += cnt
	}

	return dstPos, srcPos, nil
}
