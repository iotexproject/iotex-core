// l4test connects to a live delegate and validates StreamStateDiffs.
// Tests: 11.2 (content validation), 11.3 (live streaming), 11.4 (catch-up).
//
// Usage:
//
//	go run ./ioswarm/cmd/l4test --addr 178.62.196.98:14689 --secret <master_secret>
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/iotexproject/iotex-core/v2/ioswarm/proto" // JSON codec
)

// agentCred implements grpc.PerRPCCredentials with the correct metadata keys.
type agentCred struct {
	agentID string
	token   string
}

func (c *agentCred) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"x-ioswarm-agent-id": c.agentID,
		"x-ioswarm-token":    c.token,
	}, nil
}

func (c *agentCred) RequireTransportSecurity() bool { return false }

// Types matching proto/types.go (JSON codec)
type streamStateDiffsRequest struct {
	AgentID    string `json:"agent_id"`
	FromHeight uint64 `json:"from_height"`
}

type stateDiffResponse struct {
	Height      uint64           `json:"height"`
	Entries     []stateDiffEntry `json:"entries"`
	DigestBytes []byte           `json:"digest_bytes"`
}

type stateDiffEntry struct {
	WriteType uint8  `json:"write_type"`
	Namespace string `json:"namespace"`
	Key       []byte `json:"key"`
	Value     []byte `json:"value,omitempty"`
}

func main() {
	addr := flag.String("addr", "178.62.196.98:14689", "delegate gRPC address")
	secret := flag.String("secret", "", "HMAC master secret")
	agentID := flag.String("agent", "l4test-probe", "agent ID")
	catchup := flag.Uint64("catchup", 10, "how many blocks to catch up (0 = latest only)")
	duration := flag.Duration("duration", 60*time.Second, "how long to stream live diffs")
	flag.Parse()

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "error: --secret required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *duration+30*time.Second)
	defer cancel()

	// Compute HMAC token (iosw_ prefix + HMAC-SHA256 hex)
	mac := hmac.New(sha256.New, []byte(*secret))
	mac.Write([]byte(*agentID))
	token := "iosw_" + hex.EncodeToString(mac.Sum(nil))

	// Connect with PerRPCCredentials
	cred := &agentCred{agentID: *agentID, token: token}
	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(cred),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	// First: register agent so coordinator knows us
	log.Printf("registering agent %s...", *agentID)
	if err := registerAgent(ctx, conn, *agentID); err != nil {
		log.Printf("register warning (may be OK): %v", err)
	}

	// Get current tip from a quick stream
	log.Printf("probing current tip...")
	tip, err := probeTip(ctx, conn, *agentID, *secret)
	if err != nil {
		log.Fatalf("probe tip: %v", err)
	}
	log.Printf("current tip: %d", tip)

	fromHeight := tip
	if *catchup > 0 && tip > *catchup {
		fromHeight = tip - *catchup
	}

	// Stream state diffs
	log.Printf("streaming from height %d (catch-up %d blocks)...", fromHeight, tip-fromHeight)

	req := &streamStateDiffsRequest{
		AgentID:    *agentID,
		FromHeight: fromHeight,
	}
	reqBytes, _ := json.Marshal(req)

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "StreamStateDiffs",
		ServerStreams:  true,
	}, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		log.Fatalf("new stream: %v", err)
	}
	if err := stream.SendMsg(json.RawMessage(reqBytes)); err != nil {
		log.Fatalf("send request: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("close send: %v", err)
	}

	// Receive and validate
	var (
		count       int
		lastHeight  uint64
		catchupDone bool
		gaps        int
		invalidNS   int
		invalidWT   int
		emptyDiffs  int
		startTime   = time.Now()
		liveStart   time.Time
	)

	validNamespaces := map[string]bool{
		"Account": true, "Code": true, "Contract": true, "_meta": true,
		"System": true, "Rewarding": true, "Candidate": true, "Bucket": true,
		"Staking": true,
	}

	timeout := time.After(*duration)

	for {
		select {
		case <-timeout:
			goto done
		default:
		}

		var resp stateDiffResponse
		respBytes := json.RawMessage{}
		if err := stream.RecvMsg(&respBytes); err != nil {
			if err == io.EOF {
				log.Printf("stream ended (EOF)")
				break
			}
			log.Printf("recv error: %v", err)
			break
		}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			log.Printf("unmarshal error: %v (raw: %s)", err, string(respBytes))
			continue
		}

		count++

		// Validate monotonic height
		if lastHeight > 0 && resp.Height != lastHeight+1 {
			gaps++
			log.Printf("  GAP: expected height %d, got %d", lastHeight+1, resp.Height)
		}

		// Detect catch-up → live transition
		if !catchupDone && resp.Height >= tip {
			catchupDone = true
			liveStart = time.Now()
			log.Printf("  catch-up complete at height %d (%d diffs in %v)", resp.Height, count, time.Since(startTime).Round(time.Millisecond))
		}

		// Validate entries
		if len(resp.Entries) == 0 {
			emptyDiffs++
		}
		for _, e := range resp.Entries {
			if !validNamespaces[e.Namespace] {
				invalidNS++
				log.Printf("  INVALID namespace: %q at height %d", e.Namespace, resp.Height)
			}
			if e.WriteType > 1 {
				invalidWT++
				log.Printf("  INVALID writeType: %d at height %d", e.WriteType, resp.Height)
			}
		}

		// Print progress
		if count <= 5 || count%50 == 0 {
			log.Printf("  height=%d entries=%d digest=%x", resp.Height, len(resp.Entries), resp.DigestBytes[:min(8, len(resp.DigestBytes))])
		}

		lastHeight = resp.Height
	}

