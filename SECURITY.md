# Security policy

## Reporting a vulnerability

**Do not file a public issue for security problems.**

Use GitHub's private vulnerability reporting:
**[Report a vulnerability →](https://github.com/librito-io/kobo-agent/security/advisories/new)**

This sends a private report visible only to maintainers. We aim to respond within 7 days.

## Scope

- Pairing flow and device-token claim
- The Librito API token at rest on the Kobo (storage, file permissions)
- Sync transport (token handling over the network)
- The local SQLite highlight store the agent reads/writes

## Out of scope

- Reports requiring physical access to the Kobo device
- The Librito web backend — report those via that project's own policy
- Issues in third-party software the agent runs alongside (Kobo Nickel, NickelMenu,
  NickelDBus) — report to those projects directly
