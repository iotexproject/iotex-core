package logfilter

import (
	"bytes"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// BlockHeightPrefix is an prefix which will concatenate with blocknumber when adding into rangeBloomFilter
	BlockHeightPrefix = "height"
)

// LogFilter contains options for contract log filtering.
type LogFilter struct {
	pbFilter *iotexapi.LogsFilter
	// FilterLogsRequest.Topics restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}, {B}}         matches topic A in first position, B in second position
	// {{A, B}}, {C, D}}  matches topic (A OR B) in first position, (C OR D) in second position
}

// NewLogFilter returns a new log filter
func NewLogFilter(in *iotexapi.LogsFilter) *LogFilter {
	return &LogFilter{
		pbFilter: in,
	}
}

// MatchLogs returns matching logs in a given block
func (l *LogFilter) MatchLogs(receipts []*action.Receipt) []*action.Log {
	res := []*action.Log{}
	for _, r := range receipts {
		for j, v := range r.Logs() {
			if l.match(v.ConvertToLogPb()) {
				res = append(res, r.Logs()[j])
			}
		}
	}
	return res
}

// match checks if a given log matches the filter
// TODO: replace iotextypes.Log with action.log
func (l *LogFilter) match(log *iotextypes.Log) bool {
	addrMatch := len(l.pbFilter.Address) == 0
	if !addrMatch {
		for _, e := range l.pbFilter.Address {
			if e == log.ContractAddress {
				addrMatch = true
				break
			}
		}
	}
	if !addrMatch {
		return false
	}
	if len(l.pbFilter.Topics) > len(log.Topics) {
		// trying to match a prefix of log's topic list, so topics longer than that is consider invalid
		return false
	}
	if len(l.pbFilter.Topics) == 0 {
		// {} or nil matches any address or topic list
		return true
	}
	for i, e := range l.pbFilter.Topics {
		if e == nil || len(e.Topic) == 0 {
			continue
		}
		target := log.Topics[i]
		match := false
		for _, v := range e.Topic {
			if bytes.Equal(v, target) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// ExistInBloomFilter returns true if topics of filter exist in the bloom filter
func (l *LogFilter) ExistInBloomFilter(bf bloom.BloomFilter) bool {
	if bf == nil {
		return true
	}

	for _, e := range l.pbFilter.Topics {
		if e == nil || len(e.Topic) == 0 {
			continue
		}

		for _, v := range e.Topic {
			if bf.Exist(v) {
				return true
			}
		}
	}
	// {} or nil matches any address or topic list
	return len(l.pbFilter.Topics) == 0
}

// ExistInBloomFilterv2 returns true if addresses and topics of filter exist in the range bloom filter (topic: position-sensitive)
func (l *LogFilter) ExistInBloomFilterv2(bf bloom.BloomFilter) bool {
	if len(l.pbFilter.Address) > 0 {
		flag := false
		for _, addr := range l.pbFilter.Address {
			if bf.Exist([]byte(addr)) {
				flag = true
			}
		}
		if !flag {
			return false
		}
	}

	if len(l.pbFilter.Topics) > 0 {
		for i, e := range l.pbFilter.Topics {
			if e == nil || len(e.Topic) == 0 {
				continue
			}
			match := false
			for _, v := range e.Topic {
				if bf.Exist(append(byteutil.Uint64ToBytes(uint64(i)), v...)) {
					match = true
					continue
				}
			}
			if !match {
				return false
			}
		}
		return true
	}

	// {} or nil matches any address or topic list
	return true
}

// SelectBlocksFromRangeBloomFilter filters out RangeBloomFilter for selecting block numbers which have logFilter's topics/address
// TODO[dorothy]: optimize using goroutine
func (l *LogFilter) SelectBlocksFromRangeBloomFilter(bf bloom.BloomFilter, start, end uint64) []uint64 {
	blkNums := make([]uint64, 0)
	for blockHeight := start; blockHeight <= end; blockHeight++ {
		Heightkey := append([]byte(BlockHeightPrefix), byteutil.Uint64ToBytes(blockHeight)...)
		if len(l.pbFilter.Address) > 0 {
			flag := false
			for _, addr := range l.pbFilter.Address {
				if bf.Exist(append(Heightkey, []byte(addr)...)) {
					flag = true
					break
				}
			}
			if !flag {
				continue
			}
		}
		if len(l.pbFilter.Topics) > 0 {
			flag := false
		checkTopic:
			for _, e := range l.pbFilter.Topics {
				if e == nil || len(e.Topic) == 0 {
					continue
				}
				for _, v := range e.Topic {
					if bf.Exist(append(Heightkey, v...)) {
						flag = true
						break checkTopic
					}
				}
			}
			if !flag {
				continue
			}
		}
		blkNums = append(blkNums, blockHeight)
	}
	return blkNums
}
