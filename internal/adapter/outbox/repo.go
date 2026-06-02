package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"
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
	ID           string
	Topic        string
	Key          string
	Payload      []byte
	AttemptCount int
}

func (r *Repo) ClaimPending(ctx context.Context, limit int) ([]PendingRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`WITH claimed AS (
		     UPDATE outbox_message
		     SET status = 'IN_FLIGHT',
		         locked_at = now(),
		         attempt_count = attempt_count + 1
		     WHERE id IN (
		         SELECT id FROM outbox_message
		         WHERE status = 'PENDING' AND next_retry_at <= now()
		         ORDER BY created_at
		         FOR UPDATE SKIP LOCKED
		         LIMIT $1
		     )
		     RETURNING id, topic, key, payload, attempt_count, created_at
		 )
		 SELECT id, topic, key, payload, attempt_count
		 FROM claimed
		 ORDER BY created_at`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("outbox.ClaimPending: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []PendingRow
	for rows.Next() {
		var row PendingRow
		if err := rows.Scan(&row.ID, &row.Topic, &row.Key, &row.Payload, &row.AttemptCount); err != nil {
			return nil, fmt.Errorf("outbox.ClaimPending scan: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox.ClaimPending rows: %w", err)
	}
	return result, nil
}

func (r *Repo) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE outbox_message
		 SET status = 'PUBLISHED', published_at = now(), last_error = NULL
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("outbox.MarkPublished: %w", err)
	}
	return nil
}

const (
	backoffBaseSeconds = 1
	backoffCapSeconds  = 3600
)

func (r *Repo) MarkFailed(ctx context.Context, id string, publishErr error, maxAttempts int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE outbox_message
		 SET status = CASE WHEN attempt_count >= $3 THEN 'DEAD' ELSE 'PENDING' END,
		     last_error = $2,
		     next_retry_at = now() + (LEAST($4 * power(2, attempt_count), $5) * interval '1 second')
		 WHERE id = $1`,
		id, publishErr.Error(), maxAttempts, backoffBaseSeconds, backoffCapSeconds,
	)
	if err != nil {
		return fmt.Errorf("outbox.MarkFailed: %w", err)
	}
	return nil
}

func (r *Repo) RecoverStuck(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE outbox_message
		 SET status = 'PENDING', locked_at = NULL
		 WHERE status = 'IN_FLIGHT' AND locked_at < now() - ($1 * interval '1 second')`,
		olderThan.Seconds(),
	)
	if err != nil {
		return 0, fmt.Errorf("outbox.RecoverStuck: %w", err)
	}
	return res.RowsAffected()
}
