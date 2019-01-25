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
	"io"
)

type scopedProvider struct {
	Provider

	prefix string
}

var _ Provider = (*scopedProvider)(nil)

func (s *scopedProvider) Get(key string) Value {
	return s.Provider.Get(fmt.Sprintf("%s.%s", s.prefix, key))
}

// NewScopedProvider wraps a provider and adds a prefix to all Get calls.
func NewScopedProvider(prefix string, provider Provider) Provider {
	if prefix == "" {
		return provider
	}
	return &scopedProvider{provider, prefix}
}

// NewProviderGroup composes multiple providers, with later providers
// overriding earlier ones. The merge logic is described in the package-level
// documentation. To preserve backward compatibility, the resulting provider
// disables strict unmarshalling.
//
// Prefer using NewYAML instead of this where possible. NewYAML gives you
// strict unmarshalling by default and allows use of other options at the same
// time.
func NewProviderGroup(name string, providers ...Provider) (Provider, error) {
	opts := make([]YAMLOption, 0, len(providers)+2)
	opts = append(opts, Name(name), Permissive())
	for _, p := range providers {
		if v := p.Get(Root); v.HasValue() {
			opts = append(opts, Static(v.Value()))
		}
	}
	return NewYAML(opts...)
}

// NewStaticProvider serializes a Go data structure to YAML, then loads it
// into a provider. To preserve backward compatibility, the resulting provider
// disables strict unmarshalling.
//
// Deprecated: use NewYAML and the Static option directly. This enables strict
// unmarshalling by default and allows use of other options at the same time.
func NewStaticProvider(data interface{}) (Provider, error) {
	return NewStaticProviderWithExpand(data, nil)
}

// NewStaticProviderWithExpand serializes a Go data structure to YAML, expands
// any environment variable references using the supplied lookup function,
// then loads the result into a provider. See the Expand option for a
// description of the environment variable replacement syntax. To preserve
// backward compatibility, the resulting provider disables strict
// unmarshalling.
//
// Deprecated: use NewYAML and the Static and Expand options directly. This
// enables strict unmarshalling by default and allows use of other options at
// the same time.
func NewStaticProviderWithExpand(data interface{}, lookup LookupFunc) (Provider, error) {
	return NewYAML(Static(data), Expand(lookup), Permissive())
}

// NewYAMLProviderFromBytes merges multiple YAML-formatted byte slices into a
// single provider. Later configuration blobs override earlier ones using the
// merge logic described in the package-level documentation. To preserve
// backward compatibility, the resulting provider disables strict
// unmarshalling.
//
// Deprecated: use NewYAML with the Source and Expand options directly. This
// enables strict unmarshalling by default and allows use of other options at
// the same time.
func NewYAMLProviderFromBytes(yamls ...[]byte) (Provider, error) {
	readers := make([]io.Reader, len(yamls))
	for i, y := range yamls {
		readers[i] = bytes.NewReader(y)
	}
	return NewYAMLProviderFromReader(readers...)
}

// NewYAMLProviderFromFiles opens and merges multiple YAML files into a single
// provider. Later files override earlier files using the merge logic
// described in the package-level documentation. To preserve backward
// compatibility, the resulting provider disables strict unmarshalling.
//
// Deprecated: use NewYAML and the File option directly. This enables strict
// unmarshalling by default and allows use of other options at the same time.
func NewYAMLProviderFromFiles(filenames ...string) (Provider, error) {
	return NewYAMLProviderWithExpand(nil, filenames...)
}

// NewYAMLProviderFromReader merges multiple YAML-formatted io.Readers into a
// single provider. Later readers override earlier ones using the merge logic
// described in the package-level documentation. To preserve backward
// compatibility, the resulting provider disables strict unmarshalling.
//
// Deprecated: use NewYAML and the Source option directly. This enables strict
// unmarshalling by default and allows use of other options at the same time.
func NewYAMLProviderFromReader(readers ...io.Reader) (Provider, error) {
	return NewYAMLProviderFromReaderWithExpand(nil, readers...)
}

// NewYAMLProviderFromReaderWithExpand merges multiple YAML-formatted
// io.Readers, expands any environment variable references using the supplied
// lookup function, and then loads the result into a provider. Later readers
// override earlier readers using the merge logic described in the
// package-level documentation. See the Expand option for a description of the
// environment variable replacement syntax. To preserve backward
// compatibility, the resulting provider disables strict unmarshalling.
//
// Deprecated: use NewYAML and the Source and Expand options directly. This
// enables strict unmarshalling by default and allows use of other options at
// the same time.
func NewYAMLProviderFromReaderWithExpand(lookup LookupFunc, readers ...io.Reader) (Provider, error) {
	opts := make([]YAMLOption, 0, len(readers)+2)
	opts = append(opts, Permissive(), Expand(lookup))
	for _, r := range readers {
		opts = append(opts, Source(r))
	}
	return NewYAML(opts...)
}

// NewYAMLProviderWithExpand opens and merges multiple YAML-formatted
// files, expands any environment variable references using the supplied
// lookup function, and then loads the result into a provider. Later readers
// override earlier readers using the merge logic described in the
// package-level documentation. See the Expand option for a description of the
// environment variable replacement syntax. To preserve backward
// compatibility, the resulting provider disables strict unmarshalling.
//
// Deprecated: use NewYAML and the File and Expand options directly. This
// enables strict unmarshalling by default and allows use of other options at
// the same time.
func NewYAMLProviderWithExpand(lookup LookupFunc, filenames ...string) (Provider, error) {
	opts := make([]YAMLOption, 0, len(filenames)+2)
	opts = append(opts, Permissive(), Expand(lookup))
	for _, name := range filenames {
		opts = append(opts, File(name))
	}
	return NewYAML(opts...)
}
