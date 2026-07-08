// Command keygen generates HMAC-based API keys for IOSwarm agents.
//
// Usage:
//
//	ioswarm-keygen --master=<secret> --agent=<agent-id>
//	ioswarm-keygen --master=<secret> --agent=lobster-001
//
// The generated key can be passed to ioswarm-agent via --api-key flag.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

const apiKeyPrefix = "iosw_"

func main() {
	master := flag.String("master", "", "master secret (shared with coordinator)")
	agent := flag.String("agent", "", "agent ID to generate key for")
	flag.Parse()

	if *master == "" || *agent == "" {
		fmt.Fprintf(os.Stderr, "Usage: ioswarm-keygen --master=<secret> --agent=<agent-id>\n")
		os.Exit(1)
	}

	mac := hmac.New(sha256.New, []byte(*master))
	mac.Write([]byte(*agent))
	key := apiKeyPrefix + hex.EncodeToString(mac.Sum(nil))

	fmt.Printf("Agent:   %s\n", *agent)
	fmt.Printf("API Key: %s\n", key)
}
