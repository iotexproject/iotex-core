# CLAUDE.md — action/protocol/rewarding

Module-specific guidance for the rewarding protocol. Read [/AGENTS.md](../../../AGENTS.md) first for repo-wide rules.

## What this package does

Block and epoch rewards, foundation bonus, exempt list, and the claim flow.
All state lives in a single isolated namespace, `state.RewardingNamespace`,
keyed by short prefixes:

| Key | Purpose |
|---|---|
| `"adm"` | admin config (reward amounts, delegate counts, bonus period) |
| `"fnd"` | fund balance (`totalBalance`, `unclaimedBalance`) |
| `"adm" \|\| addr` | per-address unclaimed reward balance |
| `"xpt"` | exempt-from-epoch-reward addresses |
| `BlockRewardHistoryKeyPrefix \|\| height` | sentinel: "this height already rewarded" |
| `EpochRewardHistoryKeyPrefix \|\| epoch` | sentinel: "this epoch already rewarded" |

Entry points: `protocol.go`, `fund.go`, `reward.go`, `admin.go`,
`rewardingpb/rewarding.proto`.

---

## Module red lines

### Reward amounts change only at hardfork heights

- `CreatePreStates()` swaps reward amounts at named heights:
  - **Aleutian** — epoch reward
  - **Dardanelles**, **Wake** — block reward
  - **Kamchatka** — foundation-bonus period extension
- Never mutate reward amounts outside this path; always go through
  `SetReward(ctx, sm, amount, blockOrEpoch)` with `assertAmount() > 0`.
- See `protocol.go:103–172`.

### Idempotency guards via sentinel history keys

- `assertNoRewardYet(blockOrEpoch, height)` checks for the history sentinel
  before granting. Granting twice panics: `"reward history already exists"`.
- The history record is intentionally empty — the **key's existence** is the
  signal. Do not "fix" this by adding a payload.
- See `reward.go:702–714`.

### Fund accounting — `unclaimedBalance ≤ totalBalance`, always

- `Deposit` increases both balances.
- `GrantReward` decreases `unclaimedBalance` only; the granted amount moves
  to the recipient's per-address account.
- `Claim` decreases `totalBalance` and pays out from the per-address account.
- Integer division in `splitEpochReward()` truncates; the remainder stays in
  `unclaimedBalance` and is never auto-redistributed. Do not assume exact
  conservation at epoch boundaries.

### Greenland v1 → v2 state migration is one-shot

- Runs once at `GreenlandBlockHeight` (`migrateValueGreenland`). State not
  migrated by then is lost.
- Do not write new keys to the v1 path. Do not retroactively touch the
  migration code.

### Reward amounts are decimal strings in proto

- All `big.Int` reward amounts serialize as base-10 strings (e.g.
  `"123456789012345678901234567890"`).
- Always parse with `new(big.Int).SetString(s, 10)` and check the `ok` flag.
  Malformed amounts panic genesis init.

### Cross-protocol coupling: staking is required for slashing

- Unproductive-delegate slashing (`slashUqd`) calls into the staking
  protocol to slash self-stake buckets. If staking is missing or in a bad
  state, the epoch grant fails. Do not silence these errors.

---

## Sentinels and gotchas

- **Votes == 0 means "no reward."** Candidates with zero votes are excluded
  from both foundation bonus and epoch reward (`reward.go:321, 695`). This
  is not a flag, just a consequence of the weighted split — don't add a
  separate "exempt" check thinking it's missing.
- **No minimum claim amount.** A 1-rau claim is valid and executes
  immediately. Plan accordingly if reward amounts ever shrink.
- **Block/epoch grants are system actions**, generated automatically
  post-block, not user-initiated. The user-initiated action is only
  `ClaimFromRewardingFund`.

---

## Where to look

| Topic | File |
|---|---|
| Protocol registration, Start, fork gating | `protocol.go` |
| Admin config (reward amounts, bonus period) | `admin.go` |
| Fund struct, deposit/claim accounting | `fund.go` |
| Block/epoch grant logic, slashing | `reward.go` |
| Proto schema (decimal-string amounts) | `rewardingpb/rewarding.proto` |
