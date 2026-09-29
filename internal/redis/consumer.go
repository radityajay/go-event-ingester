package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/radityajayantara/go-event-ingester/internal/config"
	"github.com/radityajayantara/go-event-ingester/internal/model"
	"github.com/redis/go-redis/v9"
)

// Consumer reads events from a Redis Stream using consumer groups.
type Consumer struct {
	client   *redis.Client
	stream   string
	group    string
	consumer string
}

// NewConsumer creates a new Redis Streams consumer.
func NewConsumer(cfg config.RedisConfig, consumerName string) (*Consumer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	// Create consumer group if it doesn't exist.
	// "0" means read from the beginning of the stream.
	err := client.XGroupCreateMkStream(context.Background(), cfg.Stream, cfg.Group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &Consumer{
		client:   client,
		stream:   cfg.Stream,
		group:    cfg.Group,
		consumer: consumerName,
	}, nil
}

// ReadBatch reads a batch of events from the stream.
// It blocks for up to blockDuration waiting for new messages.
func (c *Consumer) ReadBatch(ctx context.Context, count int64, blockDuration time.Duration) ([]model.Event, []string, error) {
	result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, ">"},
		Count:    count,
		Block:    blockDuration,
	}).Result()

	if err == redis.Nil {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	var events []model.Event
	var msgIDs []string

	for _, stream := range result {
		for _, msg := range stream.Messages {
			data, ok := msg.Values["data"].(string)
			if !ok {
				slog.Warn("skipping message with invalid data", "id", msg.ID)
				continue
			}

			var event model.Event
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				slog.Warn("skipping message with unmarshal error", "id", msg.ID, "error", err)
				continue
			}

			events = append(events, event)
			msgIDs = append(msgIDs, msg.ID)
		}
	}

	return events, msgIDs, nil
}

// Ack acknowledges processed messages so they won't be redelivered.
func (c *Consumer) Ack(ctx context.Context, ids ...string) error {
	return c.client.XAck(ctx, c.stream, c.group, ids...).Err()
}

// Close closes the Redis connection.
func (c *Consumer) Close() error {
	return c.client.Close()
}
