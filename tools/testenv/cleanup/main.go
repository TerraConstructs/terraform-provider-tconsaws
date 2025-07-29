package main

import (
	"fmt"
	"log"
	"os"

	"tools/testenv"
)

func main() {
	// First purge queues to clean up any test data
	if err := testenv.PurgeTestQueues(); err != nil {
		log.Printf("⚠️  Warning: Failed to purge test queues: %v", err)
		// Continue with teardown even if purge failed
	}

	// Then teardown the environment (optional - useful for CI)
	if len(os.Args) > 1 && os.Args[1] == "--teardown" {
		if err := testenv.TeardownTestEnvironment(); err != nil {
			log.Printf("❌ Failed to teardown test environment: %v", err)
			os.Exit(1)
		}
	}

	fmt.Println("🎉 Test environment cleanup complete!")
}
