# Investigation: Container Restart Loop (64 restarts)

**Date**: 2026-03-18
**Host**: 178.62.196.98 (ubuntu-s-8vcpu-16gb-480gb-intel-ams3-01)
**Container**: `iotex` (ioswarm-v2.3.5 branch, image raullen/iotex-core:ioswarm-v12)
**Reported symptom**: Container RestartCount = 64, restart policy = `on-failure`

---

## Timeline

| Time (UTC) | Event |
|------------|-------|
| Mar 14 17:25 | Container created, first start. Runs stable ~3 days. |
| Mar 17 19:11 | Second start seen in logs (1 restart in ~3 days). |
| Mar 17 22:10 – Mar 18 00:12 | **Crash loop**: 62 restarts in ~2 hours, every ~2 minutes. |
| Mar 18 00:12 – 02:49 | ~2.5 hour gap (container eventually caught up or stabilized briefly). |
| Mar 18 02:49 – 02:54 | Another burst of restarts (4 OOM kills in dmesg). |
| Mar 18 02:54 | Latest start, currently running. |

## Root Cause #1: OOM Killer (Primary)

**Evidence**: `dmesg` shows 9+ OOM kills of `iotex-server`:

```
[1889601] Out of memory: Killed process 467343 (iotex-server) anon-rss:15306788kB (~14.6 GiB)
[1889719] Out of memory: Killed process 471532 (iotex-server) anon-rss:15658604kB (~14.9 GiB)
[1889830] Out of memory: Killed process 471787 (iotex-server) anon-rss:15651500kB (~14.9 GiB)
[1889935] Out of memory: Killed process 472829 (iotex-server) anon-rss:15656452kB (~14.9 GiB)
```

**System resources**:
- Total RAM: 15.6 GiB (no swap configured)
- Container memory limit: none (0 = unlimited)
- At OOM: iotex-server uses ~14.9 GiB (95%+ of total), leaving nothing for OS/nginx/sshd

**Why memory spikes**: During block sync (catching up after restart), iotex-core processes blocks rapidly. Combined with:
1. trie.db is 44.4 GiB — BoltDB mmap'd into memory; heavy random reads during sync cause page cache pressure
2. `MemCacheSize: 134217728` (128 MiB) for DB layer
3. `maxCacheSize: 1000` blocks
4. `WorkingSetCacheSize: 20` — each working set holds state diffs
5. IOSwarm prefetcher runs concurrently during sync, spawning goroutines that read state (additional memory pressure)
6. DiffStore appending every block during sync (205K+ entries, 1.5 GiB on disk, all loaded/indexed by BoltDB)

**Crash loop mechanism**:
1. Container starts → begins syncing from last committed block
2. Rapid block processing + state reads + diffstore writes → memory grows to ~15 GiB
3. OOM killer kills iotex-server → container restarts (on-failure policy)
4. Restart from same block → same memory spike → killed again → loop
5. Each restart advances a few hundred blocks (diffstore: 46152525 → 46152571 → 46152615, ~45 blocks per cycle)
6. Eventually catches up to chain tip, memory stabilizes, loop stops

## Root Cause #2: "Miss genesis context" Panic (Secondary, IOSwarm bug)

**Evidence**: 689 panics caught, 2067 "Miss genesis context" log entries.

```
adapter.go:153: acct, err := accountutil.AccountState(context.Background(), s.sf, ioAddr)
                                                       ^^^^^^^^^^^^^^^^^
                                                       BUG: should be s.chainCtx()
```

**Call chain**:
```
Prefetcher.Prefetch() → StateReaderAdapter.AccountState()
  → accountutil.AccountState(context.Background(), ...)
  → AccountStateWithHeight() → protocol.WithFeatureCtx()
  → genesis.MustExtractGenesisContext() → PANIC: "Miss genesis context"
```

The `AccountState` method at `adapter.go:153` passes `context.Background()` instead of `s.chainCtx()`. The `chainCtx()` method exists and correctly attaches genesis context, but `AccountState` doesn't use it.

The panic is **recovered** by the defer at line 143-147, so it doesn't crash the process, but:
- It fires every second (once per poll cycle) during block sync when pending txs exist
- Each panic logs a full stack trace (~1 KB each × 689 = wasted CPU + I/O)
- The affected transaction gets skipped (no state prefetched → sent without snapshot)

**Same bug likely exists in**: `GetCode()` (line 178) and `GetStorageAt()` (line 193) — need to verify if they also use `context.Background()`.

## Root Cause #3: DiffStore unbounded growth (Contributing)

