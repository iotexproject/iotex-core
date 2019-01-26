package config

import "bytes"

var (
	_dollar       = []byte("$")
	_doubleDollar = []byte("$$")
)

// To avoid a variety of other bugs (e.g,
// https://github.com/uber-go/config/issues/80), we defer environment variable
// expansion until we've already merged all sources. However, merging blends
// data from sources that should have variables expanded and those that
// shouldn't (e.g., secrets). To protect variable-like strings in sources that
// shouldn't be expanded, we must escape them.
func escapeVariables(bs []byte) []byte {
	return bytes.Replace(bs, _dollar, _doubleDollar, -1)
}
