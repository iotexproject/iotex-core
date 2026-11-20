// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"strconv"
	"testing"
	"time"

	p2plib "github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

// collectMetrics returns all series currently held by a collector.
func collectMetrics(t *testing.T, c prometheus.Collector) []*dto.Metric {
	ch := make(chan prometheus.Metric)
	go func() {
		c.Collect(ch)
		close(ch)
	}()
	var out []*dto.Metric
	for m := range ch {
		var pb dto.Metric
		require.NoError(t, m.Write(&pb))
		out = append(out, &pb)
	}
	return out
}

func metricLabels(m *dto.Metric) map[string]string {
	labels := make(map[string]string, len(m.GetLabel()))
	for _, l := range m.GetLabel() {
		labels[l.GetName()] = l.GetValue()
	}
	return labels
}

// unicastInCount sums the inbound unicast counter over all message labels.
func unicastInCount(t *testing.T) float64 {
	var sum float64
	for _, m := range collectMetrics(t, _p2pMsgCounter) {
		l := metricLabels(m)
		if l["protocol"] == "unicast" && l["direction"] == "in" {
			sum += m.GetCounter().GetValue()
		}
	}
	return sum
}

func isKnownMsgTypeLabel(v string) bool {
	if v == "unknown" {
		return true
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return false
	}
	_, ok := iotexrpc.MessageType_name[int32(n)]
	return ok
}

func TestUnicastInboundUnknownMsgTypeSharesOneSeries(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const chainID = 7
	a := NewAgent(Config{
		Host:              "127.0.0.1",
		Port:              testutil.RandomPort(),
		ReconnectInterval: 150 * time.Second,
		MasterKey:         "unicast-label-receiver",
	}, chainID, hash.ZeroHash256, func(proto.Message) (bool, error) {
		return false, nil
	}, func(context.Context, uint32, string, proto.Message) {},
		func(context.Context, uint32, peer.AddrInfo, proto.Message) {})
	r.NoError(a.Start(ctx))
	t.Cleanup(func() { r.NoError(a.Stop(ctx)) })
	target, err := a.Info()
	r.NoError(err)

	sender, err := p2plib.NewHost(ctx, p2plib.HostName("127.0.0.1"),
		p2plib.Port(testutil.RandomPort()), p2plib.MasterKey("unicast-label-sender"),
		p2plib.SecureIO(), p2plib.DHTProtocolID(chainID))
	r.NoError(err)
	t.Cleanup(func() { r.NoError(sender.Close()) })

	topic := _unicastTopic + a.(*agent).topicSuffix
	counterSeries := len(collectMetrics(t, _p2pMsgCounter))
	latencySeries := len(collectMetrics(t, _p2pMsgLatency))
	before := unicastInCount(t)

	// message types outside the enum, each distinct
	const n = 64
	for i := 0; i < n; i++ {
		mt := iotexrpc.MessageType(20000 + i)
		_, defined := iotexrpc.MessageType_name[int32(mt)]
		r.False(defined)
		data, err := proto.Marshal(&iotexrpc.UnicastMsg{
			ChainId:   chainID,
			PeerId:    sender.HostIdentity(),
			MsgType:   mt,
			MsgBody:   []byte{1},
			Timestamp: timestamppb.Now(),
		})
		r.NoError(err)
		r.NoError(sender.Unicast(ctx, target, topic, data))
	}
	// every message has been handled once the inbound total moved by n
	r.Eventually(func() bool {
		return unicastInCount(t)-before >= n
	}, 20*time.Second, 50*time.Millisecond)

	// at most one new series per vector ("unknown"/"failure")
	r.LessOrEqual(len(collectMetrics(t, _p2pMsgCounter)), counterSeries+1)
	r.LessOrEqual(len(collectMetrics(t, _p2pMsgLatency)), latencySeries+1)
	var unknown float64
	for _, m := range collectMetrics(t, _p2pMsgCounter) {
		l := metricLabels(m)
		if l["protocol"] == "unicast" && l["direction"] == "in" && l["message"] == "unknown" {
			unknown += m.GetCounter().GetValue()
		}
	}
	r.GreaterOrEqual(unknown, float64(n))
	for _, c := range []prometheus.Collector{_p2pMsgCounter, _p2pMsgLatency} {
		for _, m := range collectMetrics(t, c) {
			v := metricLabels(m)["message"]
			r.True(isKnownMsgTypeLabel(v), "unexpected message label %q", v)
		}
	}
}
