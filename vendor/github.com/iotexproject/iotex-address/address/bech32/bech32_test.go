// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bech32

import (
	"strings"
	"testing"
)

func TestBech32(t *testing.T) {
	tests := []struct {
		str   string
		valid bool
	}{
		// Try some test vectors from https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki#Bech32
		{"A12UEL5L", true},
		{"a12uel5l", true},
		{"an83characterlonghumanreadablepartthatcontainsthenumber1andtheexcludedcharactersbio1tt5tgs", true},
		{"abcdef1qpzry9x8gf2tvdw0s3jn54khce6mua7lmqqqxw", true},
		{"11qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqc8247j", true},
		{"split1checkupstagehandshakeupstreamerranterredcaperred2y9e3w", true},
		{"Split1checkupstagehandshakeupstreamerranterredcaperred2y9e3w", false},                   // mix of lower upper
		{"split1checkupstagehandshakeupstreamerranterredcaperred2y9e2w", false},                   // invalid checksum
		{"s lit1checkupstagehandshakeupstreamerranterredcaperredp8hs2p", false},                   // invalid character (space) in hrp
		{"spl" + string(127) + "t1checkupstagehandshakeupstreamerranterredcaperred2y9e3w", false}, // invalid character (DEL) in hrp
		{"split1cheo2y9e2w", false}, // invalid character (o) in data part
		{"split1a2y9w", false},      // too short data part
		{"1checkupstagehandshakeupstreamerranterredcaperred2y9e3w", false}, // empty hrp
		{"li1dgmt3", false}, // Too short checksum
		{"11qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqc8247j", false}, // overall max length exceeded
	}

	for _, test := range tests {
		str := test.str
		hrp, decoded, err := Decode(str)
		if !test.valid {
			// Invalid string decoding should result in error.
			if err == nil {
				t.Errorf("expected decoding to fail for invalid string %v", test.str)
			}
			continue
		}

		// Valid string decoding should result in no error.
		if err != nil {
			t.Errorf("expected string to be valid bech32: %v", err)
		}

		// Check that it encodes to the same string
		encoded, err := Encode(hrp, decoded)
		if err != nil {
			t.Errorf("encoding failed: %v", err)
		}

		if encoded != strings.ToLower(str) {
			t.Errorf("expected data to encode to %v, but got %v", str, encoded)
		}

		// Flip a bit in the string an make sure it is caught.
		pos := strings.LastIndexAny(str, "1")
		flipped := str[:pos+1] + string((str[pos+1] ^ 1)) + str[pos+2:]
		_, _, err = Decode(flipped)
		if err == nil {
			t.Error("expected decoding to fail")
		}
	}
}
