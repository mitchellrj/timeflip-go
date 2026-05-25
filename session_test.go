package timeflip

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestSession(t *testing.T, conn *fakeConnection) *Session {
	t.Helper()
	return newTestSessionWithRequest(t, conn, ConnectRequest{DeviceID: "tf"})
}

func newTestSessionWithRequest(t *testing.T, conn *fakeConnection, req ConnectRequest) *Session {
	t.Helper()
	client, err := NewClient(&fakeTransport{connections: map[DeviceID]*fakeConnection{"tf": conn}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func withAuthorizationTiming(t *testing.T, window time.Duration, interval time.Duration) {
	t.Helper()
	oldWindow := authorizationResultWindow
	oldInterval := authorizationPollInterval
	authorizationResultWindow = window
	authorizationPollInterval = interval
	t.Cleanup(func() {
		authorizationResultWindow = oldWindow
		authorizationPollInterval = oldInterval
	})
}

func withCommandPollInterval(t *testing.T, interval time.Duration) {
	t.Helper()
	old := commandPollInterval
	commandPollInterval = interval
	t.Cleanup(func() {
		commandPollInterval = old
	})
}

func TestAuthorizeRejectsWrongPassword(t *testing.T) {
	withAuthorizationTiming(t, time.Millisecond, 0)
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x01},
	}})
	_, err := session.Authorize(context.Background(), "000000")
	if !errors.Is(err, ErrAuthorizationFailed) {
		t.Fatalf("expected authorization failure, got %v", err)
	}
}

func TestAuthorizeBlankPasswordUsesDefault(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x02},
	}}
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

func TestAuthorizeRejectsEmptyResult(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {},
	}})
	_, err := session.Authorize(context.Background(), DefaultPassword)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestAuthorizeRejectsMalformedResult(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0xFF},
	}})
	_, err := session.Authorize(context.Background(), DefaultPassword)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestAuthorizeRejectsReadError(t *testing.T) {
	readErr := errors.New("read failed")
	session := newTestSession(t, &fakeConnection{readErrs: map[CharacteristicID]error{
		charCommandResult: readErr,
	}})
	_, err := session.Authorize(context.Background(), DefaultPassword)
	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestAuthorizeWaitsForFreshSuccessAfterStaleWrongResult(t *testing.T) {
	withAuthorizationTiming(t, time.Second, 0)
	conn := &fakeConnection{readSeq: map[CharacteristicID][][]byte{
		charCommandResult: {
			{0x01},
			{0x02},
		},
	}}
	session := newTestSession(t, conn)
	result, err := session.Authorize(context.Background(), DefaultPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authorized {
		t.Fatalf("expected authorization result, got %+v", result)
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

func TestReadDeviceInfoV3UsesAdvertisedName(t *testing.T) {
	session := newTestSessionWithRequest(t, &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charFirmwareRevision: []byte("TFv3.1"),
		},
		readErrs: map[CharacteristicID]error{
			charDeviceName:       errors.New("device name characteristic should not be read for explicit v3"),
			charManufacturerName: ErrProtocol,
			charModelNumber:      ErrProtocol,
			charHardwareRevision: ErrProtocol,
			charSystemID:         ErrProtocol,
		},
	}, ConnectRequest{DeviceID: "tf", AdvertisedName: "TimeFlip Broadcast", ProtocolVersion: ProtocolV3})
	info, err := session.ReadDeviceInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "TimeFlip Broadcast" || info.FirmwareRevision != "TFv3.1" || info.ProtocolVersion != ProtocolV3 {
		t.Fatalf("unexpected v3 info: %+v", info)
	}
	if _, ok := info.Raw[charDeviceName]; ok {
		t.Fatalf("v3 read info should not include a Generic Access device name read: %+v", info.Raw)
	}
}

func TestReadDeviceInfoV3RefreshesAdvertisedName(t *testing.T) {
	conn := &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charFirmwareRevision: []byte("TFv3.1"),
		},
		readErrs: map[CharacteristicID]error{
			charDeviceName:       errors.New("device name characteristic should not be read for explicit v3"),
			charManufacturerName: ErrProtocol,
			charModelNumber:      ErrProtocol,
			charHardwareRevision: ErrProtocol,
			charSystemID:         ErrProtocol,
		},
	}
	transport := &fakeTransport{
		peripherals: []Peripheral{{ID: "tf", Name: "Updated Broadcast"}},
		connections: map[DeviceID]*fakeConnection{"tf": conn},
	}
	client, err := NewClient(transport, Config{})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(context.Background(), ConnectRequest{DeviceID: "tf", AdvertisedName: "Old Broadcast", ProtocolVersion: ProtocolV3})
	if err != nil {
		t.Fatal(err)
	}
	info, err := session.ReadDeviceInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Updated Broadcast" {
		t.Fatalf("expected current advertised name, got %+v", info)
	}
}

