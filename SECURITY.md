# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Vulnerability Disclosure Policy

### Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

**Please DO NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **GitHub Security Advisories**: Use the [Private vulnerability reporting](https://github.com/JavierLimon/openbao-teradata-secret-plugin/security/advisories/new) feature to report vulnerabilities securely.

2. **Email**: Send an email to the maintainers with details of the vulnerability.

### What to Include

When reporting a vulnerability, please include:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact assessment of the vulnerability

### Response Timeline

- **Initial Response**: We aim to acknowledge receipt of vulnerability reports within 48 hours.
- **Status Update**: We will provide a more detailed response within 7 days, including:
  - Confirmation of the vulnerability
  - Our assessment of severity
  - Expected timeline for a fix
- **Resolution**: We will work to release a fix as quickly as possible, depending on complexity.

### Disclosure Policy

- We follow a **coordinated disclosure** process.
- We request that you give us reasonable time to address the vulnerability before public disclosure.
- We will credit reporters in the security advisory (if desired).

### Scope

This security policy applies to:

- The `openbao-teradata-secret-plugin` repository
- All released versions of the plugin
- Documentation and build artifacts

### Out of Scope

The following are not considered vulnerabilities in this project:

- Issues in third-party dependencies (please report upstream)
- Social engineering attacks
- Physical security attacks
- Denial of service attacks (resource exhaustion scenarios should still be reported)

### Security Best Practices

When using this plugin:

1. **Credential Storage**: Never commit credentials to version control
2. **Connection Strings**: Use secure methods to inject sensitive configuration
3. **Role Permissions**: Follow the principle of least privilege
4. **Credential Rotation**: Regularly rotate root credentials
5. **Monitoring**: Enable audit logging to track credential usage
6. **Network**: Run with appropriate network isolation

### Security Updates

Security updates will be released as patch versions and announced through:

- GitHub Security Advisories
- Release notes
- Community channels

### Supported Encryption

- Connection strings should use ODBC driver encryption features where available
- Credentials stored in the backend are encrypted by OpenBao
- Transport layer security (TLS) should be enabled for all database connections

### Third-Party Dependencies

We regularly update dependencies to address security vulnerabilities. Please report any known vulnerabilities in dependencies to the respective upstream projects.

### Contact

For security-related inquiries not suitable for public channels, please contact the maintainers directly.
