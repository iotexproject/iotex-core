# Security Policy

IoTeX is a live, permissionless network that secures real value. We work with
security researchers to find and fix vulnerabilities before they can be
exploited, and, with their permission, credit them when we publish the advisory.

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
  iotex-core (report those upstream). Dependency issues that are reachable
  through iotex-core are in scope — report them here;
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

For good-faith research that follows this policy and targets systems we own or
operate, IoTeX will not initiate legal action based solely on that research.
This policy cannot authorize activity on third-party systems or bind other
parties, including independent node operators.

## Severity

We assess impact on the network first, then record a CVSS score in the advisory.

| Severity | Examples |
|---|---|
| **Critical** | Consensus failure or chain split; network halt; unauthorized minting or movement of funds; remote code execution on nodes; exposure of node or delegate private keys |
| **High** | Remotely triggerable crash or resource exhaustion affecting a significant share of block producers; state divergence on some nodes; bypass of protocol rules without direct fund loss |
| **Medium** | Crash or incorrect results on API nodes; cheap resource exhaustion of a single node; incorrect data returned by RPC that clients rely on |
| **Low** | Issues with limited, local, or hard-to-trigger impact |

## What happens after you report

We aim to acknowledge reports within **2 business days** and to provide an
initial assessment, including severity, within **5 business days**, followed by
regular status updates. Fix and release timing depends on impact, validation,
and safe rollout across the network; we will tell you if it changes.

Confirmed findings are tracked as GitHub Security Advisories. We normally
prepare fixes privately and coordinate their publication with the release;
public commit messages and release notes describe the change neutrally until
disclosure. We may adjust this workflow when needed to protect the network.

## Coordinated disclosure

We coordinate disclosure with the reporter after a fix is available and node
operators have had a reasonable opportunity to upgrade. For consensus-critical
issues, patched builds may reach block producers before the public release.
Timing may change based on exploitation, network safety, and patch adoption; we
will discuss material changes with you. Where appropriate we request a CVE when
the advisory is published.

If a vulnerability is being actively exploited, we will act faster and tell you
before we disclose.

## Supported versions

Currently supported: **v2.5.x**. Older release lines receive security fixes only
when explicitly announced for a specific release. Node operators should always
run the latest patch release.

Published advisories:
<https://github.com/iotexproject/iotex-core/security/advisories>

## Recognition

With your permission, we credit you when we publish the advisory.

## For block producers and node operators

Security releases are announced in the delegate channels. When a release is
marked as a required security upgrade, upgrade promptly. A patched node also
protects the consensus you participate in.
