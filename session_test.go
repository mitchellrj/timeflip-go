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

func TestReadDeviceInfoReturnsPartialFields(t *testing.T) {
	session := newTestSession(t, &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charDeviceName:       []byte("TIMEFLIP2"),
			charFirmwareRevision: []byte("1.2.3"),
		},
		readErrs: map[CharacteristicID]error{
			charManufacturerName: ErrProtocol,
			charModelNumber:      ErrProtocol,
			charHardwareRevision: ErrProtocol,
			charSystemID:         ErrProtocol,
		},
	})
	info, err := session.ReadDeviceInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "TIMEFLIP2" || info.FirmwareRevision != "1.2.3" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.ManufacturerName != "" || info.ModelNumber != "" || info.HardwareRevision != "" || info.SystemID != "" {
		t.Fatalf("expected missing fields to stay blank: %+v", info)
	}
}

func TestReadDeviceInfoFailsWhenAllFieldsMissing(t *testing.T) {
	session := newTestSession(t, &fakeConnection{readErrs: map[CharacteristicID]error{
		charDeviceName:       ErrProtocol,
		charManufacturerName: ErrProtocol,
		charModelNumber:      ErrProtocol,
		charHardwareRevision: ErrProtocol,
		charFirmwareRevision: ErrProtocol,
		charSystemID:         ErrProtocol,
	}})
	_, err := session.ReadDeviceInfo(context.Background())
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error when all fields are missing, got %v", err)
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

func TestSendCommandMalformedStatusPreservesPayload(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {byte(cmdName), 0x00},
	}})
	result, err := session.SetName(context.Background(), "Desk Timer", CommandOptions{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	if string(result.Payload) != string([]byte{byte(cmdName), 0x00}) || string(result.Status.Raw) != string([]byte{byte(cmdName), 0x00}) {
		t.Fatalf("expected raw malformed command payload to be preserved, got result=%+v", result)
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

func TestEventsTimeFlipEventsCharacteristicEmitsRawEvent(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, errs, err := session.Events(ctx, EventOptions{Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	conn.subscriptions[charEvents] <- Notification{Characteristic: charEvents, Payload: []byte{0xAA, 0x01}}
	select {
	case event := <-events:
		if event.Kind != EventRaw || event.Source != charEvents {
			t.Fatalf("unexpected event: %+v", event)
		}
		payload, ok := event.Payload.([]byte)
		if !ok || len(payload) != 2 || payload[0] != 0xAA || payload[1] != 0x01 {
			t.Fatalf("unexpected raw payload: %#v", event.Payload)
		}
	case err := <-errs:
		t.Fatalf("unexpected stream error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for raw event")
	}
}

func TestNotificationDecodeErrorIncludesSource(t *testing.T) {
	session := newTestSession(t, &fakeConnection{})
	_, err := session.decodeNotification(Notification{Characteristic: charHistory, Payload: []byte{0x01}}, false)
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %v", err)
	}
	if opErr.Operation != "events" || opErr.Stage != "history" {
		t.Fatalf("unexpected error context: %+v", opErr)
	}
}
