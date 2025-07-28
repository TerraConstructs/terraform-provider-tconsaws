#!/bin/bash

# End-to-end integration test for terraform-provider-tconsaws
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== terraform-provider-tconsaws Integration Test ==="
echo ""

# Step 1: Start Docker Compose environment
echo "1. Starting Docker Compose test environment..."
cd "$PROJECT_DIR"
docker compose -f docker-compose.test.yml up -d

# Step 2: Wait for services
echo "2. Waiting for services to be ready..."
source "$SCRIPT_DIR/test-setup.sh" > /dev/null 2>&1

# Step 3: Set environment variables
echo "3. Setting up environment variables..."
export AWS_ENDPOINT_URL_SQS="http://localhost:9324"
export AWS_EC2_METADATA_SERVICE_ENDPOINT="http://localhost:1338"
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"

# Step 4: Install/update local provider
echo "4. Installing local provider..."
go install .

# Step 5: Test Terraform configuration
echo "5. Testing Terraform configuration..."
cd "$PROJECT_DIR/examples/basic-signal"

# Initialize Terraform
terraform init

# Run terraform plan to see what would be created
echo "Running terraform plan..."
terraform plan

echo ""
echo "=== Test Ready ==="
echo ""
echo "The test environment is running. To complete the integration test:"
echo ""
echo "1. In one terminal, run: terraform apply"
echo "   (This will start waiting for signals)"
echo ""
echo "2. In another terminal, send a signal using the tcsignal-aws binary:"
echo "   export AWS_ENDPOINT_URL_SQS=http://localhost:9324"
echo "   export AWS_REGION=us-east-1"
echo "   export AWS_ACCESS_KEY_ID=test"
echo "   export AWS_SECRET_ACCESS_KEY=test"
echo ""
echo "   ./bin/tcsignal-aws \\"
echo "     --queue-url http://localhost:9324/000000000000/signals \\"
echo "     --id test-deployment-123 \\"
echo "     --instance-id i-test123 \\"
echo "     --status SUCCESS"
echo ""
echo "3. The terraform apply should complete successfully after receiving the signal"
echo ""
echo "To cleanup:"
echo "terraform destroy"
echo "cd $PROJECT_DIR && docker compose -f docker-compose.test.yml down"