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

func TestAuthorizeBlankPasswordUsesDefault(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	result, err := session.Authorize(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authorized {
		t.Fatalf("expected authorization result, got %+v", result)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charPassword || string(conn.writes[0].payload) != DefaultPassword {
		t.Fatalf("expected default password write, got %+v", conn.writes)
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

func TestReadDeviceInfoFormatsSystemIDAsHex(t *testing.T) {
	session := newTestSession(t, &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charDeviceName: []byte("TIMEFLIP2"),
			charSystemID:   {0x51, 0x7D, 0x51, 0x7D},
		},
		readErrs: map[CharacteristicID]error{
			charManufacturerName: ErrProtocol,
			charModelNumber:      ErrProtocol,
			charHardwareRevision: ErrProtocol,
			charFirmwareRevision: ErrProtocol,
		},
	})
	info, err := session.ReadDeviceInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.SystemID != "0x517D517D" {
		t.Fatalf("unexpected system id: %+v", info)
	}
	if string(info.Raw[charSystemID]) != string([]byte{0x51, 0x7D, 0x51, 0x7D}) {
		t.Fatalf("expected raw system id bytes to be preserved: %+v", info.Raw)
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

func TestReadTaskParametersUsesDataResponse(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {byte(cmdReadTask), 0x01, 0x02, 0x00, 0x00, 0x05, 0xDC, 0x00, 0x00, 0x00, 0x2A},
	}}
	session := newTestSession(t, conn)
	task, err := session.ReadTaskParameters(context.Background(), 1, CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !task.Assigned || task.Facet != 1 || task.Mode != 2 || task.PomodoroLimitSeconds != 1500 || task.ElapsedSeconds != 42 {
		t.Fatalf("unexpected task parameters: %+v", task)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charCommand || string(conn.writes[0].payload) != string([]byte{byte(cmdReadTask), 0x01}) {
		t.Fatalf("unexpected command write: %+v", conn.writes)
	}
}

func TestReadTaskParametersWaitsForMatchingCommandResponse(t *testing.T) {
	conn := &fakeConnection{readSeq: map[CharacteristicID][][]byte{
		charCommandResult: {
			{0x18, 0x00},
			{byte(cmdReadTask), 0x01, 0x02, 0x00, 0x00, 0x05, 0xDC, 0x00, 0x00, 0x00, 0x2A},
		},
	}}
	session := newTestSession(t, conn)
	task, err := session.ReadTaskParameters(context.Background(), 1, CommandOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if task.Facet != 1 || task.PomodoroLimitSeconds != 1500 {
		t.Fatalf("unexpected task parameters: %+v", task)
	}
}

func TestReadTaskParametersTreats1900AsUnassigned(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x19, 0x00},
	}}
	session := newTestSession(t, conn)
	task, err := session.ReadTaskParameters(context.Background(), 3, CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if task.Assigned || task.Facet != 3 || string(task.Raw) != string([]byte{0x19, 0x00}) {
		t.Fatalf("expected unassigned task response, got %+v", task)
	}
}

func TestReadTapSettingsUsesDataResponse(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {byte(cmdTapRead), 0x3A, 20, 0x3B, 10, 0x3C, 5, 0x3D, 30},
	}}
	session := newTestSession(t, conn)
	settings, err := session.ReadTapSettings(context.Background(), CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Configured || settings.Threshold != 20 || settings.Limit != 10 || settings.Latency != 5 || settings.Window != 30 {
		t.Fatalf("unexpected tap settings: %+v", settings)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charCommand || string(conn.writes[0].payload) != string([]byte{byte(cmdTapRead)}) {
		t.Fatalf("unexpected command write: %+v", conn.writes)
	}
}

func TestReadTapSettingsWaitsForMatchingCommandResponse(t *testing.T) {
	conn := &fakeConnection{readSeq: map[CharacteristicID][][]byte{
		charCommandResult: {
			{0x18, 0x00},
			{byte(cmdTapRead), 0x3A, 20, 0x3B, 10, 0x3C, 5, 0x3D, 30},
		},
	}}
	session := newTestSession(t, conn)
	settings, err := session.ReadTapSettings(context.Background(), CommandOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Threshold != 20 || settings.Window != 30 {
		t.Fatalf("unexpected tap settings: %+v", settings)
	}
}

func TestReadTapSettingsTreats1900AsUnassigned(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x19, 0x00},
	}}
	session := newTestSession(t, conn)
	settings, err := session.ReadTapSettings(context.Background(), CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Configured || string(settings.Raw) != string([]byte{0x19, 0x00}) {
		t.Fatalf("expected unassigned tap settings response, got %+v", settings)
	}
}

func TestReadHistoryProtocolErrorIncludesPayloadDetails(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charHistory: {0x01, 0x02},
	}})
	_, err := session.ReadHistory(context.Background(), HistoryRequest{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	var payloadErr *ProtocolPayloadError
	if !errors.As(err, &payloadErr) {
		t.Fatalf("expected payload details, got %v", err)
	}
	if payloadErr.Expected == "" || string(payloadErr.Payload) != string([]byte{0x01, 0x02}) {
		t.Fatalf("unexpected payload error: %+v", payloadErr)
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

func TestEventsTimeFlipEventsCharacteristicPromotesSideText(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, errs, err := session.Events(ctx, EventOptions{Buffer: 2})
	if err != nil {
		t.Fatal(err)
	}
	conn.subscriptions[charEvents] <- Notification{Characteristic: charEvents, Payload: []byte("New Side: 0x04")}
	conn.subscriptions[charEvents] <- Notification{Characteristic: charEvents, Payload: []byte("New Side: 0x84")}

	select {
	case event := <-events:
		facet, ok := event.Payload.(FacetEvent)
		if event.Kind != EventFacet || event.Source != charEvents || !ok || facet.Facet != 4 {
			t.Fatalf("unexpected promoted facet event: %+v", event)
		}
		if string(event.Raw) != "New Side: 0x04" || string(facet.Raw) != "New Side: 0x04" {
			t.Fatalf("expected raw text to be preserved: event=%+v facet=%+v", event, facet)
		}
	case err := <-errs:
		t.Fatalf("unexpected stream error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for facet event")
	}

	select {
	case event := <-events:
		tap, ok := event.Payload.(DoubleTapEvent)
		if event.Kind != EventDoubleTap || event.Source != charEvents || !ok || tap.Facet != 4 || !tap.Pause {
			t.Fatalf("unexpected promoted double-tap event: %+v", event)
		}
		if string(event.Raw) != "New Side: 0x84" || string(tap.Raw) != "New Side: 0x84" {
			t.Fatalf("expected raw text to be preserved: event=%+v tap=%+v", event, tap)
		}
	case err := <-errs:
		t.Fatalf("unexpected stream error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for double-tap event")
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
