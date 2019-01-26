// Copyright (c) 2017 Uber Technologies, Inc.
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

// Package config is an encoding-agnostic configuration abstraction. It
// supports merging multiple configuration files, expanding environment
// variables, and a variety of other small niceties. It currently supports
// YAML, but may be extended in the future to support more restrictive
// encodings like JSON or TOML.
//
// Merging Configuration
//
// It's often convenient to separate configuration into multiple files; for
// example, an application may want to first load some universally-applicable
// configuration and then merge in some environment-specific overrides. This
// package supports this pattern in a variety of ways, all of which use the
// same merge logic.
//
// Simple types (numbers, strings, dates, and anything else YAML would
// consider a scalar) are merged by replacing lower-priority values with
// higher-priority overrides. For example, consider this merge of base.yaml
// and override.yaml:
//   # base.yaml
//   some_key: foo
//
//   # override.yaml
//   some_key: bar
//
//   # merged result
//   some_key: bar
//
// Slices, arrays, and anything else YAML would consider a sequence are also
// replaced. Again merging base.yaml and override.yaml:
//   # base.yaml
//   some_key: [foo, bar]
//
//   # override.yaml
//   some_key: [baz, quux]
//
//   # merged output
//   some_key: [baz, quux]
//
// Maps are recursively deep-merged, handling scalars and sequences as
// described above. Consider a merge between a more complex set of YAML files:
//   # base.yaml
//   some_key:
//     foo: bar
//     foos: [1, 2]
//
//   # override.yaml
//   some_key:
//     baz: quux
//     foos: [3, 4]
//
//  # merged output
//	some_key:
//	  foo: bar      # from base.yaml
//	  baz: quux     # from override.yaml
//	  foos: [3, 4]  # from override.yaml
//
// In all cases, explicit nils (represented in YAML with a tilde) override any
// pre-existing configuration. For example,
//   # base.yaml
//   foo: {bar: baz}
//
//   # override.yaml
//   foo: ~
//
//   # merged output
//   foo: ~
//
// Strict Unmarshalling
//
// By default, the NewYAML constructor enables gopkg.in/yaml.v2's strict
// unmarshalling mode. This prevents a variety of common programmer errors,
// especially when deep-merging loosely-typed YAML files. In strict mode,
// providers throw errors if keys are duplicated in the same configuration
// source, all keys aren't used when populating a struct, or a merge
// encounters incompatible data types. This behavior can be disabled with the
// Permissive option.
//
// To maintain backward compatibility, all other constructors default to
// permissive unmarshalling.
//
// Quote Strings
//
// YAML allows strings to appear quoted or unquoted, so these two lines are
// identical:
//   foo: bar
//   "foo": "bar"
//
// However, the YAML specification special-cases some unquoted strings. Most
// obviously, true and false are interpreted as Booleans (unless quoted). Less
// obviously, yes, no, on, off, and many variants of these words are also
// treated as Booleans (see http://yaml.org/type/bool.html for the complete
// specification).
//
// Correctly deep-merging sources requires this package to unmarshal and then
// remarshal all YAML, which implicitly converts these special-cased unquoted
// strings to their canonical representation. For example,
//   foo: yes  # before merge
//   foo: true # after merge
//
// Quoting special-cased strings prevents this surprising behavior.
//
// Deprecated APIs
//
// Unfortunately, this package was released with a variety of bugs and an
// overly large API. The internals of the configuration provider have been
// completely reworked and all known bugs have been addressed, but many
// duplicative exported functions were retained to preserve backward
// compatibility. New users should rely on the NewYAML constructor. In
// particular, avoid NewValue - it's unnecessary, complex, and may panic.
//
// Deprecated functions are documented in the format expected by the
// staticcheck linter, available at https://staticcheck.io/.
package config