func TestReadDeviceInfoUsesAdvertisedNameWhenCharacteristicMissing(t *testing.T) {
	session := newTestSessionWithRequest(t, &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charFirmwareRevision: []byte("FW_v3.47"),
		},
		readErrs: map[CharacteristicID]error{
			charDeviceName:       ErrProtocol,
			charManufacturerName: ErrProtocol,
			charModelNumber:      ErrProtocol,
			charHardwareRevision: ErrProtocol,
			charSystemID:         ErrProtocol,
		},
	}, ConnectRequest{DeviceID: "tf", AdvertisedName: "Broadcast Name"})
	info, err := session.ReadDeviceInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Broadcast Name" {
		t.Fatalf("expected advertised fallback name, got %+v", info)
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
		charCommand:       {byte(cmdReadTask), 0x02},
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
	conn := &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charCommand: {byte(cmdReadTask), 0x02},
		},
		readSeq: map[CharacteristicID][][]byte{charCommandResult: {
			{0x18, 0x00},
			{byte(cmdReadTask), 0x01, 0x02, 0x00, 0x00, 0x05, 0xDC, 0x00, 0x00, 0x00, 0x2A},
		}},
	}
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
		charCommand:       {byte(cmdReadTask), 0x02},
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
		charCommand:       {byte(cmdTapRead), 0x02},
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
	conn := &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charCommand: {byte(cmdTapRead), 0x02},
		},
		readSeq: map[CharacteristicID][][]byte{charCommandResult: {
			{0x18, 0x00},
			{byte(cmdTapRead), 0x3A, 20, 0x3B, 10, 0x3C, 5, 0x3D, 30},
		}},
	}
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
		charCommand:       {byte(cmdTapRead), 0x02},
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
	session.protocol = ProtocolV4
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
	if strings.Contains(err.Error(), "0x0102") || !strings.Contains(err.Error(), "raw payload redacted") {
		t.Fatalf("expected redacted payload in error string, got %q", err.Error())
	}
}

func TestReadHistoryV3UsesCommandOutputCharacteristic(t *testing.T) {
	packet := make([]byte, 21)
	packet[0] = 0x00
	packet[1] = 0x01
	packet[2] = byte(7<<2) | 0x02
	conn := &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charCommand: {byte(cmdHistoryRead), 0x02},
		},
		readSeq: map[CharacteristicID][][]byte{
			charCommandResult: {packet, make([]byte, 21)},
		},
	}
	session := newTestSession(t, conn)
	session.protocol = ProtocolV3
	entries, err := session.ReadHistory(context.Background(), HistoryRequest{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Facet != 7 || entries[0].DurationSeconds != 258 {
		t.Fatalf("unexpected v3 history entries: %+v", entries)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charCommand || string(conn.writes[0].payload) != string([]byte{byte(cmdHistoryRead)}) {
		t.Fatalf("expected v3 history command write, got %+v", conn.writes)
	}
}

func TestReadSystemStateUnsupportedForV3(t *testing.T) {
	session := newTestSession(t, &fakeConnection{})
	session.protocol = ProtocolV3
	_, err := session.ReadSystemState(context.Background())
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported operation, got %v", err)
	}
}

func TestReadAccelerometerUsesV3AccelerometerCharacteristic(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charEvents: {0x00, 0x10, 0xFF, 0xF0, 0x01, 0x00},
	}})
	session.protocol = ProtocolV3
	sample, err := session.ReadAccelerometer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sample.X != 16 || sample.Y != -16 || sample.Z != 256 || string(sample.Raw) != string([]byte{0x00, 0x10, 0xFF, 0xF0, 0x01, 0x00}) || sample.ReadAt.IsZero() {
		t.Fatalf("unexpected accelerometer sample: %+v", sample)
	}
}

func TestReadAccelerometerClassifiesSuccessfulAutoReadAsV3(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charEvents: {0x00, 0x01, 0x00, 0x02, 0x00, 0x03},
	}})
	sample, err := session.ReadAccelerometer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sample.X != 1 || sample.Y != 2 || sample.Z != 3 {
		t.Fatalf("unexpected accelerometer sample: %+v", sample)
	}
	if session.protocol != ProtocolV3 {
		t.Fatalf("expected protocol to be classified as v3, got %q", session.protocol)
	}
}

func TestReadAccelerometerUnsupportedForV4(t *testing.T) {
	session := newTestSession(t, &fakeConnection{})
	session.protocol = ProtocolV4
	_, err := session.ReadAccelerometer(context.Background())
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported operation, got %v", err)
	}
}

