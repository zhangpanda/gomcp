# Security Policy

## Supported Versions

Security updates are applied to the latest minor release on the **main** branch.

| Version | Supported          |
| ------- | ------------------ |
| ≥ 1.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for undisclosed security vulnerabilities.

Instead, report through one of these channels:

1. **GitHub Security Advisories** — use [Private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) for this repository if enabled.
2. **Maintainer contact** — open a **draft** or **private** communication path as listed in the repository **Security** policy on GitHub (owner: `zhangpanda`).

Include:

- A short description of the issue and its impact
- Steps to reproduce or proof of concept (if safe to share)
- Affected versions or commit ranges, if known

You should receive an initial acknowledgment within a few business days. We will coordinate disclosure and fixes before any public details are published.

## Scope

This policy covers the **GoMCP** library and the **example** binaries in this repository. Deployments that embed GoMCP remain the responsibility of their operators (network exposure, secrets, auth middleware configuration, etc.).
