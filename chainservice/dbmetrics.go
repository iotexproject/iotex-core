// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
)

// dbFileSizeUpdateInterval is how often the on-disk size of each DB file is sampled.
// DB files grow slowly, so a coarse interval keeps the observability overhead negligible.
const dbFileSizeUpdateInterval = 5 * time.Minute

var dbFileSizeMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_db_file_size_bytes",
		Help: "On-disk size in bytes of each node database file, labeled by logical db name and path.",
	},
	[]string{"db", "path"},
)

func init() {
	// Register once per process. Registration in init() avoids duplicate-register
	// panics when multiple ChainService instances are constructed (e.g. sub-chains, tests).
	prometheus.MustRegister(dbFileSizeMtc)
}

// dbFile describes a configured on-disk database and its logical name (used as the metric label).
type dbFile struct {
	name string
	path string
}

// localChainDBPath resolves the on-disk path of the chain DB. Unlike the other DB paths,
// ChainDBPath may be a URL: a "file://"/plain path opens a local file, while "grpc://" points at
// a remote block DAO that has no local file. It mirrors the parsing in buildBlockDAO. It returns
// an empty string when there is no local file to stat (remote or unparseable), so the caller skips it.
func localChainDBPath(path string) string {
	if path == "" {
		return ""
	}
	uri, err := url.Parse(path)
	if err != nil {
		// Not a URL; treat as a plain filesystem path (buildBlockDAO would fail earlier otherwise).
		return path
	}
	switch uri.Scheme {
	case "", "file":
		return uri.Path
	default:
		// e.g. "grpc": remote block DAO, no local file to measure.
		return ""
	}
}

// configuredDBFiles returns the bounded, static set of database files configured for the node.
// Empty paths are skipped so that disabled components do not create series.
func configuredDBFiles(cfg config.Config) []dbFile {
	candidates := []dbFile{
		{"chain", localChainDBPath(cfg.Chain.ChainDBPath)},
		{"trie", cfg.Chain.TrieDBPath},
		{"index", cfg.Chain.IndexDBPath},
		{"bloomfilter_index", cfg.Chain.BloomfilterIndexDBPath},
		{"candidate_index", cfg.Chain.CandidateIndexDBPath},
		{"staking_index", cfg.Chain.StakingIndexDBPath},
		{"contractstaking_index", cfg.Chain.ContractStakingIndexDBPath},
		{"blob_store", cfg.Chain.BlobStoreDBPath},
		{"patch_receipt_index", cfg.Chain.PatchReceiptIndexPath},
		{"history_index", cfg.Chain.HistoryIndexPath},
		{"gravity_chain", cfg.Chain.GravityChainDB.DbPath},
		{"consensus", cfg.Consensus.RollDPoS.ConsensusDBPath},
	}
	files := make([]dbFile, 0, len(candidates))
	for _, c := range candidates {
		if c.path == "" {
			continue
		}
		files = append(files, c)
	}
	return files
}

// dbFileSizeBytes returns the on-disk size of path. A path may be a single file (boltdb) or a
// directory (pebbledb), in which case the sizes of all contained files are summed. Any stat/walk
// error (e.g. a missing or rotated file) is returned so the caller can skip that sample.
func dbFileSizeBytes(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	var total int64
	err = filepath.WalkDir(path, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		fi, statErr := d.Info()
		if statErr != nil {
			// File may have been rotated away mid-walk; skip it rather than aborting.
			return nil
		}
		total += fi.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// newDBFileSizeCollector builds a lifecycle-managed recurring task that periodically records the
// on-disk size of each configured DB file into the dbFileSizeMtc gauge. It returns nil when no DB
// files are configured, so callers should guard the lifecycle registration accordingly.
func newDBFileSizeCollector(cfg config.Config) *routine.RecurringTask {
	files := configuredDBFiles(cfg)
	if len(files) == 0 {
		return nil
	}
	update := func() {
		for _, f := range files {
			size, err := dbFileSizeBytes(f.path)
			if err != nil {
				// Missing or rotated file: skip this sample, keep the collector alive.
				log.L().Debug("skip db file size metric", zap.String("db", f.name), zap.String("path", f.path), zap.Error(err))
				continue
			}
			dbFileSizeMtc.WithLabelValues(f.name, f.path).Set(float64(size))
		}
	}
	// Sample once at startup so the metric is populated before the first tick.
	update()
	return routine.NewRecurringTask(update, dbFileSizeUpdateInterval)
}
