#!/bin/bash

# Setup SQS queues for testing
set -e

# Set AWS CLI environment for local testing
export AWS_ENDPOINT_URL_SQS="http://localhost:9324"
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"

echo "Creating SQS queues for testing..."

# Create test queues
aws sqs create-queue --queue-name signals --region us-east-1 --endpoint-url http://localhost:9324 > /dev/null 2>&1 || echo "Queue 'signals' already exists"
aws sqs create-queue --queue-name signals-multi --region us-east-1 --endpoint-url http://localhost:9324 > /dev/null 2>&1 || echo "Queue 'signals-multi' already exists"
aws sqs create-queue --queue-name signals-timeout --region us-east-1 --endpoint-url http://localhost:9324 > /dev/null 2>&1 || echo "Queue 'signals-timeout' already exists"

echo "SQS queues created successfully!"
echo ""
echo "Available queues:"
echo "- http://localhost:9324/000000000000/signals"
echo "- http://localhost:9324/000000000000/signals-multi"
echo "- http://localhost:9324/000000000000/signals-timeout"