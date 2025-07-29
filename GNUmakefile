default: fmt lint install generate

build:
	go build -v .

install: build
	go install -v .

# requires https://golangci-lint.run/welcome/install/#local-installation
lint:
	$(shell go env GOPATH)/bin/golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./internal/...

testacc-setup:
	@echo "Setting up test environment..."
	@cd tools && go run ./testenv/setup

testacc-clean:
	@echo "Cleaning up test environment..."
	@cd tools && go run ./testenv/cleanup

testacc-teardown:
	@echo "Tearing down test environment..."
	@cd tools && go run ./testenv/cleanup --teardown

testacc: testacc-setup
	@echo "Checking for tcsignal-aws binary..."
	@if [ ! -f "./bin/tcsignal-aws" ]; then echo "Binary not found, downloading..."; $(MAKE) install-signal-binary; fi
	@echo "Running acceptance tests..."
	TF_ACC=1 TF_ACC_LOG_PATH=acceptance.log AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test go test -v -cover -timeout 120m ./internal/provider -run TestAcc
	@$(MAKE) testacc-clean

# Download tcsignal-aws binary for testing and development
install-signal-binary:
	@echo "Detecting platform..."
	@OS=$$(uname -s); \
	ARCH=$$(uname -m); \
	case $$ARCH in \
		x86_64) ARCH="x86_64" ;; \
		aarch64|arm64) ARCH="arm64" ;; \
		*) echo "Unsupported architecture: $$ARCH"; exit 1 ;; \
	esac; \
	case $$OS in \
		Linux) OS="Linux" ;; \
		Darwin) OS="Darwin" ;; \
		*) echo "Unsupported OS: $$OS"; exit 1 ;; \
	esac; \
	BINARY_NAME="signal-aws_$${OS}_$${ARCH}.tar.gz"; \
	echo "Downloading $$BINARY_NAME..."; \
	mkdir -p bin; \
	curl -L -o bin/$$BINARY_NAME "https://github.com/TerraConstructs/signal-aws/releases/latest/download/$$BINARY_NAME"; \
	cd bin && tar -xzf $$BINARY_NAME; \
	rm $$BINARY_NAME; \
	chmod +x tcsignal-aws; \
	echo "✅ tcsignal-aws binary installed to ./bin/tcsignal-aws"

ecr-auth: ## Authenticate to AWS ECR Public
	@which aws > /dev/null || (echo "AWS CLI not found. Install with: https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html" && exit 1)
	@echo "Authenticating to AWS ECR Public..."
	@(aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws)

.PHONY: fmt lint test testacc testacc-setup testacc-clean testacc-teardown build install generate install-signal-binary ecr-auth
