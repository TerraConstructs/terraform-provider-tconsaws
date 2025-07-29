# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`terraform-provider-tconsaws` is a Terraform provider that brings CloudFormation cfn-signal equivalent functionality to Terraform using AWS SQS. It provides a single resource `tconsaws_signal` that waits for EC2 instances to signal readiness via SQS before proceeding with downstream resources.

## Core Architecture

### Provider Structure
- **Entry Point**: `main.go` - standard Terraform provider server setup
- **Provider Config**: `internal/provider/provider.go` - AWS authentication and client setup using Plugin Framework v6.0
- **Signal Resource**: `internal/provider/signal_resource.go` - core resource that polls SQS for instance readiness signals
- **Client Binary**: `bin/tcsignal-aws` - companion binary that instances use to send signals

### Key Dependencies
- Go 1.23.7+
- Terraform Plugin Framework (modern approach, not legacy SDK)
- AWS SDK v2 for SQS and IMDS operations
- Uses long-polling SQS for efficient signal collection

## Development Commands

### Essential Build Commands
```bash
# Default target: format, lint, install, generate docs
make

# Individual commands
make build                # Build provider binary
make install              # Install to GOBIN for local testing
make install-signal-binary # Download tcsignal-aws binary for testing
make lint                 # Run golangci-lint
make test                 # Unit tests
make testacc              # Automated acceptance tests with full environment setup
make testacc-setup        # Start test environment (ElasticMQ)
make testacc-clean        # Clean test queues between runs
make testacc-teardown     # Stop test environment
make generate             # Generate documentation
```

### Local Development Setup
```bash
# 1. Install provider locally
go install .

# 2. Create dev override in ~/.terraformrc
provider_installation {
  dev_overrides {
    "registry.terraform.io/terraconstructs/tconsaws" = "/path/to/your/GOBIN"
  }
  direct {}
}

# 3. Start test environment
make testacc-setup
```

### Testing

The project uses automated test environment management with Docker Compose:

- **Automated Acceptance Tests**: `make testacc` - complete test cycle with setup, test execution, and cleanup
- **Manual Test Commands**:
  - `make testacc-setup` - Start test environment (ElasticMQ + EC2 metadata mock)
  - `make testacc-clean` - Clean test queues between test runs
  - `make testacc-teardown` - Stop test environment completely
- **Test Environment**: Uses ElasticMQ on port 9324 and EC2 metadata mock on port 1338
- **Test Architecture**: Test automation code isolated in `tools/` directory with separate go.mod to prevent binary contamination

## Signal Resource Architecture

The `tconsaws_signal` resource implements this workflow:
1. **Create**: Long-polls SQS queue filtering by `signal_id` attribute
2. **Signal Processing**: Counts unique `instance_id` values until `expected_count` reached
3. **Failure Handling**: Immediately fails if any FAILURE signal received
4. **Timeout**: Respects both per-operation and overall timeouts
5. **Update**: Any schema change is ForceNew (triggers fresh wait cycle)

### Key Attributes
- `queue_url` (required) - SQS queue URL for receiving signals
- `signal_id` (required) - Unique deployment identifier for filtering messages
- `expected_count` (required) - Number of success signals needed
- `retries` (default: 3) - SQS operation retry count
- `publish_timeout` (default: "10s") - Per SQS operation timeout
- `triggers` - Map of values that trigger resource recreation
- `timeouts.create` - Overall resource timeout (default: Terraform's 20m)

## Message Protocol

Instances send SQS messages with:
- **Message Attributes**: `signal_id`, `instance_id` 
- **Message Body**: JSON with `status` ("SUCCESS" or "FAILURE") and optional `reason`
- **Instance ID**: Retrieved from IMDS or overridden via `--instance-id` flag

## Configuration Compatibility

Provider configuration mirrors the official AWS provider for seamless integration:
- Same authentication methods (access keys, profiles, IAM roles)
- Same configuration attributes (`region`, `profile`, `assume_role`, etc.)
- Uses AWS SDK v2 default credential chain

## Testing Strategy

The project uses automated Docker Compose management for acceptance testing:

### Test Environment Components
- **ElasticMQ**: SQS API-compatible message queue (localhost:9324)
- **EC2 Metadata Mock**: Simulates EC2 instance metadata service (localhost:1338)
- **Queue Management**: Automated queue creation and cleanup between tests
- **Health Checks**: Automated service readiness verification

### Test Queue Configuration
Standardized queue names used across tests:
- `test-single` - Single instance signal tests
- `test-multiple` - Multiple instance signal tests
- `test-timeout` - Timeout behavior tests
- `test-filter` - Signal filtering tests

### Test Automation Architecture
- **Location**: `tools/testenv/` directory with separate go.mod
- **Binary Isolation**: Test automation code excluded from published provider binary
- **Environment Management**: `tools/testenv/environment.go` provides setup/teardown functions
- **CLI Tools**: Command-line utilities in `tools/testenv/setup/` and `tools/testenv/cleanup/`
- **Make Integration**: Automated test targets handle complete test lifecycle

### Test Workflow
1. **Setup**: Start Docker Compose services and wait for readiness
2. **Execution**: Run acceptance tests with proper environment variables
3. **Cleanup**: Purge test queues and optionally teardown services