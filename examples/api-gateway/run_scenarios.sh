#!/bin/bash

# Run API Gateway Scenarios
# Demonstrates how metrics help during production incidents

echo "=== API Gateway Scenario Runner ==="
echo ""
echo "Choose a scenario:"
echo "1) Normal operation (shows static metrics)"
echo "2) Black Friday meltdown (shows incident response)"
echo ""
echo -n "Enter choice [1-2]: "
read choice

case $choice in
    1)
        echo "Running normal operation..."
        go run . -scenario normal
        ;;
    2)
        echo "Running Black Friday meltdown scenario..."
        echo "Watch how metrics reveal the problem and solution!"
        echo ""
        go run . -scenario blackfriday
        ;;
    *)
        echo "Invalid choice. Running normal operation..."
        go run . -scenario normal
        ;;
esac