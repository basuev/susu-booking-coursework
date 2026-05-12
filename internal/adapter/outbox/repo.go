package outbox

import (
	"context"
	"database/sql"
	"fmt"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func AppendMessage(ctx context.Context, tx *sql.Tx, msg Message) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO outbox_message (topic, key, payload) VALUES ($1, $2, $3)`,
		msg.Topic, msg.Key, msg.Payload,
	)
	if err != nil {
		return fmt.Errorf("outbox.AppendMessage: %w", err)
	}
	return nil
}

type PendingRow struct {
	ID      string
	Topic   string
	Key     string
	Payload []byte
}

func (r *Repo) SelectPending(ctx context.Context, limit int) ([]PendingRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("outbox.SelectPending begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		`SELECT id, topic, key, payload
		 FROM outbox_message
		 WHERE status = 'PENDING'
		 ORDER BY created_at ASC
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("outbox.SelectPending query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []PendingRow
	for rows.Next() {
		var row PendingRow
		if err := rows.Scan(&row.ID, &row.Topic, &row.Key, &row.Payload); err != nil {
			return nil, fmt.Errorf("outbox.SelectPending scan: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox.SelectPending rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("outbox.SelectPending commit: %w", err)
	}
	return result, nil
}

func (r *Repo) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE outbox_message SET status = 'PUBLISHED', published_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("outbox.MarkPublished: %w", err)
	}
	return nil
}
