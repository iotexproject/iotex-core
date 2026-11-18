# Try quantum-safe staking buckets on a local chain

Status: **draft, non-activated**. The IIP-65 fork is not scheduled on testnet
or mainnet. This guide starts a single-node local chain with the fork on, so
every `ioctl stake2 guard` command in [`qsb-ioctl.md`](./qsb-ioctl.md) can be
run end to end.

## 1. Start the chain

Build ioctl, create two throwaway accounts, and start the chain with both
funded:

```sh
make ioctl                                  # or: go build -o bin/ioctl ./tools/ioctl
bin/ioctl config set endpoint localhost:14014 --insecure
bin/ioctl account create                    # twice: an OWNER and a SPONSOR
bin/ioctl account import key owner          # paste each private key
bin/ioctl account import key sponsor

tools/qsb-localnet/run.sh OWNER_ADDRESS SPONSOR_ADDRESS
```

`run.sh` builds the server, writes a genesis and a config into `.qsb-localnet/`,
and runs the node in the foreground (Ctrl-C stops it; `--reset` starts a fresh
chain). Each address gets 100,000,000 IOTX. Use these accounts on this chain
only.

What the script sets, and why:

- **Every fork at height 1, including `toBeEnabledHeight`.** The guard is on
  only when `IsToBeEnabled(h) && IsZanzibarGamma(h)`
  (`action/protocol/context.go`); setting `toBeEnabledHeight` alone leaves it
  off (`bucket guard is disabled`). Genesis validation also rejects Zanzibar
  before Greenland or Xingu, so all forks move together. The script reads the
  fork list from `blockchain/genesis/genesis.go`.
- **`poll.enableGravityChainVoting: false`**, so the node does not need an
  Ethereum endpoint for the election committee.
- **Every data path in `.qsb-localnet/data`.** The defaults point at
  `/var/data`, including the actpool store and the consensus DB.

## 2. Make a bucket to guard

The guard covers native buckets other than a candidate's self-stake, so
register a candidate and then stake to it from the owner:

```sh
bin/ioctl stake2 register delegate1 SPONSOR SPONSOR SPONSOR 1200000 91 --no-bls -s sponsor -w
bin/ioctl stake2 create 500 delegate1 0 -s owner -w     # bucket 1
```

The register call creates bucket 0 (the self-stake), so the owner's bucket is 1.
A stake duration of 0 days lets the bucket unstake right away; with a longer
duration a guarded unstake passes the guard and is then refused by the normal
maturity rule (status 207).

## 3. Guard it and operate it

```sh
bin/ioctl stake2 guard vault create                 # a separate guard mnemonic
bin/ioctl stake2 guard bind 1 --key-index 0 -s owner -w
bin/ioctl stake2 guard show 1

bin/ioctl stake2 release 1 -s owner -w              # status 216: bucket is guarded
bin/ioctl stake2 guard unstake 1 -s sponsor -w      # status 1, nonce 0 -> 1

# offline variant
bin/ioctl stake2 guard unstake 1 --sponsor sponsor --out op.json
bin/ioctl stake2 guard sign op.json
bin/ioctl stake2 guard submit op.json -s sponsor -w
```

Expected on an Apple M-series laptop: BIND uses 1,788,100 gas (1.7881 IOTX at
the minimum gas price) and each guard signature takes about 0.27 s to make.
`ioctl` reports the chain as mainnet (chain ID 1) because the local config
keeps the default chain ID; the domain still binds every signature to this
chain's genesis hash, so nothing signed here is valid elsewhere.
