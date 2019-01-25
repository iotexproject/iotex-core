package pubsub

import (
	"fmt"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

const SignPrefix = "libp2p-pubsub:"

func verifyMessageSignature(m *pb.Message) error {
	pubk, err := messagePubKey(m)
	if err != nil {
		return err
	}

	xm := *m
	xm.Signature = nil
	xm.Key = nil
	bytes, err := xm.Marshal()
	if err != nil {
		return err
	}

	bytes = withSignPrefix(bytes)

	valid, err := pubk.Verify(bytes, m.Signature)
	if err != nil {
		return err
	}

	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func messagePubKey(m *pb.Message) (crypto.PubKey, error) {
	var pubk crypto.PubKey

	pid, err := peer.IDFromBytes(m.From)
	if err != nil {
		return nil, err
	}

	if m.Key == nil {
		// no attached key, it must be extractable from the source ID
		pubk, err = pid.ExtractPublicKey()
		if err != nil {
			return nil, fmt.Errorf("cannot extract signing key: %s", err.Error())
		}
		if pubk == nil {
			return nil, fmt.Errorf("cannot extract signing key")
		}
	} else {
		pubk, err = crypto.UnmarshalPublicKey(m.Key)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal signing key: %s", err.Error())
		}

		// verify that the source ID matches the attached key
		if !pid.MatchesPublicKey(pubk) {
			return nil, fmt.Errorf("bad signing key; source ID %s doesn't match key", pid)
		}
	}

	return pubk, nil
}

func signMessage(pid peer.ID, key crypto.PrivKey, m *pb.Message) error {
	bytes, err := m.Marshal()
	if err != nil {
		return err
	}

	bytes = withSignPrefix(bytes)

	sig, err := key.Sign(bytes)
	if err != nil {
		return err
	}

	m.Signature = sig

	pk, _ := pid.ExtractPublicKey()
	if pk == nil {
		pubk, err := key.GetPublic().Bytes()
		if err != nil {
			return err
		}
		m.Key = pubk
	}

	return nil
}

func withSignPrefix(bytes []byte) []byte {
	return append([]byte(SignPrefix), bytes...)
}
