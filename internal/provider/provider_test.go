// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"tconsaws": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// Check that required environment variables for local testing are set
	// This ensures the Docker Compose test environment is running
	if os.Getenv("AWS_ENDPOINT_URL_SQS") == "" {
		t.Fatal("AWS_ENDPOINT_URL_SQS must be set for acceptance tests")
	}
	if os.Getenv("AWS_EC2_METADATA_SERVICE_ENDPOINT") == "" {
		t.Fatal("AWS_EC2_METADATA_SERVICE_ENDPOINT must be set for acceptance tests")
	}
}
