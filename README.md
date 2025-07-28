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

```hcl
terraform {
  required_providers {
    tconsaws = {
      source  = "registry.terraform.io/terraconstructs/tconsaws"
      version = "~> 1.0"
    }
  }
}

provider "tconsaws" {
  region = "us-east-1"
}

resource "aws_sqs_queue" "signals" {
  name = "deployment-signals"
}

resource "aws_instance" "web" {
  count = 3
  # ... instance configuration ...
  
  user_data = <<-EOD
    #!/bin/bash
    # Install and configure your application
    
    # Signal success when ready
    /usr/local/bin/tcsignal-aws \
      --queue-url "${aws_sqs_queue.signals.url}" \
      --id "deployment-${random_id.signal.hex}" \
      --status SUCCESS
  EOD
}

resource "tconsaws_signal" "web_ready" {
  queue_url      = aws_sqs_queue.signals.url
  signal_id      = "deployment-${random_id.signal.hex}"
  expected_count = length(aws_instance.web)
  
  timeouts {
    create = "10m"
  }
  
  depends_on = [aws_instance.web]
}

# Resources that depend on instances being ready
resource "aws_eip_association" "web" {
  count         = length(aws_instance.web)
  instance_id   = aws_instance.web[count.index].id
  allocation_id = aws_eip.web[count.index].id
  
  depends_on = [tconsaws_signal.web_ready]
}
```

## Resource: `tconsaws_signal`

Waits for a specified number of success signals from compute resources.

### Arguments

- `queue_url` (Required) - SQS queue URL where signals will be sent
- `signal_id` (Required) - Unique identifier for this deployment
- `expected_count` (Required) - Number of success signals required
- `retries` (Optional) - Number of retries for transient SQS errors (default: 3)
- `publish_timeout` (Optional) - Timeout for each SQS operation (default: "10s")
- `triggers` (Optional) - Map of values that trigger resource recreation
- `timeouts` (Optional) - Resource timeout configuration

### Attributes

- `success_count` - Number of success signals received
- `failure_received` - Whether any failure signal was received
- `instance_ids` - List of unique instance IDs that sent signals

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

## Provider Configuration

The provider uses the same configuration options as the AWS provider:

```hcl
provider "tconsaws" {
  region                   = "us-east-1"
  access_key              = "your-access-key"
  secret_key              = "your-secret-key" 
  profile                 = "your-aws-profile"
  shared_credentials_files = ["~/.aws/credentials"]
  
  # Override endpoints for testing
  # endpoints {
  #   sqs = "http://localhost:9324" 
  # }
}
```

## Development

### Building the Provider

```shell
go install .
```

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
go install .
```

### Testing

Start the test environment:
```shell
./test/integration-test.sh
```

Run acceptance tests:
```shell
export TF_ACC=1
go test -v ./internal/provider/ -run TestAcc
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MPL-2.0 License - see the LICENSE file for details.