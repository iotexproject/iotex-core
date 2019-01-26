# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## 1.3.1 - 2018-10-22
### Fixed
- Fix environment variable interpolation when `WithDefault` is used.
- Fix support for non-scalar keys in YAML mappings (eg: "1: car").

## 1.3.0 - 2018-07-30
### Added
- Add a `RawSource` option that selectively disables variable expansion.

## 1.2.2 - 2018-07-12
### Changed
- Undo deprecation of `NewProviderGroup`.

### Fixed
- Handle empty sources correctly in `NewProviderGroup`.

## 1.2.1 - 2018-07-06
### Fixed
- Handle empty sources and top-level nulls correctly.

## v1.2.0 - 2018-06-28
### Added
- Add `NewYAML`, a new `Provider` constructor that lets callers mix Go values,
  readers, files, and other options.
- Add support for `gopkg.in/yaml.v2`'s strict mode. The behavior of the existing
  constructors is unchanged, but `NewYAML` enables strict mode by default.

### Fixed
- Fix behavior of explicit `nil`s during merges. `nil` now correctly replaces
  any value it's merged into.
- Fix panic when a default is set for a value explicitly set to `nil`.
- Eliminate situations where configuration was mistakenly shallow-copied,
  making any mutations visible to other callers.
- Correctly handle `omitempty`, `inline`, and `flow` fields.
- Make `NopProvider.HasValue` always return false.
- Stop expanding environment variables in YAML comments.
- Make `NewValue` panic when the user-supplied parameters don't match the
  contents of the `Provider`. See the package documentation for details.
- Remove undocumented support for the `default` struct tag, which was supposed
  to have been removed prior to the 1.0 release. All known users of the tag were
  migrated.

### Deprecated
- Deprecate all existing `Provider` constructors except `NewScopedProvider` in
  favor of `NewYAML`.
- Deprecate `NewValue` in favor of `Provider.Get`.
- Deprecate `Value.HasValue`, `Value.Value`, and `Value.WithDefault` in favor of
  strongly-typed approaches using `Value.Populate`.

## v1.1.0 - 2017-09-28
### Added
- Make expand functions transform a special sequence $$ to literal $.
- Export `Provider` constructors that take `io.Reader`.

### Fixed
- Determine the types of objects encapsulated by `config.Value` with the YAML
  unmarshaller regardless of whether expansion was performed or not.

## v1.0.2 - 2017-08-17
### Fixed
- Fix populate panic for a nil pointer.

## v1.0.1 - 2017-08-04
### Fixed
- Fix unmarshal text on missing value.

## v1.0.0 - 2017-07-31
### Changed
- Skip populating function and value types instead of reporting errors.
- Return an error from provider constructors instead of panicking.
- Return an error from `Value.WithDefault` if the default is unusable.

### Removed
- Remove timestamps on `Value`.
- Remove `Try` and `As` conversion helpers.
- Remove `Value.IsDefault` method.
- Remove `Load` family of functions.
- Unexport `NewYAMLProviderFromReader` family of functions.

### Fixed
- Use semantic version paths for yaml and validator packages.

## v1.0.0-rc1 - 2017-06-26
### Removed
- Trim `Provider` interface down to just `Name` and `Get`.
