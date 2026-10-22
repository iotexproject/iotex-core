# Security Policy

IoTeX is a live, permissionless network that secures real value. We work with
security researchers to find and fix vulnerabilities before they can be
exploited, and we credit them publicly when the fix is released.

**Do not report vulnerabilities in public GitHub issues, pull requests,
discussions, or community channels such as Discord or Telegram.**

## Reporting a vulnerability

Report privately through GitHub Private Vulnerability Reporting:

**<https://github.com/iotexproject/iotex-core/security/advisories/new>**

A good report includes:

* the affected component, and the version or commit hash;
* the impact as you understand it, for example consensus failure, unauthorized
  state change, node crash, or resource exhaustion;
* reproduction steps or a proof of concept, preferably against a local devnet;
* any suggested fix or mitigation;
* how you would like to be credited.

You do not need a working exploit. A precise root-cause analysis is enough for
us to start.

## Scope

**In scope:** the `iotex-core` node software, including:

* consensus (Roll-DPoS, block validation, endorsements);
* state transition and protocols: accounts, staking, rewarding, poll, and EVM
  execution;
* the action pool and transaction validation;
* P2P networking and block sync;
* the gRPC and Web3 JSON-RPC API served by nodes;
* the `ioctl` CLI shipped from this repository;
* genesis and hard-fork activation logic.

**Out of scope** (report to the respective project):

* vulnerabilities in third-party dependencies that are not reachable through
  iotex-core (report them upstream; if they are reachable through iotex-core,
  report here too);
* websites, front ends, and ecosystem dApps not maintained in this repository;
* rate limiting or availability of public RPC endpoints operated by third
  parties;
* social engineering, phishing, and physical attacks;
* findings that require a compromised operator machine or leaked private keys.

## Rules of engagement

* Test against a local devnet or your own nodes. Do not run exploits against
  mainnet or testnet, and do not degrade the network or other people's nodes.
* Do not access, modify, or move funds or data that are not yours.
* Keep the details confidential until a fix is released and disclosure is
  coordinated with us.

Research that follows these rules is considered authorized. We will not pursue
or support legal action against researchers acting in good faith under this
policy.

## Severity

We assess impact on the network first, then record a CVSS score in the advisory.

| Severity | Examples |
|---|---|
| **Critical** | Consensus failure or chain split; network halt; unauthorized minting or movement of funds; remote code execution on nodes; exposure of node or delegate private keys |
| **High** | Remotely triggerable crash or resource exhaustion affecting a significant share of block producers; state divergence on some nodes; bypass of protocol rules without direct fund loss |
| **Medium** | Crash or incorrect results on API nodes; cheap resource exhaustion of a single node; incorrect data returned by RPC that clients rely on |
| **Low** | Issues with limited, local, or hard-to-trigger impact |

## What happens after you report

| Step | Target |
|---|---|
| Acknowledgement | within 2 business days |
| Triage and initial severity assessment | within 5 business days |
| Status updates | at least weekly until resolved |
| Fix for Critical / High findings | prioritized immediately; typically days, not weeks |

Every confirmed finding is tracked as a GitHub Security Advisory. We develop the
fix in a temporary private fork tied to that advisory, so the vulnerability is
not visible in public branches before release. Public commit messages and
release notes describe the change neutrally until disclosure.

## Coordinated disclosure

1. **Fix and release.** The fix is merged and shipped in a tagged release of
   every maintained release line.
2. **Rollout.** For consensus-critical issues, patched builds may be distributed
   to block producers before the public release. We track adoption through
   node version reporting.
3. **Disclosure.** We publish the advisory, and request a CVE where appropriate,
   once one of these is true:
   * a sufficient share of active block producers runs the patched version
     (normally at least two-thirds);
   * 90 days have passed since the report, unless we agree on a different
     timeline with the reporter.

If a vulnerability is being actively exploited, we will act faster and tell you
before we disclose.

## Supported versions

Security fixes are released for:

* the latest release line;
* the previous release line, until the next hard fork that requires the new
  line activates on mainnet.

Node operators should always run the latest patch release of their line.
Published advisories:
<https://github.com/iotexproject/iotex-core/security/advisories>

## Recognition

With your permission, we credit you in the published advisory and the release
notes.

## For block producers and node operators

Security releases are announced in the delegate channels. When a release is
marked as a required security upgrade, upgrade promptly. A patched node also
protects the consensus you participate in.
