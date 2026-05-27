//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
)

func TestTxManager_CommitOnSuccess(t *testing.T) {
	db := newTestDB(t)
	mgr := NewTxManager(db)
	ctx := context.Background()

	err := mgr.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := txFromContext(ctx)
		if !ok {
			t.Fatalf("expected tx in context")
		}
		_, err := tx.ExecContext(ctx,
			`CREATE TEMP TABLE tx_probe(id INT)`)
		return err
	})
	if err != nil {
		t.Fatalf("WithinTx: %v", err)
	}
}

func TestTxManager_RollbackOnError(t *testing.T) {
	db := newTestDB(t)
	mgr := NewTxManager(db)
	ctx := context.Background()

	repo := NewBookingRepo(db)
	b := makeBooking(t)

	wantErr := errors.New("boom")
	err := mgr.WithinTx(ctx, func(ctx context.Context) error {
		if err := repo.Save(ctx, b); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}

	_, err = repo.FindByID(ctx, b.ID())
	if err == nil {
		t.Fatalf("booking should not have been persisted after rollback")
	}
}

func TestTxManager_Nested_ReusesOuterTx(t *testing.T) {
	db := newTestDB(t)
	mgr := NewTxManager(db)
	ctx := context.Background()

	err := mgr.WithinTx(ctx, func(outerCtx context.Context) error {
		outerTx, _ := txFromContext(outerCtx)
		return mgr.WithinTx(outerCtx, func(innerCtx context.Context) error {
			innerTx, ok := txFromContext(innerCtx)
			if !ok || innerTx != outerTx {
				t.Fatalf("nested call must reuse outer tx")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatalf("WithinTx: %v", err)
	}
}
