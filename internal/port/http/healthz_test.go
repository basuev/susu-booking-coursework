package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats.go"
)

type fakeNats struct {
	status nats.Status
}

func (f *fakeNats) Status() nats.Status { return f.status }

func TestHealthz_OKWhenAllUp(t *testing.T) {
	t.Parallel()

	s := NewServer(":0", nil, &fakeNats{status: nats.CONNECTED})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthz_BadWhenNatsDown(t *testing.T) {
	t.Parallel()

	s := NewServer(":0", nil, &fakeNats{status: nats.DISCONNECTED})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rec.Code)
	}
}

func TestHealthz_OKWithoutNats(t *testing.T) {
	t.Parallel()

	s := NewServer(":0", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
