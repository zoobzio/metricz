#!/bin/bash

# Traffic generation script for API Gateway example
# Simulates realistic traffic patterns

echo "Starting traffic generation..."
echo "API Gateway should be running on :8080"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if gateway is running
if ! curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health | grep -q "200"; then
    echo -e "${RED}Error: API Gateway not responding on :8080${NC}"
    echo "Please start the gateway with: go run ."
    exit 1
fi

echo -e "${GREEN}Gateway is healthy!${NC}"
echo ""

# Function to make requests
make_request() {
    local endpoint=$1
    local count=$2
    local delay=$3
    
    echo "Sending $count requests to $endpoint (${delay}s delay)..."
    
    for i in $(seq 1 $count); do
        response=$(curl -s -w "\n%{http_code} %{time_total}s" http://localhost:8080$endpoint 2>/dev/null)
        status=$(echo "$response" | tail -n1 | cut -d' ' -f1)
        time=$(echo "$response" | tail -n1 | cut -d' ' -f2)
        
        if [ "$status" = "200" ]; then
            echo -e "  [$i] ${GREEN}✓${NC} $endpoint - ${time}"
        elif [ "$status" = "404" ]; then
            echo -e "  [$i] ${YELLOW}⚠${NC} $endpoint - 404 Not Found"
        else
            echo -e "  [$i] ${RED}✗${NC} $endpoint - Error $status"
        fi
        
        sleep $delay
    done
    echo ""
}

# Phase 1: Light traffic
echo -e "${YELLOW}Phase 1: Light Traffic${NC}"
echo "------------------------"
make_request "/api/users" 5 0.5
make_request "/api/orders" 3 0.5
make_request "/api/inventory" 4 0.5

# Phase 2: Mixed traffic
echo -e "${YELLOW}Phase 2: Mixed Traffic${NC}"
echo "----------------------"
(make_request "/api/users" 10 0.2) &
(make_request "/api/orders" 10 0.3) &
(make_request "/api/inventory" 10 0.25) &
wait

# Phase 3: Burst traffic
echo -e "${YELLOW}Phase 3: Burst Traffic${NC}"
echo "----------------------"
echo "Sending 50 concurrent requests..."

for i in $(seq 1 50); do
    endpoints=("/api/users" "/api/orders" "/api/inventory")
    endpoint=${endpoints[$((RANDOM % 3))]}
    
    (
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080$endpoint 2>/dev/null)
        if [ "$response" = "200" ]; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗${NC}"
        fi
    ) &
done
wait
echo ""

# Phase 4: Sustained load
echo -e "${YELLOW}Phase 4: Sustained Load (30 seconds)${NC}"
echo "------------------------------------"
echo "Generating continuous traffic..."

end=$(($(date +%s) + 30))
request_count=0

while [ $(date +%s) -lt $end ]; do
    endpoints=("/api/users" "/api/orders" "/api/inventory")
    endpoint=${endpoints[$((RANDOM % 3))]}
    
    curl -s -o /dev/null http://localhost:8080$endpoint 2>/dev/null &
    request_count=$((request_count + 1))
    
    # Show progress every 10 requests
    if [ $((request_count % 10)) -eq 0 ]; then
        echo -ne "\rRequests sent: $request_count"
    fi
    
    # Small delay to prevent overwhelming
    sleep 0.05
done
wait
echo -e "\rRequests sent: $request_count"
echo ""

# Show metrics
echo -e "${YELLOW}Metrics Summary${NC}"
echo "---------------"
echo "Fetching metrics from /metrics endpoint..."
echo ""

# Get key metrics
counters=$(curl -s http://localhost:8080/metrics | grep -E "^requests_" | grep -v "TYPE")
errors=$(curl -s http://localhost:8080/metrics | grep -E "^errors_" | grep -v "TYPE")

echo "Request Counts:"
echo "$counters" | while read line; do
    metric=$(echo $line | cut -d' ' -f1)
    value=$(echo $line | cut -d' ' -f2)
    echo "  $metric: $value"
done
echo ""

if [ ! -z "$errors" ]; then
    echo "Error Counts:"
    echo "$errors" | while read line; do
        metric=$(echo $line | cut -d' ' -f1)
        value=$(echo $line | cut -d' ' -f2)
        echo -e "  ${RED}$metric: $value${NC}"
    done
    echo ""
fi

# Check health
echo "Health Status:"
health=$(curl -s http://localhost:8080/health)
if echo "$health" | grep -q "OK"; then
    echo -e "  ${GREEN}$health${NC}"
else
    echo -e "  ${RED}$health${NC}"
fi

echo ""
echo "Traffic generation complete!"
echo "View full metrics at: http://localhost:8080/metrics"