func TestAccelerometerSamplesPollsUntilCanceled(t *testing.T) {
	session := newTestSession(t, &fakeConnection{readSeq: map[CharacteristicID][][]byte{
		charEvents: {
			{0x00, 0x01, 0x00, 0x02, 0x00, 0x03},
		},
	}})
	session.protocol = ProtocolV3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples, errs, err := session.AccelerometerSamples(ctx, AccelerometerOptions{Buffer: 1, Interval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case sample := <-samples:
		if sample.X != 1 || sample.Y != 2 || sample.Z != 3 {
			t.Fatalf("unexpected accelerometer sample: %+v", sample)
		}
	case err := <-errs:
		t.Fatalf("unexpected accelerometer stream error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for accelerometer sample")
	}
	cancel()
	select {
	case _, ok := <-samples:
		if ok {
			t.Fatal("samples channel still open")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for accelerometer stream close")
	}
}

func TestAccelerometerSamplesRejectsInvalidOptions(t *testing.T) {
	session := newTestSession(t, &fakeConnection{})
	session.protocol = ProtocolV3
	_, _, err := session.AccelerometerSamples(context.Background(), AccelerometerOptions{Buffer: -1})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	_, _, err = session.AccelerometerSamples(context.Background(), AccelerometerOptions{Interval: -time.Millisecond})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestAccelerometerSamplesUnsupportedForV4(t *testing.T) {
	session := newTestSession(t, &fakeConnection{})
	session.protocol = ProtocolV4
	_, _, err := session.AccelerometerSamples(context.Background(), AccelerometerOptions{})
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported operation, got %v", err)
	}
}

func TestReadTrackerStatusUsesCommandOutput(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand:       {byte(cmdStatus), 0x02},
		charCommandResult: {0x02, 0x01, 0x00, 0x0F},
		charFacets:        {0x08},
	}}
	session := newTestSession(t, conn)
	status, err := session.ReadTrackerStatus(context.Background(), CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if status.LockEnabled || !status.PauseEnabled || status.AutoPauseMinutes != 15 || !status.CurrentFacetKnown || status.CurrentFacet != 8 {
		t.Fatalf("unexpected tracker status: %+v", status)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charCommand || string(conn.writes[0].payload) != string([]byte{byte(cmdStatus)}) {
		t.Fatalf("unexpected status command write: %+v", conn.writes)
	}
}

func TestReadTrackerStatusWaitsForDecodablePayload(t *testing.T) {
	conn := &fakeConnection{
		reads: map[CharacteristicID][]byte{
			charCommand: {byte(cmdStatus), 0x02},
			charFacets:  {0x03},
		},
		readSeq: map[CharacteristicID][][]byte{charCommandResult: {
			{byte(cmdReadTask), 0x01, 0x02, 0x03},
			{0x01, 0x02, 0x00, 0x00},
		}},
	}
	session := newTestSession(t, conn)
	status, err := session.ReadTrackerStatus(context.Background(), CommandOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !status.LockEnabled || status.PauseEnabled || status.AutoPauseMinutes != 0 || !status.CurrentFacetKnown || status.CurrentFacet != 3 {
		t.Fatalf("unexpected tracker status: %+v", status)
	}
}

func TestSendCommandRejected(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {byte(cmdLock), 0x01},
	}})
	_, err := session.SetLock(context.Background(), true, CommandOptions{})
	if !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("expected command rejected, got %v", err)
	}
}

func TestSendCommandWaitsForMatchingCommandAcknowledgement(t *testing.T) {
	session := newTestSession(t, &fakeConnection{readSeq: map[CharacteristicID][][]byte{
		charCommand: {
			{0x18, 0x02},
			{byte(cmdName), 0x02},
		},
	}})
	result, err := session.SetName(context.Background(), "test", CommandOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Status.OK || result.Status.Code != cmdName {
		t.Fatalf("unexpected command result: %+v", result)
	}
}

func TestSendCommandIgnoresTrailingCommandCharacteristicBytes(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {byte(cmdName), 0x02, 0xAA, 0xBB, 0x00, 0x00, 0x00, 0x01},
	}})
	result, err := session.SetName(context.Background(), "test", CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Status.OK || result.Status.Code != cmdName {
		t.Fatalf("unexpected command result: %+v", result)
	}
	if string(result.Status.Raw) != string([]byte{byte(cmdName), 0x02, 0xAA, 0xBB}) {
		t.Fatalf("expected first four acknowledgement bytes only, got 0x%X", result.Status.Raw)
	}
	if string(result.Payload) != string(result.Status.Raw) {
		t.Fatalf("expected payload to mirror trimmed acknowledgement, got payload=0x%X raw=0x%X", result.Payload, result.Status.Raw)
	}
}

