#!/bin/bash
# Reliability test runner with configurable load levels

set -e

echo "================================================"
echo "Metricz Reliability Tests"
echo "================================================"

# Default mode: Quick CI tests
if [ "$1" == "stress" ]; then
    echo "Running in STRESS TEST mode (heavy load)..."
    echo "This will take longer but tests extreme conditions"
    export METRICZ_STRESS_TEST=true
elif [ "$1" == "ci" ] || [ -z "$1" ]; then
    echo "Running in CI mode (quick tests)..."
    echo "Set METRICZ_STRESS_TEST=true or use './run_tests.sh stress' for heavy testing"
    unset METRICZ_STRESS_TEST
else
    echo "Usage: $0 [ci|stress]"
    echo "  ci     - Quick tests for CI (default)"
    echo "  stress - Heavy stress testing"
    exit 1
fi

echo ""
echo "Configuration:"
if [ "$METRICZ_STRESS_TEST" == "true" ]; then
    echo "  - Workers: 1000"
    echo "  - Operations: 10000" 
    echo "  - Test duration: 5-10 seconds"
    echo "  - Metrics created: 100,000+"
else
    echo "  - Workers: 10-50"
    echo "  - Operations: 100-1000"
    echo "  - Test duration: 100-500ms"
    echo "  - Metrics created: 1,000"
fi

echo ""
echo "Running tests..."
echo "------------------------------------------------"

# Run tests with timeout protection
if [ "$METRICZ_STRESS_TEST" == "true" ]; then
    # Stress tests get 60 second timeout
    timeout 60s go test -v -race . || {
        echo "Tests failed or timed out after 60 seconds"
        exit 1
    }
else
    # CI tests get 10 second timeout
    timeout 10s go test -v -race . || {
        echo "Tests failed or timed out after 10 seconds"
        exit 1
    }
fi

echo ""
echo "================================================"
echo "All reliability tests passed!"
echo "================================================"