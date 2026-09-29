package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/radityajayantara/go-event-ingester/internal/config"
	"github.com/radityajayantara/go-event-ingester/internal/model"
)

// Store handles PostgreSQL operations for event persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates a new PostgreSQL store and runs migrations.
func NewStore(cfg config.PostgresConfig) (*Store, error) {
	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return store, nil
}

// migrate creates the events table if it doesn't exist.
func (s *Store) migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS events (
			id          VARCHAR(36) PRIMARY KEY,
			user_id     VARCHAR(255) NOT NULL,
			event_type  VARCHAR(50)  NOT NULL,
			page        VARCHAR(500),
			device      VARCHAR(50),
			country     VARCHAR(10),
			timestamp   TIMESTAMPTZ  NOT NULL,
			ingested_at TIMESTAMPTZ  NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_events_user_id    ON events (user_id);
		CREATE INDEX IF NOT EXISTS idx_events_event_type ON events (event_type);
		CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON events (timestamp);
	`
	_, err := s.db.Exec(query)
	return err
}

// InsertBatch inserts multiple events in a single query for high throughput.
func (s *Store) InsertBatch(ctx context.Context, events []model.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Build bulk insert: INSERT INTO events (...) VALUES ($1,...), ($9,...), ...
	const cols = 8
	valueStrings := make([]string, 0, len(events))
	valueArgs := make([]interface{}, 0, len(events)*cols)

	for i, e := range events {
		base := i * cols
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
		))
		valueArgs = append(valueArgs,
			e.ID, e.UserID, e.EventType, e.Page, e.Device, e.Country, e.Timestamp, e.IngestedAt,
		)
	}

	query := fmt.Sprintf(
		`INSERT INTO events (id, user_id, event_type, page, device, country, timestamp, ingested_at)
		 VALUES %s
		 ON CONFLICT (id) DO NOTHING`,
		strings.Join(valueStrings, ", "),
	)

	_, err := s.db.ExecContext(ctx, query, valueArgs...)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
