# Testing terraform-provider-tconsaws

This directory contains test setup and integration tests for the `terraform-provider-tconsaws` provider.

## Prerequisites

- Docker and Docker Compose
- Go 1.23+
- Terraform
- AWS CLI (optional, for manual testing)

## Quick Start

1. **Setup local provider development:**
   ```bash
   # This creates ~/.terraformrc with dev_overrides
   go install .
   ```

2. **Start test environment:**
   ```bash
   ./test/integration-test.sh
   ```
   This will start the Docker Compose stack and prepare everything for testing.

3. **Run the integration test:**
   - Follow the instructions printed by the integration test script
   - Use terraform apply to start waiting for signals
   - Send signals using the provided AWS CLI command or tcsignal-aws binary

## Test Environment

The Docker Compose stack includes:

- **ElasticMQ** (SQS-compatible): `http://localhost:9324`
  - Pre-configured queue: `http://localhost:9324/000000000000/signals`
- **EC2 Metadata Mock**: `http://localhost:1338`

## Manual Testing

### Environment Variables
```bash
export AWS_ENDPOINT_URL_SQS=http://localhost:9324
export AWS_EC2_METADATA_SERVICE_ENDPOINT=http://localhost:1338
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
```

### Send Signal via tcsignal-aws binary
```bash
./bin/tcsignal-aws \
  --queue-url http://localhost:9324/000000000000/signals \
  --id test-deployment-123 \
  --instance-id i-test123 \
  --status SUCCESS
```

The `tcsignal-aws` binary supports the following options (as per PRD):
- `-u, --queue-url string`: (required) SQS queue URL
- `-i, --id string`: (required) unique signal ID for the deployment  
- `-s, --status string`: shortcut to send "SUCCESS" or "FAILURE" without exec
- `-n, --instance-id string`: override instance ID (default: fetch from IMDS)
- `-e, --exec string`: run command and signal based on its exit code

## Running Acceptance Tests

```bash
# Set environment variables
source <(./test/test-setup.sh | grep '^export')

# Run acceptance tests
export TF_ACC=1
go test -v ./internal/provider/ -run TestAcc
```

## Cleanup

```bash
docker-compose -f docker-compose.test.yml down
```