// Copyright (c) 2018 Uber Technologies, Inc.
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

	yaml "gopkg.in/yaml.v2"
)

// areSameYAML checks whether two values represent the same YAML data. It's
// only called from NewValue, where we must validate that the user-supplied
// value matches the contents of the user-supplied provider.
func areSameYAML(fromProvider, fromUser interface{}) (bool, error) {
	p, err := yaml.Marshal(fromProvider)
	if err != nil {
		// Unreachable with YAML provider, but possible if the provider is a
		// third-party implementation.
		return false, fmt.Errorf("can't represent %#v as YAML: %v", fromProvider, err)
	}
	u, err := yaml.Marshal(fromUser)
	if err != nil {
		return false, fmt.Errorf("can't represent %#v as YAML: %v", fromUser, err)
	}
	return bytes.Equal(p, u), nil
}