func TestSendCommandTrimsNULTerminatedCommandAcknowledgement(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {byte(cmdName), 0x02, 0x00, 0xBB, 0xCC},
	}})
	result, err := session.SetName(context.Background(), "test", CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Status.Raw) != string([]byte{byte(cmdName), 0x02}) {
		t.Fatalf("expected acknowledgement to stop at NUL terminator, got 0x%X", result.Status.Raw)
	}
}

func TestSendCommandMalformedStatusPreservesPayload(t *testing.T) {
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {byte(cmdName), 0x00},
	}})
	result, err := session.SetName(context.Background(), "Desk Timer", CommandOptions{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	if string(result.Payload) != string([]byte{byte(cmdName), 0x00}) || string(result.Status.Raw) != string([]byte{byte(cmdName), 0x00}) {
		t.Fatalf("expected raw malformed command payload to be preserved, got result=%+v", result)
	}
}

func TestSendCommandStopsAfterStatusPollBudget(t *testing.T) {
	withCommandPollInterval(t, 0)
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {0x18, 0x02},
	}})
	_, err := session.SetName(context.Background(), "test", CommandOptions{Timeout: time.Second})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestReadCommandOutputStopsAfterPollBudget(t *testing.T) {
	withCommandPollInterval(t, 0)
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand:       {byte(cmdReadTask), 0x02},
		charCommandResult: {byte(cmdName), 0x02},
	}})
	_, err := session.ReadTaskParameters(context.Background(), 1, CommandOptions{Timeout: time.Second})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestReadHistoryV3StopsAfterPacketBudget(t *testing.T) {
	packet := make([]byte, 21)
	packet[0] = 0x00
	packet[1] = 0x01
	packet[2] = byte(7<<2) | 0x02
	session := newTestSession(t, &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand:       {byte(cmdHistoryRead), 0x02},
		charCommandResult: packet,
	}})
	session.protocol = ProtocolV3
	_, err := session.ReadHistory(context.Background(), HistoryRequest{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestSetAutoPauseUsesTimeFlipBigEndianForV3(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommand: {byte(cmdAutoPause), 0x02},
	}}
	session := newTestSession(t, conn)
	session.protocol = ProtocolV3
	_, err := session.SetAutoPause(context.Background(), 0x1234, CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.writes) != 1 || string(conn.writes[0].payload) != string([]byte{byte(cmdAutoPause), 0x12, 0x34}) {
		t.Fatalf("expected v3 big-endian auto-pause write, got %+v", conn.writes)
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
		facet := event.Payload.(FacetEvent)
		if len(event.Raw) != 0 || len(facet.Raw) != 0 {
			t.Fatalf("expected raw facet bytes to be omitted by default, got event=%+v facet=%+v", event, facet)
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

func TestEventsDefaultLeavesHistoryAvailableForReadHistory(t *testing.T) {
	history := make([]byte, 17)
	history[4] = 3
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charHistory: history,
	}}
	session := newTestSession(t, conn)
	session.protocol = ProtocolV4
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, err := session.Events(ctx, EventOptions{Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := conn.subscriptions[charHistory]; ok {
		t.Fatal("history characteristic should not be subscribed by default")
	}
	entries, err := session.ReadHistory(context.Background(), HistoryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Facet != 3 {
		t.Fatalf("unexpected history entries: %+v", entries)
	}
	if len(conn.writes) != 1 || conn.writes[0].characteristic != charHistory {
		t.Fatalf("expected history read write, got %+v", conn.writes)
	}
}

func TestEventsCanOptIntoHistoryNotifications(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, err := session.Events(ctx, EventOptions{Buffer: 1, IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := conn.subscriptions[charHistory]; !ok {
		t.Fatal("history characteristic was not subscribed with IncludeHistory")
	}
}

func TestEventsIncludeRawPreservesTypedEventRaw(t *testing.T) {
	conn := &fakeConnection{}
	session := newTestSession(t, conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, errs, err := session.Events(ctx, EventOptions{Buffer: 1, IncludeRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	conn.subscriptions[charFacets] <- Notification{Characteristic: charFacets, Payload: []byte{3}}
	select {
	case event := <-events:
		facet, ok := event.Payload.(FacetEvent)
		if event.Kind != EventFacet || !ok || string(event.Raw) != string([]byte{3}) || string(facet.Raw) != string([]byte{3}) {
			t.Fatalf("expected raw facet bytes with IncludeRaw, got event=%+v facet=%+v", event, facet)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
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
		if string(event.Raw) != string([]byte{0xAA, 0x01}) {
			t.Fatalf("expected EventRaw to carry raw bytes, got %+v", event)
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
		if len(event.Raw) != 0 || len(facet.Raw) != 0 {
			t.Fatalf("expected raw text to be omitted by default: event=%+v facet=%+v", event, facet)
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
		if len(event.Raw) != 0 || len(tap.Raw) != 0 {
			t.Fatalf("expected raw text to be omitted by default: event=%+v tap=%+v", event, tap)
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
