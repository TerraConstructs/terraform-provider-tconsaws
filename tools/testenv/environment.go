// Package testenv provides automated test environment management for acceptance tests
package testenv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// TestQueues defines the queue names used in acceptance tests
var TestQueues = []string{
	"test-single",
	"test-multiple",
	"test-timeout",
	"test-filter",
}

// SetupTestEnvironment starts the Docker Compose stack and waits for services to be ready
func SetupTestEnvironment() error {
	fmt.Println("🏗️  Setting up acceptance test environment...")

	// Start docker compose
	fmt.Println("Starting ElasticMQ...")
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.test.yml", "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker compose: %v", err)
	}

	// Wait for ElasticMQ to be ready
	fmt.Println("Waiting for ElasticMQ to be ready...")
	if !waitForService("http://localhost:9324/", 30*time.Second, isElasticMQHealthy) {
		return fmt.Errorf("ElasticMQ failed to start within 30 seconds")
	}
	fmt.Println("✅ ElasticMQ is ready!")

	fmt.Println("🎉 Test environment is ready!")
	return nil
}

// TeardownTestEnvironment stops the Docker Compose stack
func TeardownTestEnvironment() error {
	fmt.Println("🧹 Tearing down test environment...")

	cmd := exec.Command("docker", "compose", "-f", "docker-compose.test.yml", "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop docker compose: %v", err)
	}

	fmt.Println("✅ Test environment stopped!")
	return nil
}

// PurgeTestQueues clears all messages from test queues
func PurgeTestQueues() error {
	fmt.Println("🧽 Purging test queues...")

	// Configure AWS SDK for local ElasticMQ
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test",
			"test",
			"",
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create SQS client with local endpoint
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String("http://localhost:9324")
	})

	// Purge each test queue
	for _, queueName := range TestQueues {
		queueURL := fmt.Sprintf("http://localhost:9324/000000000000/%s", queueName)

		_, err := client.PurgeQueue(context.TODO(), &sqs.PurgeQueueInput{
			QueueUrl: aws.String(queueURL),
		})
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to purge queue %s: %v\n", queueName, err)
			// Continue with other queues
		} else {
			fmt.Printf("✅ Purged queue: %s\n", queueName)
		}
	}

	fmt.Println("🎉 Test queues purged!")
	return nil
}

// waitForService waits for a service to be healthy with timeout
func waitForService(url string, timeout time.Duration, healthCheck func(string) bool) bool {
	deadline := time.Now().Add(timeout)
	attempt := 1

	for time.Now().Before(deadline) {
		if healthCheck(url) {
			return true
		}

		fmt.Printf("   Waiting... (attempt %d)\n", attempt)
		attempt++
		time.Sleep(1 * time.Second)
	}

	return false
}

// isElasticMQHealthy checks if ElasticMQ service is healthy
func isElasticMQHealthy(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// ElasticMQ returns 400 for root path, which is expected
	return resp.StatusCode == 400
}
