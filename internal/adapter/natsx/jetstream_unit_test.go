package natsx

import "testing"

func TestClient_Close_NilConn(t *testing.T) {
	t.Parallel()
	c := &Client{}
	c.Close()
}

func TestConstants(t *testing.T) {
	t.Parallel()
	if BookingStreamName == "" {
		t.Fatalf("stream name empty")
	}
	if BookingStreamSubjects == "" {
		t.Fatalf("subjects empty")
	}
}
