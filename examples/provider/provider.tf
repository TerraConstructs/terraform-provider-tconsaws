# Copyright (c) TerraConstructs.
# SPDX-License-Identifier: MPL-2.0

provider "tconsaws" {
  region = "us-east-1"

  # Optional: specify profile for credentials
  # profile = "default"

  # Optional: specify custom endpoint for testing
  # endpoints {
  #   sqs = "http://localhost:9324"
  # }
}
