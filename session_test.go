package timeflip

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestSession(t *testing.T, conn *fakeConnection) *Session {
	t.Helper()
	client, err := NewClient(&fakeTransport{connections: map[DeviceID]*fakeConnection{"tf": conn}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(context.Background(), ConnectRequest{DeviceID: "tf"})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestAuthorizeRejectsWrongPassword(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x02},
	}})
	_, err := session.Authorize(context.Background(), "000000")
	if !errors.Is(err, ErrAuthorizationFailed) {
		t.Fatalf("expected authorization failure, got %v", err)
	}
}

func TestReadBattery(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charBattery: {88},
	}})
	battery, err := session.ReadBattery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if battery.Percentage != 88 {
		t.Fatalf("unexpected battery: %+v", battery)
	}
}

func TestSendCommandRejected(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {byte(cmdLock), 0x01},
	}})
	_, err := session.SetLock(context.Background(), true, CommandOptions{})
	if !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("expected command rejected, got %v", err)
	}
}

func TestEventsDecodeAndCloseOnCancel(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	ctx, cancel := context.WithCancel(context.Background())
	events, errs, err := session.Events(ctx, EventOptions{Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	conn.subscriptions[charFacets] <- Notification{Characteristic: charFacets, Payload: []byte{3}}
	select {
	case event := <-events:
		if event.Kind != EventFacet {
			t.Fatalf("unexpected event: %+v", event)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("events channel still open")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for close")
	}
}
