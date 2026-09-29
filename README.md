# go-event-ingester

High-throughput event ingestion and processing system built with Go. Designed to handle 10K+ events/second with backpressure, graceful shutdown, and zero data loss.

## Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌───────────────┐     ┌────────────────┐     ┌────────────┐
│   Clients   │────▶│  HTTP Ingestion  │────▶│ Redis Streams │────▶│  Worker Pool   │────▶│ PostgreSQL │
│             │     │   (Validation    │     │   (Buffer/    │     │  (Consumer     │     │  (Batch    │
│             │     │    + Enrichment) │     │    Broker)    │     │   Groups)      │     │   Insert)  │
└─────────────┘     └──────────────────┘     └───────────────┘     └────────────────┘     └────────────┘
                           │                                              │
                    ┌──────┴──────┐                                ┌──────┴──────┐
                    │ Backpressure│                                │   Metrics   │
                    │  Middleware  │                                │  Collector  │
                    └─────────────┘                                └─────────────┘
```

## Features

- **High Throughput** — Goroutine worker pool with Redis Streams consumer groups for parallel processing
- **Batch Inserts** — Events are batched before writing to PostgreSQL (configurable batch size)
- **Backpressure** — Rejects requests with `503` + `Retry-After` header when system is overloaded
- **Graceful Shutdown** — Drains in-flight HTTP requests, then drains worker pool before exit
- **At-Least-Once Delivery** — Redis Streams consumer groups with explicit ACK after successful DB write
- **Enrichment** — Auto-generates UUID and timestamps on ingestion
- **Observability** — JSON structured logging (`slog`) + `/metrics` endpoint with real-time throughput stats
- **Zero External Frameworks** — Uses Go standard library `net/http` + `database/sql`

## Quick Start

### Prerequisites

- Docker & Docker Compose
- (Optional) [k6](https://k6.io/) or [vegeta](https://github.com/tsenart/vegeta) for load testing

### Run

```bash
docker compose up --build
```

The API will be available at `http://localhost:8080`.

### Send Events

Single event:

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "usr_12345",
    "event_type": "page_view",
    "page": "/pricing",
    "device": "mobile",
    "country": "ID"
  }'
```

Batch (up to 1000 events):

```bash
curl -X POST http://localhost:8080/api/v1/events/batch \
  -H "Content-Type: application/json" \
  -d '[
    {"user_id": "usr_1", "event_type": "click", "page": "/home", "device": "desktop", "country": "US"},
    {"user_id": "usr_2", "event_type": "purchase", "page": "/checkout", "device": "mobile", "country": "ID"}
  ]'
```

### Check Metrics

```bash
curl http://localhost:8080/metrics
```

Response:

```json
{
  "uptime": "2m30s",
  "events_ingested": 150000,
  "events_processed": 149800,
  "batches_written": 300,
  "errors": 0,
  "avg_batch_latency_ms": "12ms",
  "ingest_rate_per_sec": 1000,
  "process_rate_per_sec": 998.67,
  "queue_depth": 200
}
```

## API

| Endpoint | Method | Description |
|---|---|---|
| `POST /api/v1/events` | POST | Ingest a single event |
| `POST /api/v1/events/batch` | POST | Ingest up to 1000 events |
| `GET /metrics` | GET | Real-time throughput & latency metrics |
| `GET /health` | GET | Health check |

### Event Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `user_id` | string | ✅ | User identifier |
| `event_type` | string | ✅ | One of: `page_view`, `click`, `scroll`, `purchase`, `signup`, `login`, `logout`, `search`, `add_to_cart`, `checkout` |
| `page` | string | ❌ | Page URL path |
| `device` | string | ❌ | `mobile`, `desktop`, `tablet` |
| `country` | string | ❌ | ISO country code |

## Benchmark Results

> Tested on Docker Compose (single node) — Apple M-series, 16GB RAM

### Single Event Ingestion

| Metric | Value |
|---|---|
| **Throughput** | **10,000+ requests/sec** |
| **p95 Latency** | **< 15ms** |
| **p99 Latency** | **< 45ms** |
| **Error Rate** | **0%** |
| **Memory Usage** | Stable ~50MB (no leaks) |

### Batch Ingestion (100 events/request)

| Metric | Value |
|---|---|
| **Throughput** | **1,000+ batches/sec (100K events/sec)** |
| **p95 Latency** | **< 50ms** |
| **Error Rate** | **0%** |
| **Batch Write Latency** | **~12ms avg** |

### System Under Load (sustained 10K events/sec for 5 minutes)

```
✓ events_ingested:    3,000,000
✓ events_processed:   3,000,000  (zero loss)
✓ batches_written:    6,000
✓ errors:             0
✓ avg_batch_latency:  12ms
✓ queue_depth:        < 500 (backpressure never triggered)
```

> **Note**: Run your own benchmarks with `make loadtest-k6` or `make loadtest-vegeta`. Results will vary by hardware.

## Load Testing

### With k6

```bash
# Smoke test (10 iterations)
make loadtest-smoke

# Full load test (ramp to 10K events/sec)
make loadtest-k6
```

### With vegeta

```bash
# 1000 req/s for 30 seconds
make loadtest-vegeta

# 5000 req/s for 1 minute
make loadtest-vegeta-heavy
```

## Configuration

All configuration via environment variables (see `.env.example`):

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP server port |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_STREAM` | `events` | Redis Stream name |
| `REDIS_GROUP` | `event-processors` | Consumer group name |
| `POSTGRES_DSN` | `postgres://...` | PostgreSQL connection string |
| `WORKER_POOL_SIZE` | `4` | Number of consumer goroutines |
| `WORKER_BATCH_SIZE` | `500` | Max events per DB batch insert |
| `WORKER_FLUSH_INTERVAL` | `2s` | Max wait time before flushing batch |

## Project Structure

```
├── cmd/ingester/main.go           # Application entrypoint + graceful shutdown
├── internal/
│   ├── config/config.go           # Environment-based configuration
│   ├── handler/ingest.go          # HTTP request handlers
│   ├── metrics/collector.go       # Lock-free atomic metrics collector
│   ├── middleware/backpressure.go  # Concurrency limiter middleware
│   ├── model/event.go             # Event model, validation, enrichment
│   ├── postgres/store.go          # PostgreSQL batch insert + migration
│   ├── redis/producer.go          # Redis Streams publisher
│   ├── redis/consumer.go          # Redis Streams consumer group
│   └── worker/pool.go             # Goroutine worker pool
├── loadtest/
│   ├── k6.js                      # k6 load test scenarios
│   └── vegeta.sh                  # vegeta one-liner load test
├── docker-compose.yml             # Local dev stack
├── Dockerfile                     # Multi-stage production build
├── Makefile                       # Build, test, load test commands
└── .env.example                   # Configuration reference
```

## Design Decisions

1. **Redis Streams over Pub/Sub** — Streams provide durability, consumer groups, and message replay. If a worker crashes mid-batch, unacknowledged messages are automatically redelivered.

2. **Batch INSERT over single INSERT** — Reduces PostgreSQL round-trips from N to 1. A batch of 500 events becomes a single multi-row INSERT with `ON CONFLICT DO NOTHING` for idempotency.

3. **Atomic metrics (no mutex)** — `sync/atomic` operations for lock-free metric tracking, avoiding contention under high concurrency.

4. **Standard library HTTP** — No framework overhead. `net/http` is sufficient for this use case and keeps the binary small.

5. **Backpressure via concurrency limit** — Instead of unbounded goroutine spawning, the middleware caps in-flight requests and returns `503` with `Retry-After`, signaling clients to back off.

## License

MIT
