package multiaddr

// You **MUST** register your multicodecs with
// https://github.com/multiformats/multicodec before adding them here.
const (
	P_IP4               = 0x0004
	P_TCP               = 0x0006
	P_UDP               = 0x0111
	P_DCCP              = 0x0021
	P_IP6               = 0x0029
	P_IP6ZONE           = 0x002A
	P_QUIC              = 0x01CC
	P_SCTP              = 0x0084
	P_UDT               = 0x012D
	P_UTP               = 0x012E
	P_UNIX              = 0x0190
	P_P2P               = 0x01A5
	P_IPFS              = 0x01A5 // alias for backwards compatability
	P_HTTP              = 0x01E0
	P_HTTPS             = 0x01BB
	P_ONION             = 0x01BC // also for backwards compatibility
	P_ONION3            = 0x01BD
	P_GARLIC64          = 0x01CA
	P_P2P_WEBRTC_DIRECT = 0x0114
)

var (
	protoIP4 = Protocol{
		Name:       "ip4",
		Code:       P_IP4,
		VCode:      CodeToVarint(P_IP4),
		Size:       32,
		Path:       false,
		Transcoder: TranscoderIP4,
	}
	protoTCP = Protocol{
		Name:       "tcp",
		Code:       P_TCP,
		VCode:      CodeToVarint(P_TCP),
		Size:       16,
		Path:       false,
		Transcoder: TranscoderPort,
	}
	protoUDP = Protocol{
		Name:       "udp",
		Code:       P_UDP,
		VCode:      CodeToVarint(P_UDP),
		Size:       16,
		Path:       false,
		Transcoder: TranscoderPort,
	}
	protoDCCP = Protocol{
		Name:       "dccp",
		Code:       P_DCCP,
		VCode:      CodeToVarint(P_DCCP),
		Size:       16,
		Path:       false,
		Transcoder: TranscoderPort,
	}
	protoIP6 = Protocol{
		Name:       "ip6",
		Code:       P_IP6,
		VCode:      CodeToVarint(P_IP6),
		Size:       128,
		Transcoder: TranscoderIP6,
	}
	// these require varint
	protoIP6ZONE = Protocol{
		Name:       "ip6zone",
		Code:       P_IP6ZONE,
		VCode:      CodeToVarint(P_IP6ZONE),
		Size:       LengthPrefixedVarSize,
		Path:       false,
		Transcoder: TranscoderIP6Zone,
	}
	protoSCTP = Protocol{
		Name:       "sctp",
		Code:       P_SCTP,
		VCode:      CodeToVarint(P_SCTP),
		Size:       16,
		Transcoder: TranscoderPort,
	}
	protoONION2 = Protocol{
		Name:       "onion",
		Code:       P_ONION,
		VCode:      CodeToVarint(P_ONION),
		Size:       96,
		Transcoder: TranscoderOnion,
	}
	protoONION3 = Protocol{
		Name:       "onion3",
		Code:       P_ONION3,
		VCode:      CodeToVarint(P_ONION3),
		Size:       296,
		Transcoder: TranscoderOnion3,
	}
	protoGARLIC64 = Protocol{
		Name:       "garlic64",
		Code:       P_GARLIC64,
		VCode:      CodeToVarint(P_GARLIC64),
		Size:       LengthPrefixedVarSize,
		Transcoder: TranscoderGarlic64,
	}
	protoUTP = Protocol{
		Name:  "utp",
		Code:  P_UTP,
		VCode: CodeToVarint(P_UTP),
	}
	protoUDT = Protocol{
		Name:  "udt",
		Code:  P_UDT,
		VCode: CodeToVarint(P_UDT),
	}
	protoQUIC = Protocol{
		Name:  "quic",
		Code:  P_QUIC,
		VCode: CodeToVarint(P_QUIC),
	}
	protoHTTP = Protocol{
		Name:  "http",
		Code:  P_HTTP,
		VCode: CodeToVarint(P_HTTP),
	}
	protoHTTPS = Protocol{
		Name:  "https",
		Code:  P_HTTPS,
		VCode: CodeToVarint(P_HTTPS),
	}
	protoP2P = Protocol{
		Name:       "ipfs",
		Code:       P_P2P,
		VCode:      CodeToVarint(P_P2P),
		Size:       LengthPrefixedVarSize,
		Transcoder: TranscoderP2P,
	}
	protoUNIX = Protocol{
		Name:       "unix",
		Code:       P_UNIX,
		VCode:      CodeToVarint(P_UNIX),
		Size:       LengthPrefixedVarSize,
		Path:       true,
		Transcoder: TranscoderUnix,
	}
	protoP2P_WEBRTC_DIRECT = Protocol{
		Name:  "p2p-webrtc-direct",
		Code:  P_P2P_WEBRTC_DIRECT,
		VCode: CodeToVarint(P_P2P_WEBRTC_DIRECT),
	}
)

func init() {
	for _, p := range []Protocol{
		protoIP4,
		protoTCP,
		protoUDP,
		protoDCCP,
		protoIP6,
		protoIP6ZONE,
		protoSCTP,
		protoONION2,
		protoONION3,
		protoGARLIC64,
		protoUTP,
		protoUDT,
		protoQUIC,
		protoHTTP,
		protoHTTPS,
		protoP2P,
		protoUNIX,
		protoP2P_WEBRTC_DIRECT,
	} {
		if err := AddProtocol(p); err != nil {
			panic(err)
		}
	}

	// explicitly set both of these
	protocolsByName["p2p"] = protoP2P
	protocolsByName["ipfs"] = protoP2P
}
