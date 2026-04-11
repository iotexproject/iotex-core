# Security Policy

## Supported Versions

Security updates are provided for the latest minor release of `iotex-core` on
the `master` branch. Older versions may not receive fixes — please upgrade to
the most recent tagged release before filing a report.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report suspected vulnerabilities privately using one of the following:

- **GitHub Private Vulnerability Reporting** (preferred):
  <https://github.com/iotexproject/iotex-core/security/advisories/new>
- **Email**: security@iotex.io

When reporting, please include:

- A clear description of the issue and its impact
- Steps to reproduce, proof-of-concept, or exploit code if available
- The affected version(s), commit hash, or tag
- Your name and contact info (optional — anonymous reports are accepted)

## Response Process

- We will acknowledge receipt within **3 business days**.
- We will provide an initial assessment within **7 business days**.
- We aim to release a fix for confirmed high-severity issues within **30 days**
  of the report, coordinating a disclosure timeline with the reporter.
- Credit will be given to reporters in the release notes unless anonymity is
  requested.

## Scope

In scope:

- The `iotex-core` node implementation (this repository)
- Consensus, state, blockchain, and cryptographic components
- Official build / release artifacts published from this repository

Out of scope:

- Third-party dApps, wallets, or tooling not maintained in this repository
- Infrastructure operated by node operators or third parties
- Social engineering, phishing, or physical attacks

## Safe Harbor

We will not pursue legal action against researchers who:

- Act in good faith and follow this policy
- Avoid privacy violations, data destruction, or service disruption
- Give us reasonable time to remediate before public disclosure
