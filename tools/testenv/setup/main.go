package main

import (
	"fmt"
	"log"
	"os"

	"tools/testenv"
)

func main() {
	if err := testenv.SetupTestEnvironment(); err != nil {
		log.Printf("❌ Failed to setup test environment: %v", err)
		os.Exit(1)
	}

	// Also purge queues to start with clean state
	if err := testenv.PurgeTestQueues(); err != nil {
		log.Printf("⚠️  Warning: Failed to purge test queues: %v", err)
		// Don't exit - setup succeeded even if purge failed
	}

	fmt.Println("🎉 Test environment setup complete!")
}
