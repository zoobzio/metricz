# Contributing to metricz

Thank you for your interest in contributing to metricz! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/metricz.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit your changes with a descriptive message
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a Pull Request

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Add comments for exported functions and types
- Keep functions small and focused
- Use atomic operations for thread-safe operations

### Testing

- Write tests for new functionality
- Ensure all tests pass: `go test ./...`
- Include benchmarks for performance-critical code
- Test concurrent access patterns with `-race`
- Aim for good test coverage

### Documentation

- Update documentation for API changes
- Add examples for new features
- Keep doc comments clear and concise
- Document thread safety guarantees

## Types of Contributions

### Bug Reports

- Use GitHub Issues
- Include minimal reproduction code
- Describe expected vs actual behavior
- Include Go version and OS
- For performance issues, include benchmark results

### Feature Requests

- Open an issue for discussion first
- Explain the use case
- Consider backwards compatibility
- Consider performance implications

### Code Contributions

#### Adding Metric Types

New metric types should:
- Implement atomic operations for thread safety
- Follow existing naming conventions
- Include comprehensive tests with race detection
- Add documentation with examples
- Consider memory allocation patterns

#### Performance Improvements

Performance changes should:
- Include before/after benchmarks
- Maintain correctness guarantees
- Consider impact on concurrent access
- Test with realistic workloads

#### Examples

New examples should:
- Solve a real-world monitoring problem
- Include tests and benchmarks
- Have a descriptive README
- Follow the existing structure
- Demonstrate key type safety

## Pull Request Process

1. **Keep PRs focused** - One feature/fix per PR
2. **Write descriptive commit messages**
3. **Update tests and documentation**
4. **Ensure CI passes**
5. **Run performance tests if applicable**
6. **Respond to review feedback**

## Testing

Run the full test suite:
```bash
go test ./...
```

Run with race detection:
```bash
go test -race ./...
```

Run benchmarks:
```bash
go test -bench=. ./...
```

Run specific benchmark categories:
```bash
# Core operations
go test -bench="Counter_Inc$|Gauge_Set$|Histogram_Observe$" -benchmem ./testing/benchmarks/

# Real usage patterns  
go test -bench=BenchmarkHTTPRequestTracking -benchmem ./testing/benchmarks/

# Memory stress tests
go test -bench=BenchmarkMemoryPressure ./testing/reliability/
```

## Project Structure

```
metricz/
├── *.go                    # Core library files
├── *_test.go              # Unit tests
├── examples/              # Example implementations
│   ├── api-gateway/       # HTTP request tracking
│   ├── batch-processor/   # Batch processing metrics
│   ├── service-mesh/      # Microservice monitoring
│   └── worker-pool/       # Concurrent processing
├── testing/               # Comprehensive test suite
│   ├── benchmarks/        # Performance tests
│   ├── integration/       # Integration tests
│   └── reliability/       # Stress and reliability tests
└── docs/                  # Documentation
```

## Commit Messages

Follow conventional commits:
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test additions/changes
- `refactor:` Code refactoring
- `perf:` Performance improvements
- `chore:` Maintenance tasks

Examples:
- `feat(histogram): add custom bucket configuration`
- `fix(counter): prevent overflow in extreme values`
- `perf(atomic): optimize memory access patterns`

## Performance Standards

metricz prioritizes performance. Contributions should maintain:

- **Zero allocation updates**: Metric operations should not allocate memory
- **Sub-nanosecond operations**: Core operations (Inc, Set) under 10ns
- **Lock-free reads**: Value() operations should never block
- **Constant time lookup**: Registry operations in O(1) time

Verify performance impact:
```bash
# Before your changes
go test -bench=BenchmarkYourFeature -count=10 > before.txt

# After your changes  
go test -bench=BenchmarkYourFeature -count=10 > after.txt

# Compare results
benchcmp before.txt after.txt
```

## Type Safety Requirements

All contributions must maintain compile-time type safety:

- All metric keys must use `metricz.Key` type
- Raw strings should be rejected by the compiler
- Functions should accept typed parameters only
- Error handling should be compile-time checkable

## Memory Management

metricz avoids memory allocation during normal operation:

- Use atomic operations instead of mutexes where possible
- Pre-allocate fixed-size structures
- Avoid interface boxing in hot paths
- Test memory usage with `go test -benchmem`

## Documentation Requirements

All exported functions and types must have:

- Clear doc comments following Go conventions
- Usage examples in doc comments
- Thread safety guarantees documented
- Error conditions documented
- Performance characteristics noted

## Release Process

### Automated Releases

This project uses automated release versioning. To create a release:

1. Go to Actions → Release → Run workflow
2. Leave "Version override" empty for automatic version inference
3. Click "Run workflow"

The system will:
- Automatically determine the next version from conventional commits
- Create a git tag
- Generate release notes via GoReleaser
- Publish the release to GitHub

### Commit Conventions for Versioning
- `feat:` new features (minor version: 1.2.0 → 1.3.0)
- `fix:` bug fixes (patch version: 1.2.0 → 1.2.1)  
- `feat!:` breaking changes (major version: 1.2.0 → 2.0.0)
- `docs:`, `test:`, `chore:` no version change

Example: `feat(registry): add metric export functionality`

### Version Preview on Pull Requests
Every PR automatically shows the next version that will be created:
- Check PR comments for "Version Preview" 
- Updates automatically as you add commits
- Helps verify your commits have the intended effect

## Questions?

- Open an issue for questions
- Check existing issues first
- Be patient and respectful
- For performance questions, include benchmark data

Thank you for contributing to metricz!