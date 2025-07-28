terraform {
  required_providers {
    tconsaws = {
      source = "registry.terraform.io/terraconstructs/tconsaws"
    }
    aws = {
      source = "hashicorp/aws"
    }
  }
}

# Create an SQS queue for signals
resource "aws_sqs_queue" "signals" {
  name = "deployment-signals"
}

# Example instances that will send signals
resource "aws_instance" "web" {
  count           = 3
  ami             = "ami-0c02fb55956c7d316" # Amazon Linux 2023
  instance_type   = "t3.micro"
  
  user_data = <<-EOD
    #!/bin/bash
    yum update -y
    yum install -y httpd
    systemctl start httpd
    systemctl enable httpd
    
    # Download and install tcsignal-aws binary
    curl -L -o /usr/local/bin/tcsignal-aws.tgz "https://github.com/TerraConstructs/signal-aws/releases/download/v1.0.0/signal-aws_Linux_x86_64.tar.gz"
    tar -xzf /usr/local/bin/tcsignal-aws.tgz -C /usr/local/bin
    rm /usr/local/bin/tcsignal-aws.tgz
    
    # Signal success when ready
    /usr/local/bin/tcsignal-aws \
      --queue-url "${aws_sqs_queue.signals.url}" \
      --id "deployment-${random_id.signal.hex}" \
      --status SUCCESS
  EOD

  tags = {
    Name = "web-${count.index + 1}"
  }
}

# Random ID for unique signal identification
resource "random_id" "signal" {
  byte_length = 8
}

# Wait for all instances to signal readiness
resource "tconsaws_signal" "web_ready" {
  queue_url      = aws_sqs_queue.signals.url
  signal_id      = "deployment-${random_id.signal.hex}"
  expected_count = length(aws_instance.web)
  retries        = 3
  
  timeouts {
    create = "10m"
  }
  
  triggers = {
    instance_ids = join(",", aws_instance.web[*].id)
  }
  
  depends_on = [aws_instance.web]
}

# Resources that depend on instances being ready
resource "aws_eip" "web" {
  count  = length(aws_instance.web)
  domain = "vpc"
  
  tags = {
    Name = "web-eip-${count.index + 1}"
  }
}

resource "aws_eip_association" "web" {
  count         = length(aws_instance.web)
  instance_id   = aws_instance.web[count.index].id
  allocation_id = aws_eip.web[count.index].id
  
  # Only attach EIPs after instances are ready
  depends_on = [tconsaws_signal.web_ready]
}