done:
	elapsed := time.Since(startTime)
	liveCount := 0
	if catchupDone && liveStart.After(startTime) {
		liveCount = count - int(tip-fromHeight) - 1
	}

	// Report
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  L4 State Diff Test Results")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Total diffs received:   %d\n", count)
	fmt.Printf("  Height range:           %d → %d\n", fromHeight, lastHeight)
	fmt.Printf("  Elapsed:                %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Catch-up blocks:        %d\n", tip-fromHeight)
	fmt.Printf("  Live blocks:            %d\n", liveCount)
	fmt.Println("  ─────────────────────────────────────────────────────")

	pass := true

	// 11.2: Content validation
	fmt.Printf("  11.2 Invalid namespaces:  %d", invalidNS)
	if invalidNS == 0 { fmt.Println(" ✅") } else { fmt.Println(" ❌"); pass = false }

	fmt.Printf("  11.2 Invalid writeTypes:  %d", invalidWT)
	if invalidWT == 0 { fmt.Println(" ✅") } else { fmt.Println(" ❌"); pass = false }

	fmt.Printf("  11.2 Empty diffs:         %d", emptyDiffs)
	if emptyDiffs == 0 { fmt.Println(" ✅") } else { fmt.Printf(" (may be OK for empty blocks)\n") }

	fmt.Printf("  11.2 Height gaps:         %d", gaps)
	if gaps == 0 { fmt.Println(" ✅") } else { fmt.Println(" ❌"); pass = false }

	// 11.3: Live streaming
	fmt.Printf("  11.3 Live diffs received: %d", liveCount)
	if liveCount > 0 { fmt.Println(" ✅") } else { fmt.Println(" ⚠️  (need longer duration?)"); pass = false }

	// 11.4: Catch-up
	fmt.Printf("  11.4 Catch-up complete:   %v", catchupDone)
	if catchupDone { fmt.Println(" ✅") } else { fmt.Println(" ❌"); pass = false }

	fmt.Println("  ─────────────────────────────────────────────────────")
	if pass {
		fmt.Println("  OVERALL: PASS ✅")
	} else {
		fmt.Println("  OVERALL: FAIL ❌")
	}
	fmt.Println("═══════════════════════════════════════════════════════")
}

func registerAgent(ctx context.Context, conn *grpc.ClientConn, agentID string) error {
	type registerReq struct {
		AgentID    string `json:"agent_id"`
		Capability int32  `json:"capability"`
		Region     string `json:"region"`
		Version    string `json:"version"`
	}
	req := registerReq{AgentID: agentID, Capability: 0, Region: "test", Version: "l4test-1.0"}
	reqBytes, _ := json.Marshal(req)

	var respBytes json.RawMessage
	err := conn.Invoke(ctx, "/ioswarm.IOSwarm/Register", json.RawMessage(reqBytes), &respBytes)
	if err != nil {
		return err
	}
	log.Printf("  registered: %s", string(respBytes))
	return nil
}

func probeTip(ctx context.Context, conn *grpc.ClientConn, agentID, secret string) (uint64, error) {
	// Request from height 0 = latest only, read first diff to get tip
	req := streamStateDiffsRequest{AgentID: agentID, FromHeight: 0}
	reqBytes, _ := json.Marshal(req)

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream, err := conn.NewStream(probeCtx, &grpc.StreamDesc{
		StreamName:   "StreamStateDiffs",
		ServerStreams: true,
	}, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		return 0, err
	}
	if err := stream.SendMsg(json.RawMessage(reqBytes)); err != nil {
		return 0, err
	}
	stream.CloseSend()

	// Wait for first live diff (up to 10s)
	var resp stateDiffResponse
	respBytes := json.RawMessage{}
	if err := stream.RecvMsg(&respBytes); err != nil {
		return 0, fmt.Errorf("no diff received: %w", err)
	}
	json.Unmarshal(respBytes, &resp)
	cancel() // done probing
	return resp.Height, nil
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
