# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.x.x   | ✅ Current development |

## Reporting a Vulnerability

If you discover a security vulnerability in WorldWeaver, please report it responsibly.

### How to Report

1. **Do NOT open a public issue** for security vulnerabilities.
2. Use [GitHub Security Advisories](https://github.com/apil-khadka/worldweaver/security/advisories/new) to report privately.
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### Response Timeline

- Acknowledgment within 48 hours
- Assessment within 1 week
- Fix or mitigation plan within 2 weeks for critical issues

## Security Principles

WorldWeaver follows these security practices:

### Server-Side Validation
- All client input is validated server-side before affecting world state
- Maximum power radius enforced (64 cells)
- Rate limiting on player actions
- Malformed messages are rejected without crashing

### No Client Trust
- Clients cannot directly mutate world state
- Client sends "apply heat at (x,y)" — server decides the consequence
- Clients never send "cell (x,y) is now fire"

### Input Boundaries
- Maximum message size enforced
- Coordinate bounds validated
- Influence consumption checked before action processing
- WebSocket origin policy in production

### No Secrets in Client
- Frontend contains no API keys, tokens, or private configuration
- All client code is treated as public

## Scope

WorldWeaver is a hackathon project. It implements basic security hygiene but is NOT designed for production deployment with untrusted users at scale without additional hardening.
