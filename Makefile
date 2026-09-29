.PHONY: build run test lint vet clean docker-up docker-down docker-build loadtest-smoke loadtest-k6 loadtest-vegeta metrics health

# ─── Build & Run ──────────────────────────────────────────────

build:
	go build -ldflags="-s -w" -o bin/ingester ./cmd/ingester

run: build
	./bin/ingester

# ─── Testing ──────────────────────────────────────────────────

test:
	go test ./... -v -count=1 -race

test-coverage:
	go test ./... -coverprofile=coverage.out -race
	go tool cover -func=coverage.out
	@rm -f coverage.out

# ─── Code Quality ────────────────────────────────────────────

vet:
	go vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "Install golangci-lint: https://golangci-lint.run/welcome/install/"
	golangci-lint run ./...

# ─── Docker ──────────────────────────────────────────────────

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

docker-build:
	docker compose build

docker-logs:
	docker compose logs -f app

# ─── Load Testing ────────────────────────────────────────────

loadtest-smoke:
	@echo "=== Smoke Test (10 iterations) ==="
	k6 run --iterations 10 loadtest/k6.js

loadtest-k6:
	@echo "=== Full Load Test (ramp to 10K events/sec) ==="
	k6 run loadtest/k6.js

loadtest-vegeta:
	@echo "=== Vegeta Load Test (1000 req/s, 30s) ==="
	./loadtest/vegeta.sh 1000 30s

loadtest-vegeta-heavy:
	@echo "=== Vegeta Heavy Load Test (5000 req/s, 60s) ==="
	./loadtest/vegeta.sh 5000 60s

# ─── Utilities ───────────────────────────────────────────────

metrics:
	@curl -s http://localhost:8080/metrics | python3 -m json.tool

health:
	@curl -s http://localhost:8080/health | python3 -m json.tool

clean:
	rm -rf bin/ coverage.out

help:
	@echo "Available targets:"
	@echo "  build              Build the binary"
	@echo "  run                Build and run locally"
	@echo "  test               Run all tests with race detector"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  vet                Run go vet"
	@echo "  lint               Run golangci-lint"
	@echo "  docker-up          Start all services (build + detached)"
	@echo "  docker-down        Stop all services and remove volumes"
	@echo "  docker-logs        Tail application logs"
	@echo "  loadtest-smoke     k6 smoke test (10 iterations)"
	@echo "  loadtest-k6        k6 full load test (ramp to 10K/s)"
	@echo "  loadtest-vegeta    vegeta 1K req/s for 30s"
	@echo "  loadtest-vegeta-heavy  vegeta 5K req/s for 60s"
	@echo "  metrics            Fetch /metrics endpoint"
	@echo "  health             Fetch /health endpoint"
	@echo "  clean              Remove build artifacts"
