# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | ✅ Yes             |
| < 1.0   | ❌ No (pre-release) |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

To report a security issue, please use one of the following channels:

1. **GitHub Security Advisories** (preferred): Navigate to the [Security](https://github.com/shreyansh-shankar/getitback/security/advisories/new) tab and click *Report a vulnerability*.
2. **Email**: Send a PGP-encrypted report to `security@[maintainer domain]`.

When reporting, please include:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof of concept
- Any relevant version or environment information

We aim to acknowledge reports within **48 hours** and to provide a fix or mitigation plan within **7 days** for critical issues.

## Scope

The following are **in scope**:

- Information disclosure (e.g., backup files written with insecure permissions)
- Encryption bypass or key exposure
- Path traversal during archive extraction
- Command injection via module logic
- Privilege escalation during restore

The following are **out of scope**:

- Vulnerabilities in third-party tools that `getitback` invokes (e.g., `pg_dump`, `mongodump`)
- Issues requiring physical access to the machine
- Social engineering

## Disclosure Policy

We follow coordinated disclosure. We will credit reporters in the release notes unless anonymity is requested.
