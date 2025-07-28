#!/bin/bash

# Test setup for terraform-provider-tconsaws acceptance testing
set -e

echo "Starting test environment..."

# Start Docker Compose
docker compose -f docker-compose.test.yml up -d

# Wait for services to be healthy (macOS compatible)
echo "Waiting for services to be ready..."

# Function to wait for service with timeout
wait_for_service() {
    local url=$1
    local timeout=60
    local count=0
    
    while [ $count -lt $timeout ]; do
        if curl -f "$url" > /dev/null 2>&1; then
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    
    echo "Timeout waiting for $url"
    return 1
}

# Wait for ElasticMQ (SQS)
echo "Waiting for ElasticMQ (SQS) to be ready..."
wait_for_service "http://localhost:9324/"

# Wait for EC2 metadata mock
echo "Waiting for EC2 metadata mock to be ready..."
wait_for_service "http://localhost:1338/latest/meta-data/instance-id"

echo "Test environment is ready!"
echo ""
echo "Environment variables for testing:"
echo "export AWS_ENDPOINT_URL_SQS=http://localhost:9324"
echo "export AWS_EC2_METADATA_SERVICE_ENDPOINT=http://localhost:1338"
echo "export AWS_REGION=us-east-1"
echo "export AWS_ACCESS_KEY_ID=test"
echo "export AWS_SECRET_ACCESS_KEY=test"
echo "export TF_ACC=1"
echo ""
echo "To set environment variables:"
echo "source <(./test/test-setup.sh | grep '^export')"
echo ""
echo "To run acceptance tests:"
echo "go test -v ./internal/provider/ -run TestAcc"
echo ""
echo "To stop the test environment:"
echo "docker compose -f docker-compose.test.yml down"