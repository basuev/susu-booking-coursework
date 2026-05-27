//go:build integration

package natsx

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startNATS(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "nats:2-alpine",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start nats: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = c.Terminate(ctx)
	})

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := c.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	return fmt.Sprintf("nats://%s:%s", host, port.Port())
}

func TestJetStream_ConnectPublishConsume(t *testing.T) {
	url := startNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(client.Close)

	received := make(chan []byte, 1)
	var count int32

	cc, err := client.Consume(ctx, "test-consumer", "booking.created", func(m jetstream.Msg) {
		if atomic.AddInt32(&count, 1) == 1 {
			received <- m.Data()
		}
		_ = m.Ack()
	})
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	t.Cleanup(cc.Stop)

	payload := []byte(`{"hello":"world"}`)
	if err := client.Publish(ctx, "booking.created", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if string(got) != string(payload) {
			t.Fatalf("payload mismatch: want %s got %s", payload, got)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("did not receive message in time")
	}
}

func TestJetStream_EnsureStreamIdempotent(t *testing.T) {
	url := startNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c1, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	c1.Close()

	c2, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("second connect (must be idempotent): %v", err)
	}
	c2.Close()
}
