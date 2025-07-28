terraform {
  required_providers {
    tconsaws = {
      source = "registry.terraform.io/terraconstructs/tconsaws"
    }
  }
}

provider "tconsaws" {
  region = "us-east-1"
}

resource "tconsaws_signal" "example" {
  # uses ElasticMQ for local testing
  queue_url      = "http://localhost:9324/000000000000/signals"
  signal_id      = "test-deployment-123"
  expected_count = 1

  timeouts {
    create = "2m"
  }
}
