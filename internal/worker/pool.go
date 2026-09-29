package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/radityajayantara/go-event-ingester/internal/config"
	"github.com/radityajayantara/go-event-ingester/internal/metrics"
	"github.com/radityajayantara/go-event-ingester/internal/postgres"
	redispkg "github.com/radityajayantara/go-event-ingester/internal/redis"
)

// Pool manages a set of worker goroutines that consume events from Redis Streams
// and batch-insert them into PostgreSQL.
type Pool struct {
	cfg       config.WorkerConfig
	redisCfg  config.RedisConfig
	store     *postgres.Store
	metrics   *metrics.Collector
	wg        sync.WaitGroup
	cancelFn  context.CancelFunc
}

// NewPool creates a new worker pool.
func NewPool(cfg config.WorkerConfig, redisCfg config.RedisConfig, store *postgres.Store, m *metrics.Collector) *Pool {
	return &Pool{
		cfg:      cfg,
		redisCfg: redisCfg,
		store:    store,
		metrics:  m,
	}
}

// Start launches the worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	for i := 0; i < p.cfg.PoolSize; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}

	slog.Info("worker pool started", "size", p.cfg.PoolSize)
}

// Stop signals all workers to stop and waits for them to finish.
func (p *Pool) Stop() {
	slog.Info("stopping worker pool...")
	if p.cancelFn != nil {
		p.cancelFn()
	}
	p.wg.Wait()
	slog.Info("worker pool stopped")
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()

	consumerName := fmt.Sprintf("worker-%d", id)
	consumer, err := redispkg.NewConsumer(p.redisCfg, consumerName)
	if err != nil {
		slog.Error("worker failed to create consumer", "worker", id, "error", err)
		return
	}
	defer consumer.Close()

	slog.Info("worker started", "worker", id)

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down", "worker", id)
			return
		default:
		}

		events, msgIDs, err := consumer.ReadBatch(ctx, int64(p.cfg.BatchSize), p.cfg.FlushInterval)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, clean shutdown.
			}
			slog.Error("worker read error", "worker", id, "error", err)
			p.metrics.RecordError()
			time.Sleep(time.Second) // Backoff on error.
			continue
		}

		if len(events) == 0 {
			continue
		}

		start := time.Now()
		if err := p.store.InsertBatch(ctx, events); err != nil {
			slog.Error("worker batch insert failed", "worker", id, "count", len(events), "error", err)
			p.metrics.RecordError()
			// Don't ACK — messages will be redelivered.
			time.Sleep(time.Second)
			continue
		}

		// ACK after successful insert.
		if err := consumer.Ack(ctx, msgIDs...); err != nil {
			slog.Error("worker ack failed", "worker", id, "error", err)
		}

		duration := time.Since(start)
		p.metrics.RecordBatch(len(events), duration)
		slog.Debug("worker processed batch", "worker", id, "count", len(events), "duration", duration)
	}
}
