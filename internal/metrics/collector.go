package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Collector tracks throughput and error metrics using atomic operations
// for lock-free, high-performance counting.
type Collector struct {
	eventsIngested  atomic.Int64
	eventsProcessed atomic.Int64
	batchesWritten  atomic.Int64
	errors          atomic.Int64
	totalLatencyNs  atomic.Int64 // cumulative batch write latency in nanoseconds
	startTime       time.Time
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		startTime: time.Now(),
	}
}

// RecordIngested increments the ingested events counter.
func (c *Collector) RecordIngested(count int) {
	c.eventsIngested.Add(int64(count))
}

// RecordBatch records a successful batch write.
func (c *Collector) RecordBatch(count int, latency time.Duration) {
	c.eventsProcessed.Add(int64(count))
	c.batchesWritten.Add(1)
	c.totalLatencyNs.Add(int64(latency))
}

// RecordError increments the error counter.
func (c *Collector) RecordError() {
	c.errors.Add(1)
}

// Snapshot returns a point-in-time view of all metrics.
type Snapshot struct {
	Uptime           string  `json:"uptime"`
	EventsIngested   int64   `json:"events_ingested"`
	EventsProcessed  int64   `json:"events_processed"`
	BatchesWritten   int64   `json:"batches_written"`
	Errors           int64   `json:"errors"`
	AvgBatchLatency  string  `json:"avg_batch_latency_ms"`
	IngestRate       float64 `json:"ingest_rate_per_sec"`
	ProcessRate      float64 `json:"process_rate_per_sec"`
	QueueDepth       int64   `json:"queue_depth"`
}

// GetSnapshot returns current metrics.
func (c *Collector) GetSnapshot() Snapshot {
	elapsed := time.Since(c.startTime).Seconds()
	ingested := c.eventsIngested.Load()
	processed := c.eventsProcessed.Load()
	batches := c.batchesWritten.Load()
	totalLatency := c.totalLatencyNs.Load()

	var avgLatency string
	if batches > 0 {
		avgMs := float64(totalLatency) / float64(batches) / float64(time.Millisecond)
		avgLatency = time.Duration(int64(avgMs) * int64(time.Millisecond)).String()
	} else {
		avgLatency = "0s"
	}

	var ingestRate, processRate float64
	if elapsed > 0 {
		ingestRate = float64(ingested) / elapsed
		processRate = float64(processed) / elapsed
	}

	return Snapshot{
		Uptime:          time.Since(c.startTime).Truncate(time.Second).String(),
		EventsIngested:  ingested,
		EventsProcessed: processed,
		BatchesWritten:  batches,
		Errors:          c.errors.Load(),
		AvgBatchLatency: avgLatency,
		IngestRate:      ingestRate,
		ProcessRate:     processRate,
		QueueDepth:      ingested - processed,
	}
}

// Handler returns an HTTP handler that serves metrics as JSON.
func (c *Collector) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c.GetSnapshot())
	}
}
