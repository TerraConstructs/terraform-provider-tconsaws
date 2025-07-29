// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	providerConfig = `provider "tconsaws" {
  region = "us-east-1"
  endpoints {
    sqs = "http://localhost:9324"
  }
}
`
)

func TestAccSignalResource_Single(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/test-single"
	signalID := "signal-single-001"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			// Send signal before resource creation begins
			sendSignalDirectly(t, queueURL, signalID, "i-test001", "SUCCESS")
		},
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
	queueURL := "http://localhost:9324/000000000000/test-multiple"
	signalID := "signal-multiple-001"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			// Send 3 signals before resource creation begins
			sendMultipleSignalsDirectly(t, queueURL, signalID, 3)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSignalResourceConfig_multiple(queueURL, signalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tconsaws_signal.test", "expected_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "success_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "failure_received", "false"),
				),
			},
		},
	})
}

func TestAccSignalResource_Timeout(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/test-timeout"
	signalID := "signal-timeout-001"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSignalResourceConfig_timeout(queueURL, signalID),
				ExpectError: regexp.MustCompile("Error while waiting for signals: timeout waiting for signals"),
			},
		},
	})
}

func TestAccSignalResource_Filter(t *testing.T) {
	queueURL := "http://localhost:9324/000000000000/test-filter"
	signalID1 := "signal-filter-001"
	signalID2 := "signal-filter-002"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			// Send 3 signals before resource creation begins
			sendMultipleSignalsDirectly(t, queueURL, signalID1, 3)
			sendMultipleSignalsDirectly(t, queueURL, signalID2, 2)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSignalResourceConfig_multiple(queueURL, signalID1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tconsaws_signal.test", "expected_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "success_count", "3"),
					resource.TestCheckResourceAttr("tconsaws_signal.test", "failure_received", "false"),
				),
			},
		},
	})
}

// sendSignalDirectly sends a signal using the tcsignal-aws binary directly (not as a TestCheckFunc).
func sendSignalDirectly(t *testing.T, queueURL, signalID, instanceID, status string) {
	t.Logf("Sending signal directly - queueURL=%s, signalID=%s, instanceID=%s, status=%s",
		queueURL, signalID, instanceID, status)

	cmd := exec.Command("../../bin/tcsignal-aws",
		"--queue-url", queueURL,
		"--id", signalID,
		"--instance-id", instanceID,
		"--status", status,
	)

	// required for the tcsignal-aws binary to run correctly
	cmd.Env = append(os.Environ(),
		"AWS_ENDPOINT_URL_SQS=http://localhost:9324",
		"AWS_REGION=us-east-1",
		"AWS_ACCESS_KEY_ID=test",
		"AWS_SECRET_ACCESS_KEY=test",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to send signal: %v, output: %s", err, output)
	}
	t.Logf("Signal sent successfully - output: %s", output)
}

// sendMultipleSignalsDirectly sends multiple signals using tcsignal-aws directly (not as TestCheckFunc).
func sendMultipleSignalsDirectly(t *testing.T, queueURL, signalID string, count int) {
	for i := range count {
		instanceID := fmt.Sprintf("i-test%d", i)
		sendSignalDirectly(t, queueURL, signalID, instanceID, "SUCCESS")
	}
}

// Configuration templates

func testAccSignalResourceConfig_single(queueURL, signalID string) string {
	return fmt.Sprintf(`
%s
resource "tconsaws_signal" "test" {
  queue_url      = %[2]q
  signal_id      = %[3]q
  expected_count = 1
  
  timeouts {
    create = "2m"
  }
}
`, providerConfig, queueURL, signalID)
}

func testAccSignalResourceConfig_multiple(queueURL, signalID string) string {
	return fmt.Sprintf(`
%s
resource "tconsaws_signal" "test" {
  queue_url      = %[2]q
  signal_id      = %[3]q
  expected_count = 3
  
  timeouts {
    create = "3m"
  }
}
`, providerConfig, queueURL, signalID)
}

func testAccSignalResourceConfig_timeout(queueURL, signalID string) string {
	return fmt.Sprintf(`
%s
resource "tconsaws_signal" "test" {
  queue_url      = %[2]q
  signal_id      = %[3]q
  expected_count = 1
  
  timeouts {
    create = "10s"  # Short timeout to test timeout behavior
  }
}
`, providerConfig, queueURL, signalID)
}
