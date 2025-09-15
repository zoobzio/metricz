---
name: Bug report
about: Create a report to help us improve
title: '[BUG] '
labels: 'bug'
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Create metrics registry with '...'
2. Record metric values '...'
3. Observe behavior '...'
4. See error

**Code Example**
```go
// Minimal code example that reproduces the issue
package main

import (
    "github.com/zoobzio/metricz"
)

func main() {
    // Your code here
    registry := metricz.NewRegistry()
    // ...
}
```

**Expected behavior**
A clear and concise description of what you expected to happen.

**Actual behavior**
What actually happened, including any error messages, stack traces, or incorrect metric values.

**Environment:**
 - OS: [e.g. macOS, Linux, Windows]
 - Architecture: [e.g. amd64, arm64]
 - Go version: [e.g. 1.21.0]
 - metricz version: [e.g. v1.0.0]
 - Concurrency level: [e.g. single-threaded, 10 goroutines, high contention]

**Concurrency Information (if applicable)**
 - Number of concurrent goroutines accessing metrics
 - Frequency of metric updates (e.g. 1000/sec per goroutine)
 - Are you experiencing race conditions or data corruption?
 - Any atomic operation failures?

**Additional context**
Add any other context about the problem here, including:
- Performance characteristics observed
- Memory usage patterns
- Any custom configurations or integrations
- Related issues or similar problems