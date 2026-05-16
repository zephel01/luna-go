# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (main) | ✅ |
| older tags | ❌ |

## Reporting a Vulnerability

**Please do not open a public GitHub Issue for security vulnerabilities.**

Report vulnerabilities privately via GitHub's [Security Advisories](../../security/advisories/new) feature (recommended), or by emailing:

**zephel01@gmail.com**

Include the following in your report:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Potential impact

### Response Timeline

| Milestone | Target |
|-----------|--------|
| Acknowledgement | Within 3 business days |
| Initial assessment | Within 7 business days |
| Fix / advisory | Best effort, typically within 30 days |

## Supply Chain Security

- Dependencies are pinned via `go.sum` (SHA-256 hash verification)
- CVE scanning runs on every push via `govulncheck` (Go official vulnerability DB)
- Secret scanning runs on every push via `gitleaks`
- Dependency updates are automated weekly via Dependabot (Go modules + GitHub Actions)
- New dependency PRs are reviewed manually before merge

## Out of Scope

- Vulnerabilities in downstream Ollama or OpenAI endpoints
- Social engineering attacks
- Denial of service against local CLI usage