**Config**: `DiffRetainHeight: 0` (keep all diffs forever)
**Current state**: 205K+ entries (block 45,951,339 → 46,156,676), 1.5 GiB on disk
**Impact**: BoltDB opens all pages at startup; during heavy writes (sync), this adds memory pressure and I/O contention.

## Impact Assessment

| Issue | Severity | Impact |
|-------|----------|--------|
| OOM crash loop | **High** | Node offline for ~2 hours during catch-up. Agents disconnected. Rewards not settling. |
| Genesis context panic | **Medium** | 689 panics caught. Tasks dispatched without state. Wastes CPU on stack traces. |
| DiffStore growth | **Low** | 1.5 GiB and growing. Adds to memory pressure. No pruning configured. |

## Recommended Fixes

### Fix 1: Genesis context bug (code change, high priority)

In `adapter.go:153`, change:
```go
// Before:
acct, err := accountutil.AccountState(context.Background(), s.sf, ioAddr)

// After:
acct, err := accountutil.AccountState(s.chainCtx(), s.sf, ioAddr)
```

Also audit `GetCode()` and `GetStorageAt()` for the same issue.

### Fix 2: Memory limits + swap (ops, high priority)

Option A — Add swap (quick fix):
```bash
fallocate -l 8G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

Option B — Container memory limit with OOM score adjustment:
```bash
docker update --memory=14g --memory-swap=20g iotex
```

Option C — Upgrade to 32 GiB RAM (permanent fix).

### Fix 3: Pause IOSwarm during block sync (code change, medium priority)

Add a sync-awareness check in `pollAndDispatch()`:
```go
if s.bc.TipHeight() < latestKnownHeight - 10 {
    // Still syncing, skip ioswarm polling
    return
}
```

This prevents the prefetcher from running during rapid block sync, reducing both panics and memory pressure.

### Fix 4: Enable DiffStore pruning (config change, low priority)

Add to config:
```yaml
ioswarm:
  diffRetainHeight: 10000  # keep ~27 hours of diffs
```

This keeps statediffs.db bounded. 10K blocks (~27 hours at 10s/block) is more than enough for agent catch-up.

## Fixes Applied

### Fix 1: `adapter.go:153` — Genesis context bug
```diff
- acct, err := accountutil.AccountState(context.Background(), s.sf, ioAddr)
+ acct, err := accountutil.AccountState(s.chainCtx(), s.sf, ioAddr)
```
`GetCode()` and `GetStorageAt()` already use `s.chainCtx()` — only `AccountState` was wrong.

### Fix 2: `coordinator.go:pollAndDispatch()` — Skip during block sync
Added sync detection: if `lastBlockTimestamp` is >30s behind wall clock, skip the entire poll cycle. This prevents the prefetcher from spawning goroutines and reading state during rapid block sync, which was a major contributor to memory pressure.

### Fix 3: `coordinator.go:ReceiveStateDiff()` — DiffStore auto-pruning
- Default `DiffRetainHeight` changed from 0 (keep all) to 10000 blocks (~27 hours)
- Prune runs every 1000 blocks instead of every block (reduces BoltDB write amplification)
- On first deploy, will prune ~195K stale entries from statediffs.db (~1.4 GiB reclaimed)

## Will OOM still happen?

**Honest answer: these fixes reduce the risk significantly, but 15.6 GiB RAM with no swap is tight for a mainnet full node.**

What the fixes address:
- **Sync-skip** eliminates ioswarm's memory contribution (~200-500 MiB) during the critical catch-up phase
- **DiffStore pruning** will reclaim ~1.4 GiB disk and reduce BoltDB mmap pressure
- **Genesis context fix** eliminates 689+ goroutine panics (minor memory impact, major CPU/log savings)

What remains:
- **iotex-core base memory** is the real issue: trie.db (44 GiB) + state processing during sync can use 14+ GiB on its own, without any ioswarm involvement. This is a pre-existing constraint, not an ioswarm bug.
- If the node restarts and needs to catch up >100 blocks, the base iotex-core memory alone can OOM this 16 GiB machine.

**Recommendation**: Add 8 GiB swap as a safety net. Swap is slower but prevents OOM kills:
```bash
fallocate -l 8G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

## Current Status (as of 02:55 UTC)

- Container is running, block height ~46,156,706, appears to have caught up
- 8 agents connected (2×L2, 1×L3, 5×L4)
- Shadow accuracy: 100% (3/3 since last restart, 99.33% cumulative before restart)
- On-chain settlement: working (epoch 2 distributed)
- CPU: 668% (high due to recent sync), Memory: 11.65 GiB / 15.6 GiB
- Risk: another OOM if memory doesn't settle after sync completes
