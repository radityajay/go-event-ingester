package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/radityajayantara/go-event-ingester/internal/config"
	"github.com/radityajayantara/go-event-ingester/internal/model"
	"github.com/redis/go-redis/v9"
)

// Producer publishes events to a Redis Stream.
type Producer struct {
	client *redis.Client
	stream string
}

// NewProducer creates a new Redis Streams producer.
func NewProducer(cfg config.RedisConfig) (*Producer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &Producer{
		client: client,
		stream: cfg.Stream,
	}, nil
}

// Publish sends an event to the Redis Stream.
func (p *Producer) Publish(ctx context.Context, event model.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()
}

// PublishBatch sends multiple events to the Redis Stream using a pipeline.
func (p *Producer) PublishBatch(ctx context.Context, events []model.Event) error {
	pipe := p.client.Pipeline()

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: p.stream,
			Values: map[string]interface{}{
				"data": string(data),
			},
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Close closes the Redis connection.
func (p *Producer) Close() error {
	return p.client.Close()
}
