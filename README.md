# terraform-provider-tconsaws

A Terraform provider that brings CloudFormation cfn-signal equivalent functionality to Terraform using AWS SQS.

This provider enables you to wait for EC2 instances or other compute resources to send success/failure signals during Terraform deployments, ensuring that downstream resources aren't created until instances have fully initialized.

## Features

- **Signal-based coordination**: Wait for compute resources to signal readiness via SQS
- **AWS-compatible configuration**: Uses the same credential chain as the AWS provider
- **Configurable timeouts**: Support for custom timeout configurations  
- **Instance deduplication**: Handles multiple signals from the same instance correctly
- **Failure handling**: Immediate failure on any failure signal received

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.23 (for development)
- AWS credentials configured (same as AWS provider)

## Quick Start

Refer to (Terraform Registry docs)[https://registry.terraform.io/providers/terraconstructs/tconsaws]

## Sending Signals

Use the [tcsignal-aws](https://github.com/TerraConstructs/signal-aws) binary on your compute resources:

```bash
# Send success signal
tcsignal-aws \
  --queue-url "https://sqs.us-east-1.amazonaws.com/123456789/signals" \
  --id "deployment-abc123" \
  --status SUCCESS

# Send failure signal  
tcsignal-aws \
  --queue-url "https://sqs.us-east-1.amazonaws.com/123456789/signals" \
  --id "deployment-abc123" \
  --status FAILURE

# Execute command and signal based on exit code
tcsignal-aws \
  --queue-url "https://sqs.us-east-1.amazonaws.com/123456789/signals" \
  --id "deployment-abc123" \
  --exec "./install-app.sh"
```

## Development

### Building the Provider

```shell
go install .
```

### Installing Signal Binary

For testing and development, you'll need the [tcsignal-aws](https://github.com/TerraConstructs/signal-aws) binary:

```shell
make install-signal-binary
```

This downloads the appropriate binary for your platform to `./bin/tcsignal-aws`.

### Local Development

1. Create `.terraformrc` in your home directory:
```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/terraconstructs/tconsaws" = "/path/to/your/GOBIN"
  }
  direct {}
}
```

2. Build and install:
```shell
make install
```

### Testing

Run unit tests:
```shell
make test
```

Run acceptance tests (fully automated):
```shell
make testacc
```

Available test commands:
- `make testacc-setup` - Start test environment (ElasticMQ + EC2 metadata mock)
- `make testacc-clean` - Clean test queues 
- `make testacc-teardown` - Stop test environment
- `make testacc` - Complete test cycle: setup → test → cleanup

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MPL-2.0 License - see the LICENSE file for details.
