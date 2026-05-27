package natsx

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	BookingStreamName     = "booking-events"
	BookingStreamSubjects = "booking.>"
)

type Client struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

func Connect(ctx context.Context, url string) (*Client, error) {
	conn, err := nats.Connect(url, nats.Name("booking-svc"))
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream init: %w", err)
	}

	c := &Client{conn: conn, js: js}
	if err := c.ensureBookingStream(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) ensureBookingStream(ctx context.Context) error {
	cfg := jetstream.StreamConfig{
		Name:     BookingStreamName,
		Subjects: []string{BookingStreamSubjects},
		Storage:  jetstream.FileStorage,
	}
	_, err := c.js.CreateStream(ctx, cfg)
	if err == nil {
		return nil
	}
	if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		_, updErr := c.js.UpdateStream(ctx, cfg)
		if updErr != nil {
			return fmt.Errorf("update stream %q: %w", cfg.Name, updErr)
		}
		return nil
	}
	return fmt.Errorf("create stream %q: %w", cfg.Name, err)
}

func (c *Client) Publish(ctx context.Context, subject string, payload []byte) error {
	if _, err := c.js.Publish(ctx, subject, payload); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}

func (c *Client) Consume(ctx context.Context, durable, filterSubject string, handler func(jetstream.Msg)) (jetstream.ConsumeContext, error) {
	cons, err := c.js.CreateOrUpdateConsumer(ctx, BookingStreamName, jetstream.ConsumerConfig{
		Durable:       durable,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: filterSubject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		MaxAckPending: 256,
	})
	if err != nil {
		return nil, fmt.Errorf("create consumer %q: %w", durable, err)
	}
	cc, err := cons.Consume(handler)
	if err != nil {
		return nil, fmt.Errorf("start consume %q: %w", durable, err)
	}
	return cc, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Status() nats.Status {
	if c.conn == nil {
		return nats.DISCONNECTED
	}
	return c.conn.Status()
}
