package madns

import (
	"bytes"
	"fmt"

	ma "github.com/multiformats/go-multiaddr"
)

// Extracted from source of truth for multicodec codes: https://github.com/multiformats/multicodec
const (
	P_DNS4    = 0x0036
	P_DNS6    = 0x0037
	P_DNSADDR = 0x0038
)

var Dns4Protocol = ma.Protocol{
	Code:       P_DNS4,
	Size:       ma.LengthPrefixedVarSize,
	Name:       "dns4",
	VCode:      ma.CodeToVarint(P_DNS4),
	Transcoder: DnsTranscoder,
}
var Dns6Protocol = ma.Protocol{
	Code:       P_DNS6,
	Size:       ma.LengthPrefixedVarSize,
	Name:       "dns6",
	VCode:      ma.CodeToVarint(P_DNS6),
	Transcoder: DnsTranscoder,
}
var DnsaddrProtocol = ma.Protocol{
	Code:       P_DNSADDR,
	Size:       ma.LengthPrefixedVarSize,
	Name:       "dnsaddr",
	VCode:      ma.CodeToVarint(P_DNSADDR),
	Transcoder: DnsTranscoder,
}

func init() {
	err := ma.AddProtocol(Dns4Protocol)
	if err != nil {
		panic(fmt.Errorf("error registering dns4 protocol: %s", err))
	}
	err = ma.AddProtocol(Dns6Protocol)
	if err != nil {
		panic(fmt.Errorf("error registering dns6 protocol: %s", err))
	}
	err = ma.AddProtocol(DnsaddrProtocol)
	if err != nil {
		panic(fmt.Errorf("error registering dnsaddr protocol: %s", err))
	}
}

var DnsTranscoder = ma.NewTranscoderFromFunctions(dnsStB, dnsBtS, dnsVal)

func dnsVal(b []byte) error {
	if bytes.IndexByte(b, '/') >= 0 {
		return fmt.Errorf("domain name %q contains a slash", string(b))
	}
	return nil
}

func dnsStB(s string) ([]byte, error) {
	return []byte(s), nil
}

func dnsBtS(b []byte) (string, error) {
	return string(b), nil
}
