// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSignalResource_Single(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/signals"
	signalID := "test-deployment-123"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSignalResourceConfig_single(queueURL, signalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the signal resource was created successfully
					resource.TestCheckResourceAttr("tconsaws_signal.test", "queue_url", queueURL),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "signal_id", signalID),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "expected_count", "1"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "success_count", "1"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "failure_received", "false"),
					resource.TestCheckResourceAttrSet("tconsaws_signal.test", "id"),
				),
			},
		},
	})
}

func TestAccSignalResource_Multiple(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/signals-multi"
	signalID := "test-deployment-multi-456"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSignalResourceConfig_multiple(queueURL, signalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Send 3 signals using tcsignal-aws before checking state
					testAccSendMultipleSignals(queueURL, signalID, 3),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "expected_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "success_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "failure_received", "false"),
				),
			},
		},
	})
}

func TestAccSignalResource_Timeout(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/signals-timeout"
	signalID := "test-deployment-timeout-789"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSignalResourceConfig_timeout(queueURL, signalID),
				ExpectError: regexp.MustCompile("timeout waiting for signals"),
			},
		},
	})
}

// testAccSendSignal sends a signal using the tcsignal-aws binary
func testAccSendSignal(queueURL, signalID, instanceID, status string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Use tcsignal-aws binary to send signal
		cmd := exec.Command("./bin/tcsignal-aws",
			"--queue-url", queueURL,
			"--id", signalID,
			"--instance-id", instanceID,
			"--status", status,
		)

		// Set environment variables for local testing
		cmd.Env = append(os.Environ(),
			"AWS_ENDPOINT_URL_SQS=http://localhost:9324",
			"AWS_EC2_METADATA_SERVICE_ENDPOINT=http://localhost:1338",
			"AWS_REGION=us-east-1",
			"AWS_ACCESS_KEY_ID=test",
			"AWS_SECRET_ACCESS_KEY=test",
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to send signal: %v, output: %s", err, output)
		}

		return nil
	}
}

// testAccSendMultipleSignals sends multiple signals using tcsignal-aws
func testAccSendMultipleSignals(queueURL, signalID string, count int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for i := 0; i < count; i++ {
			instanceID := fmt.Sprintf("i-test%d", i)
			if err := testAccSendSignal(queueURL, signalID, instanceID, "SUCCESS")(s); err != nil {
				return err
			}
		}
		return nil
	}
}

// testAccCreateQueue creates an SQS queue for testing
func testAccCreateQueue(queueName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion("us-east-1"),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					if service == sqs.ServiceID {
						return aws.Endpoint{
							URL: "http://localhost:9324",
						}, nil
					}
					return aws.Endpoint{}, fmt.Errorf("unknown service: %s", service)
				},
			)),
		)
		if err != nil {
			return err
		}

		client := sqs.NewFromConfig(cfg)
		_, err = client.CreateQueue(context.Background(), &sqs.CreateQueueInput{
			QueueName: aws.String(queueName),
		})
		if err != nil {
			return fmt.Errorf("failed to create queue %s: %v", queueName, err)
		}

		return nil
	}
}

// Configuration templates

func testAccSignalResourceConfig_single(queueURL, signalID string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    tconsaws = {
      source = "registry.terraform.io/tcons/tconsaws"
    }
  }
}

provider "tconsaws" {
  region = "us-east-1"
}

resource "tconsaws_signal" "test" {
  queue_url      = %[1]q
  signal_id      = %[2]q
  expected_count = 1
  
  timeouts {
    create = "2m"
  }
}
`, queueURL, signalID)
}

func testAccSignalResourceConfig_multiple(queueURL, signalID string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    tconsaws = {
      source = "registry.terraform.io/tcons/tconsaws"
    }
  }
}

provider "tconsaws" {
  region = "us-east-1"
}

resource "tconsaws_signal" "test" {
  queue_url      = %[1]q
  signal_id      = %[2]q
  expected_count = 3
  
  timeouts {
    create = "3m"
  }
}
`, queueURL, signalID)
}

func testAccSignalResourceConfig_timeout(queueURL, signalID string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    tconsaws = {
      source = "registry.terraform.io/tcons/tconsaws"
    }
  }
}

provider "tconsaws" {
  region = "us-east-1"
}

resource "tconsaws_signal" "test" {
  queue_url      = %[1]q
  signal_id      = %[2]q
  expected_count = 1
  
  timeouts {
    create = "10s"  # Short timeout to test timeout behavior
  }
}
`, queueURL, signalID)
}
