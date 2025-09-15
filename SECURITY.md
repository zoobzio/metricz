# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          | Status |
| ------- | ------------------ | ------ |
| latest  | ✅ | Active development |
| < latest | ❌ | Security fixes only for critical issues |

## Reporting a Vulnerability

We take the security of metricz seriously. If you have discovered a security vulnerability in this project, please report it responsibly.

### How to Report

**Please DO NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **GitHub Security Advisories** (Preferred)
   - Go to the [Security tab](https://github.com/zoobzio/metricz/security) of this repository
   - Click "Report a vulnerability"
   - Fill out the form with details about the vulnerability

2. **Email**
   - Send details to the repository maintainer through GitHub profile contact information
   - Use PGP encryption if possible for sensitive details

### What to Include

Please include the following information (as much as you can provide) to help us better understand the nature and scope of the possible issue:

- **Type of issue** (e.g., race condition, memory corruption, denial of service, etc.)
- **Full paths of source file(s)** related to the manifestation of the issue
- **The location of the affected source code** (tag/branch/commit or direct URL)
- **Any special configuration required** to reproduce the issue
- **Step-by-step instructions** to reproduce the issue
- **Proof-of-concept or exploit code** (if possible)
- **Impact of the issue**, including how an attacker might exploit the issue
- **Concurrent access patterns** that trigger the issue
- **Your name and affiliation** (optional)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Initial Assessment**: Within 7 days, we will provide an initial assessment of the report
- **Resolution Timeline**: We aim to resolve critical issues within 30 days
- **Disclosure**: We will coordinate with you on the disclosure timeline

### Preferred Languages

We prefer all communications to be in English.

## Security Best Practices

When using metricz in your applications, we recommend:

1. **Keep Dependencies Updated**
   ```bash
   go get -u github.com/zoobzio/metricz
   ```

2. **Registry Isolation**
   - Use separate registries for different services/components
   - Avoid sharing registries across security boundaries
   - Consider access control for registry instances

3. **Key Management**
   - Define metric keys as constants in a central location
   - Review key naming for potential information disclosure
   - Avoid embedding sensitive data in metric names

4. **Resource Management**
   - Monitor memory usage in high-cardinality scenarios
   - Set appropriate limits on metric creation
   - Use registry isolation to prevent resource exhaustion

5. **Concurrent Access**
   - Trust built-in thread safety for standard operations
   - Use proper synchronization for registry lifecycle management
   - Test concurrent access patterns in your application

6. **Data Privacy**
   - Avoid recording sensitive data in metric values
   - Consider data retention policies for exported metrics
   - Review metric exports for information leakage

## Security Features

metricz includes several built-in security features:

- **Type Safety**: Compile-time key validation prevents runtime injection
- **Thread Safety**: Atomic operations prevent race conditions
- **Memory Safety**: Bounds checking and overflow protection
- **Registry Isolation**: Independent namespaces prevent metric pollution
- **Zero Dependencies**: No external dependencies reduce attack surface
- **Input Validation**: Invalid values are safely ignored rather than causing panics

## Common Security Considerations

### Race Conditions

metricz uses atomic operations to prevent race conditions, but consider:

- Registry lifecycle management in concurrent environments
- Proper initialization order in multi-goroutine applications
- Testing with `-race` flag during development

### Memory Exhaustion

While metricz is memory-efficient, consider:

- Unbounded metric creation from user input
- Registry cleanup in long-running applications
- Monitoring memory usage of metric storage

### Information Disclosure

Be careful about:

- Sensitive data in metric key names
- Metric values revealing business logic
- Timing attacks through metric collection patterns

### Denial of Service

Protect against:

- Excessive metric creation rates
- Large histogram bucket configurations
- Unbounded registry growth

## Automated Security Scanning

This project uses:

- **CodeQL**: GitHub's semantic code analysis for security vulnerabilities
- **Dependabot**: Automated dependency updates (for dev dependencies)
- **golangci-lint**: Static analysis including security linters
- **gosec**: Go security checker for common security issues
- **Codecov**: Coverage tracking to ensure security-critical code is tested

## Testing Security

Security-related tests include:

```bash
# Race condition detection
go test -race ./...

# Memory stress testing
go test ./testing/reliability/

# Concurrent modification tests
go test ./testing/integration/

# Overflow protection tests
go test ./testing/reliability/ -run TestOverflow
```

## Vulnerability Disclosure Policy

- Security vulnerabilities will be disclosed via GitHub Security Advisories
- We follow a 90-day disclosure timeline for non-critical issues
- Critical vulnerabilities may be disclosed sooner after patches are available
- We will credit reporters who follow responsible disclosure practices

## Security Architecture

### Atomic Operations

metricz uses Go's `sync/atomic` package for all metric updates:

- Prevents race conditions without locks
- Guarantees memory consistency
- Protects against torn reads/writes

### Type System Protection

The `metricz.Key` type prevents common injection attacks:

- Compile-time validation of metric names
- No dynamic string construction in hot paths
- Prevents typos that could create security vulnerabilities

### Memory Management

- Fixed-size data structures where possible
- Bounds checking on all array/slice access
- No unsafe memory operations
- Overflow protection on arithmetic operations

## Credits

We thank the following individuals for responsibly disclosing security issues:

_This list is currently empty. Be the first to help improve our security!_

---

**Last Updated**: 2024-12-13