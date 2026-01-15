.PHONY: build run test clean migrate lint

# Build the application
build:
	go build -o bin/server cmd/server/main.go

# Run the application
run:
	go run cmd/server/main.go

# Run tests
test:
	go test -v -cover ./...

# Run tests with coverage report
coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run database migrations
migrate:
	go run cmd/server/main.go migrate

# Lint the code
lint:
	golangci-lint run ./...

# Format the code
fmt:
	go fmt ./...
	goimports -w .

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run security scan
security:
	gosec ./...

# Build Docker image
docker-build:
	docker build -t ai-reimbursement:latest .

# Run Docker container
docker-run:
	docker run -d \
		-p 8080:8080 \
		-v $(PWD)/data:/root/data \
		-v $(PWD)/generated_vouchers:/root/generated_vouchers \
		-e LARK_APP_ID=$(LARK_APP_ID) \
		-e LARK_APP_SECRET=$(LARK_APP_SECRET) \
		-e OPENAI_API_KEY=$(OPENAI_API_KEY) \
		-e ACCOUNTANT_EMAIL=$(ACCOUNTANT_EMAIL) \
		--name ai-reimbursement \
		ai-reimbursement:latest

# Stop Docker container
docker-stop:
	docker stop ai-reimbursement
	docker rm ai-reimbursement

# View logs
logs:
	tail -f logs/app.log

# Backup database
backup:
	mkdir -p backups
	cp data/reimbursement.db backups/reimbursement_$(shell date +%Y%m%d_%H%M%S).db

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  coverage      - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  migrate       - Run database migrations"
	@echo "  lint          - Lint the code"
	@echo "  fmt           - Format the code"
	@echo "  deps          - Download dependencies"
	@echo "  security      - Run security scan"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  docker-stop   - Stop Docker container"
	@echo "  logs          - View application logs"
	@echo "  backup        - Backup database"
