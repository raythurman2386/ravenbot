# Security Policy

## Reporting a Vulnerability

We take the security of ravenbot seriously. If you discover a security vulnerability, please do not disclose it publicly until we have had a chance to fix it.

### How to Report

Please report security vulnerabilities by emailing support@raythurman.dev or by creating a **private** vulnerability report on GitHub if enabled.

We will acknowledge your report within 48 hours and will provide an estimated timeframe for addressing the vulnerability.

### Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | :white_check_mark: |
| < 1.0   | :x:                |

## Security Best Practices for Self-Hosting

Since ravenbot is designed to be self-hosted, please ensure you follow these practices:

1.  **Keep Docker images updated**: Regularly pull the latest images.
2.  **Secure your `.env` file**: Ensure it is not accessible to unauthorized users.
3.  **Restrict Network Access**: If exposing the bot's ports, use a firewall or VPN.
4.  **Review MCP Permissions**: Be careful when adding new MCP servers that have filesystem or network access and only do so in a playground environment.
