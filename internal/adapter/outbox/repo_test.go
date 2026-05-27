//go:build integration

package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRepo_ClaimPending_MovesToInFlight(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	id := insertPending(t, db, "booking.created", "b-1")

	rows, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != id {
		t.Fatalf("want 1 row %s, got %#v", id, rows)
	}
	if rows[0].AttemptCount != 1 {
		t.Fatalf("attempt_count should be 1, got %d", rows[0].AttemptCount)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM outbox_message WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "IN_FLIGHT" {
		t.Fatalf("want IN_FLIGHT, got %s", status)
	}
}

func TestRepo_ClaimPending_ConcurrentNoDoublePublish(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		insertPending(t, db, "booking.created", "b-x")
	}

	const workers = 4
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		seen  = map[string]int{}
		total int
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			rows, err := repo.ClaimPending(ctx, 50)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			mu.Lock()
			for _, r := range rows {
				seen[r.ID]++
				total++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if total != 10 {
		t.Fatalf("total claimed: want 10, got %d (seen=%v)", total, seen)
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("row %s claimed %d times; must be 1", id, count)
		}
	}
}

func TestRepo_MarkPublished_SetsStatusAndTime(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	id := insertPending(t, db, "booking.created", "b-pub")
	if _, err := repo.ClaimPending(ctx, 1); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := repo.MarkPublished(ctx, id); err != nil {
		t.Fatalf("mark: %v", err)
	}
	var status string
	var publishedAt *time.Time
	if err := db.QueryRow(`SELECT status, published_at FROM outbox_message WHERE id = $1`, id).Scan(&status, &publishedAt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != "PUBLISHED" {
		t.Fatalf("want PUBLISHED, got %s", status)
	}
	if publishedAt == nil {
		t.Fatalf("published_at must be set")
	}
}

func TestRepo_MarkFailed_BackoffAndDLQ(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	id := insertPending(t, db, "booking.created", "b-fail")
	rows, err := repo.ClaimPending(ctx, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("claim: %v rows=%d", err, len(rows))
	}
	maxAttempts := 3
	if err := repo.MarkFailed(ctx, id, errors.New("first fail"), maxAttempts); err != nil {
		t.Fatalf("mark failed 1: %v", err)
	}

	var status string
	var lastErr *string
	var attempt int
	var nextRetry time.Time
	if err := db.QueryRow(
		`SELECT status, last_error, attempt_count, next_retry_at FROM outbox_message WHERE id = $1`, id,
	).Scan(&status, &lastErr, &attempt, &nextRetry); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != "PENDING" {
		t.Fatalf("want PENDING after first fail, got %s", status)
	}
	if lastErr == nil || *lastErr != "first fail" {
		t.Fatalf("last_error not set: %v", lastErr)
	}
	firstBackoff := nextRetry

	if _, err := db.ExecContext(ctx,
		`UPDATE outbox_message SET status = 'PENDING', next_retry_at = now() WHERE id = $1`, id,
	); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if rs, err := repo.ClaimPending(ctx, 1); err != nil || len(rs) != 1 {
		t.Fatalf("claim 2: err=%v rows=%d", err, len(rs))
	}
	if err := repo.MarkFailed(ctx, id, errors.New("second fail"), maxAttempts); err != nil {
		t.Fatalf("mark failed 2: %v", err)
	}

	var secondBackoff time.Time
	var attempt2 int
	if err := db.QueryRow(`SELECT next_retry_at, attempt_count FROM outbox_message WHERE id = $1`, id).Scan(&secondBackoff, &attempt2); err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	if !secondBackoff.After(firstBackoff) {
		t.Fatalf("backoff must grow: first=%v second=%v", firstBackoff, secondBackoff)
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE outbox_message SET status = 'PENDING', next_retry_at = now() WHERE id = $1`, id,
	); err != nil {
		t.Fatalf("reset 2: %v", err)
	}
	if rs, err := repo.ClaimPending(ctx, 1); err != nil || len(rs) != 1 {
		t.Fatalf("claim 3: err=%v rows=%d", err, len(rs))
	}
	if err := repo.MarkFailed(ctx, id, errors.New("third fail"), maxAttempts); err != nil {
		t.Fatalf("mark failed 3: %v", err)
	}

	var finalStatus string
	if err := db.QueryRow(`SELECT status FROM outbox_message WHERE id = $1`, id).Scan(&finalStatus); err != nil {
		t.Fatalf("scan 3: %v", err)
	}
	if finalStatus != "DEAD" {
		t.Fatalf("at maxAttempts row must be DEAD, got %s", finalStatus)
	}
}

func TestRepo_RecoverStuck_MovesInFlightToPending(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	id := insertPending(t, db, "booking.created", "b-stuck")
	if _, err := repo.ClaimPending(ctx, 1); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE outbox_message SET locked_at = now() - interval '10 minutes' WHERE id = $1`, id,
	); err != nil {
		t.Fatalf("age locked_at: %v", err)
	}

	recovered, err := repo.RecoverStuck(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("want 1 recovered, got %d", recovered)
	}

	var status string
	var locked *time.Time
	if err := db.QueryRow(`SELECT status, locked_at FROM outbox_message WHERE id = $1`, id).Scan(&status, &locked); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != "PENDING" {
		t.Fatalf("want PENDING, got %s", status)
	}
	if locked != nil {
		t.Fatalf("locked_at must be cleared")
	}
}
