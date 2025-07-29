# Testing terraform-provider-tconsaws

This directory contains test configuration files for the `terraform-provider-tconsaws` provider.

## Files

- `elasticmq.conf` - ElasticMQ configuration with pre-defined test queues

## Running Tests

Use the automated make targets from the project root:

```bash
# Run full acceptance test cycle (setup -> test -> cleanup)
make testacc

# Individual commands for manual testing:
make testacc-setup     # Start test environment
make testacc-clean     # Clean test queues  
make testacc-teardown  # Stop test environment
```

## Test Environment

The automated system starts:
- **ElasticMQ** (SQS-compatible): `http://localhost:9324`
- Pre-configured test queues: `test-single`, `test-multiple`, `test-timeout`, `test-filter`

## Manual Signal Testing

If you need to send signals manually during development:

```bash
# With test environment running (make testacc-setup)
export AWS_ENDPOINT_URL_SQS=http://localhost:9324
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

# Send a test signal (requires tcsignal-aws binary)
./bin/tcsignal-aws \
  --queue-url http://localhost:9324/000000000000/test-single \
  --id signal-single-001 \
  --instance-id i-test123 \
  --status SUCCESS
```

## Test Automation

Test automation is implemented in Go (`tools/testenv/`) and provides:
- Docker Compose lifecycle management
- Service health checks
- Queue purging between tests
- AWS SDK integration for queue management