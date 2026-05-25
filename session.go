package timeflip

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Session manages one active TimeFlip2 connection.
type Session struct {
	deviceID             DeviceID
	advertisedName       string
	advertisedNameReader func(context.Context) (string, bool)
	conn                 Connection
	defaultTimeout       time.Duration
	protocol             ProtocolVersion
	opMu                 sync.Mutex
	closeOnce            sync.Once
	done                 chan struct{}
}

const (
	maxCommandStatusPolls = 256
	maxCommandOutputPolls = 256
	maxHistoryV3Packets   = 4096
)

var (
	authorizationResultWindow = 750 * time.Millisecond
	authorizationPollInterval = 50 * time.Millisecond
	commandPollInterval       = 50 * time.Millisecond
	accelerometerPollInterval = 100 * time.Millisecond
)

// Authorize writes a six-byte password to the device password characteristic.
func (s *Session) Authorize(ctx context.Context, password string) (AuthorizationResult, error) {
	password, err := passwordOrDefault(password)
	if err != nil {
		return AuthorizationResult{}, &OperationError{Operation: "authorize", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	if err := s.conn.Write(ctx, charPassword, []byte(password)); err != nil {
		return AuthorizationResult{}, wrapContextErr("authorize", s.deviceID, "", 0, err)
	}
	return s.readAuthorizationResult(ctx)
}

func passwordOrDefault(password string) (string, error) {
	if password == "" {
		return DefaultPassword, nil
	}
	if len(password) != 6 {
		return "", ErrInvalidInput
	}
	return password, nil
}

func (s *Session) readAuthorizationResult(ctx context.Context) (AuthorizationResult, error) {
	deadline := time.NewTimer(authorizationResultWindow)
	defer deadline.Stop()
	for {
		payload, err := s.conn.Read(ctx, charCommandResult)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return AuthorizationResult{}, wrapContextErr("authorize", s.deviceID, "", 0, ctxErr)
			}
			return AuthorizationResult{}, wrapContextErr("authorize", s.deviceID, "", 0, err)
		}
		if len(payload) == 0 {
			return AuthorizationResult{}, &OperationError{Operation: "authorize", DeviceID: s.deviceID, Err: newProtocolPayloadError("authorization result 0x02 success or 0x01 failure", payload)}
		}
		switch payload[0] {
		case 0x02:
			return AuthorizationResult{Authorized: true}, nil
		case 0x01:
		default:
			return AuthorizationResult{}, &OperationError{Operation: "authorize", DeviceID: s.deviceID, Err: newProtocolPayloadError("authorization result 0x02 success or 0x01 failure", payload)}
		}

		timer := time.NewTimer(authorizationPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AuthorizationResult{}, wrapContextErr("authorize", s.deviceID, "", 0, ctx.Err())
		case <-deadline.C:
			timer.Stop()
			return AuthorizationResult{}, &OperationError{
				Operation: "authorize",
				DeviceID:  s.deviceID,
				Err:       fmt.Errorf("%w: password check returned failure result", ErrAuthorizationFailed),
			}
		case <-timer.C:
		}
	}
}

func (s *Session) verifyUsable(ctx context.Context) error {
	if s.protocol != ProtocolV3 {
		if _, err := s.ReadSystemState(ctx); err == nil {
			return nil
		} else if s.protocol == ProtocolV4 {
			return err
		}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	if _, err := s.conn.Read(ctx, charFacets); err != nil {
		return wrapContextErr("verify", s.deviceID, string(PairingStageVerify), 0, err)
	}
	return nil
}

// Close closes the session connection.
func (s *Session) Close(ctx context.Context) error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.conn.Close(ctx)
	})
	return err
}

// ReadDeviceInfo reads standard BLE device information characteristics.
func (s *Session) ReadDeviceInfo(ctx context.Context) (DeviceInfo, error) {
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	values := map[CharacteristicID][]byte{}
	var firstMissingErr error
	characteristics := []CharacteristicID{charManufacturerName, charModelNumber, charHardwareRevision, charFirmwareRevision, charSystemID}
	if s.protocol != ProtocolV3 {
		characteristics = append([]CharacteristicID{charDeviceName}, characteristics...)
	}
	for _, ch := range characteristics {
		payload, err := s.conn.Read(ctx, ch)
		if err != nil {
			if errors.Is(err, ErrProtocol) {
				if firstMissingErr == nil {
					firstMissingErr = wrapContextErr("read_device_info", s.deviceID, "", 0, err)
				}
				continue
			}
			return DeviceInfo{}, wrapContextErr("read_device_info", s.deviceID, "", 0, err)
		}
		values[ch] = payload
	}
	if len(values) == 0 {
		if firstMissingErr != nil {
			return DeviceInfo{}, firstMissingErr
		}
		return DeviceInfo{}, &OperationError{Operation: "read_device_info", DeviceID: s.deviceID, Err: ErrProtocol}
	}
	info := decodeDeviceInfo(values)
	if s.protocol == ProtocolV3 || info.ProtocolVersion == ProtocolV3 || info.Name == "" {
		if name := s.readAdvertisedName(ctx); name != "" {
			info.Name = name
		}
	}
	if s.protocol == ProtocolAuto && info.ProtocolVersion != ProtocolAuto {
		s.protocol = info.ProtocolVersion
	}
	return info, nil
}

func (s *Session) readAdvertisedName(ctx context.Context) string {
	if s.advertisedNameReader != nil {
		if name, ok := s.advertisedNameReader(ctx); ok {
			return name
		}
	}
	return s.advertisedName
}

// ReadBattery reads current battery status.
func (s *Session) ReadBattery(ctx context.Context) (BatteryStatus, error) {
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	payload, err := s.conn.Read(ctx, charBattery)
	if err != nil {
		return BatteryStatus{}, wrapContextErr("read_battery", s.deviceID, "", 0, err)
	}
	battery, err := decodeBattery(payload)
	if err != nil {
		return BatteryStatus{}, &OperationError{Operation: "read_battery", DeviceID: s.deviceID, Err: err}
	}
	return battery, nil
}

// ReadSystemState reads current TimeFlip2 system state.
func (s *Session) ReadSystemState(ctx context.Context) (SystemState, error) {
	if s.protocol == ProtocolV3 {
		return SystemState{}, &OperationError{Operation: "read_system_state", DeviceID: s.deviceID, Err: ErrUnsupportedOperation}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	payload, err := s.conn.Read(ctx, charSystemState)
	if err != nil {
		return SystemState{}, wrapContextErr("read_system_state", s.deviceID, "", 0, err)
	}
	state, err := decodeSystemState(payload)
	if err != nil {
		return SystemState{}, &OperationError{Operation: "read_system_state", DeviceID: s.deviceID, Err: err}
	}
	return state, nil
}

// ReadAccelerometer reads one raw accelerometer vector sample where the protocol exposes it.
func (s *Session) ReadAccelerometer(ctx context.Context) (AccelerometerSample, error) {
	if s.protocol == ProtocolV4 {
		return AccelerometerSample{}, &OperationError{Operation: "read_accelerometer", DeviceID: s.deviceID, Err: ErrUnsupportedOperation}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	payload, err := s.conn.Read(ctx, charEvents)
	if err != nil {
		return AccelerometerSample{}, wrapContextErr("read_accelerometer", s.deviceID, "", 0, err)
	}
	sample, err := decodeAccelerometer(payload)
	if err != nil {
		return AccelerometerSample{}, &OperationError{Operation: "read_accelerometer", DeviceID: s.deviceID, Err: newProtocolPayloadError("6-byte v3 accelerometer vector from accelerometer data characteristic", payload)}
	}
	sample.ReadAt = time.Now()
	if s.protocol == ProtocolAuto {
		s.protocol = ProtocolV3
	}
	return sample, nil
}

// AccelerometerSamples polls raw accelerometer vector samples until ctx is cancelled.
func (s *Session) AccelerometerSamples(ctx context.Context, opts AccelerometerOptions) (<-chan AccelerometerSample, <-chan error, error) {
	if opts.Buffer < 0 || opts.Interval < 0 {
		return nil, nil, &OperationError{Operation: "accelerometer_samples", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	if s.protocol == ProtocolV4 {
		return nil, nil, &OperationError{Operation: "accelerometer_samples", DeviceID: s.deviceID, Err: ErrUnsupportedOperation}
	}
	interval := opts.Interval
	if interval == 0 {
		interval = accelerometerPollInterval
	}
	samples := make(chan AccelerometerSample, opts.Buffer)
	errs := make(chan error, 1)
	go func() {
		defer close(samples)
		defer close(errs)
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.done:
				return
			case <-timer.C:
			}
			sample, err := s.ReadAccelerometer(ctx)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
			} else {
				select {
				case samples <- sample:
				case <-ctx.Done():
					return
				case <-s.done:
					return
				}
			}
			timer.Reset(interval)
		}
	}()
	return samples, errs, nil
}

// ReadTrackerStatus reads lock, pause, and auto-pause state.
func (s *Session) ReadTrackerStatus(ctx context.Context, opts CommandOptions) (TrackerStatus, error) {
	payload, err := s.readCommandPayload(ctx, commandNoPayload(cmdStatus), opts, "read_tracker_status", isTrackerStatusPayload)
	if err != nil {
		return TrackerStatus{}, err
	}
	status, err := decodeTrackerStatus(payload)
	if err != nil {
		return TrackerStatus{}, &OperationError{Operation: "read_tracker_status", DeviceID: s.deviceID, Command: cmdStatus, Err: newProtocolPayloadError("tracker status response lock/pause/autopause bytes", payload)}
	}
	facetPayload, err := s.conn.Read(ctx, charFacets)
	if err == nil {
		facet, err := decodeFacet(facetPayload)
		if err == nil {
			status.CurrentFacetKnown = true
			status.CurrentFacet = facet.Facet
			status.CurrentFacetUndefined = facet.Undefined
			status.FacetRaw = append([]byte(nil), facetPayload...)
		}
	}
	return status, nil
}

// ReadHistory reads device history entries.
func (s *Session) ReadHistory(ctx context.Context, req HistoryRequest) ([]HistoryEntry, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.readHistory(ctx, req)
}

func (s *Session) readHistory(ctx context.Context, req HistoryRequest) ([]HistoryEntry, error) {
	if s.protocol == ProtocolV3 {
		return s.readHistoryV3(ctx, req)
	}
	entries, err := s.readHistoryV4(ctx, req)
	if err == nil || s.protocol == ProtocolV4 {
		return entries, err
	}
	return s.readHistoryV3(ctx, req)
}

func (s *Session) readHistoryV4(ctx context.Context, req HistoryRequest) ([]HistoryEntry, error) {
	code := byte(0x01)
	if req.All {
		code = 0x02
	}
	payload := make([]byte, 5)
	payload[0] = code
	binary.BigEndian.PutUint32(payload[1:5], req.StartEvent)
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	if err := s.conn.Write(ctx, charHistory, payload); err != nil {
		return nil, wrapContextErr("read_history", s.deviceID, "", 0, err)
	}
	raw, err := s.conn.Read(ctx, charHistory)
	if err != nil {
		return nil, wrapContextErr("read_history", s.deviceID, "", 0, err)
	}
	entries, _, err := decodeHistory(raw)
	if err != nil {
		return nil, &OperationError{Operation: "read_history", DeviceID: s.deviceID, Err: newProtocolPayloadError("17-byte single history record or 20-byte history stream packet from history data characteristic", raw)}
	}
	return entries, nil
}

func (s *Session) readHistoryV3(ctx context.Context, _ HistoryRequest) ([]HistoryEntry, error) {
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	if err := s.conn.Write(ctx, charCommand, []byte{byte(cmdHistoryRead)}); err != nil {
		return nil, wrapContextErr("read_history", s.deviceID, "", cmdHistoryRead, err)
	}
	status, err := s.readCommandStatusFor(ctx, cmdHistoryRead, "read_history")
	if err != nil {
		return nil, err
	}
	if !status.OK {
		return nil, &OperationError{Operation: "read_history", DeviceID: s.deviceID, Command: cmdHistoryRead, Err: ErrCommandRejected}
	}
	var entries []HistoryEntry
	var lastNonZero []byte
	for packets := 0; packets < maxHistoryV3Packets; packets++ {
		raw, err := s.conn.Read(ctx, charCommandResult)
		if err != nil {
			return nil, wrapContextErr("read_history", s.deviceID, "", cmdHistoryRead, err)
		}
		decoded, stream, err := decodeHistoryV3(raw)
		if err != nil {
			return nil, &OperationError{Operation: "read_history", DeviceID: s.deviceID, Err: newProtocolPayloadError("21-byte v3 history package from command-result output characteristic", raw)}
		}
		if stream.Complete {
			if len(lastNonZero) >= 2 {
				count := int(binary.BigEndian.Uint16(lastNonZero[0:2]))
				if count > 0 && count < len(entries) {
					return entries[:count], nil
				}
			}
			return entries, nil
		}
		lastNonZero = append(lastNonZero[:0], raw...)
		entries = append(entries, decoded...)
	}
	return nil, &OperationError{Operation: "read_history", DeviceID: s.deviceID, Command: cmdHistoryRead, Err: newProtocolPayloadError("v3 history completion packet within packet budget", lastNonZero)}
}

// ReadTaskParameters reads task parameters for a facet.
func (s *Session) ReadTaskParameters(ctx context.Context, facet FacetID, opts CommandOptions) (TaskParameters, error) {
	payload, err := s.readCommandPayload(ctx, Command{Code: cmdReadTask, Payload: []byte{byte(facet)}}, opts, "read_task_parameters", isUnassignedCommandResult)
	if err != nil {
		return TaskParameters{}, err
	}
	if isUnassignedCommandResult(payload) {
		return TaskParameters{Facet: facet, Assigned: false, Raw: append([]byte(nil), payload...)}, nil
	}
	if len(payload) < 11 || CommandCode(payload[0]) != cmdReadTask {
		return TaskParameters{}, &OperationError{Operation: "read_task_parameters", DeviceID: s.deviceID, Command: cmdReadTask, Err: newProtocolPayloadError("task response 0x14 plus 10 data bytes", payload)}
	}
	return TaskParameters{
		Facet:                FacetID(payload[1]),
		Assigned:             true,
		Mode:                 payload[2],
		PomodoroLimitSeconds: binary.BigEndian.Uint32(payload[3:7]),
		ElapsedSeconds:       binary.BigEndian.Uint32(payload[7:11]),
		Raw:                  append([]byte(nil), payload...),
	}, nil
}

// ReadTapSettings reads double-tap accelerometer settings.
func (s *Session) ReadTapSettings(ctx context.Context, opts CommandOptions) (TapSettings, error) {
	payload, err := s.readCommandPayload(ctx, commandNoPayload(cmdTapRead), opts, "read_tap_settings", isUnassignedCommandResult)
	if err != nil {
		return TapSettings{}, err
	}
	if isUnassignedCommandResult(payload) {
		return TapSettings{Configured: false, Raw: append([]byte(nil), payload...)}, nil
	}
	settings, err := tapSettingsFromPayload(payload)
	if err != nil {
		return TapSettings{}, &OperationError{Operation: "read_tap_settings", DeviceID: s.deviceID, Command: cmdTapRead, Err: err}
	}
	return settings, nil
}

func (s *Session) readCommandPayload(ctx context.Context, cmd Command, opts CommandOptions, operation string, accept func([]byte) bool) ([]byte, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.readCommandPayloadLocked(ctx, cmd, opts, operation, accept)
}

func (s *Session) readCommandPayloadLocked(ctx context.Context, cmd Command, opts CommandOptions, operation string, accept func([]byte) bool) ([]byte, error) {
	encoded, err := encodeCommand(cmd)
	if err != nil {
		return nil, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: cmd.Code, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, opts.Timeout)
	defer cancel()
	if err := s.conn.Write(ctx, charCommand, encoded); err != nil {
		return nil, wrapContextErr(operation, s.deviceID, "", cmd.Code, err)
	}
	status, err := s.readCommandStatusFor(ctx, cmd.Code, operation)
	if err != nil {
		return nil, err
	}
	if !status.OK {
		return nil, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: cmd.Code, Err: ErrCommandRejected}
	}
	payload, err := s.readCommandOutputFor(ctx, cmd.Code, operation, accept)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// SendCommand writes a supported command and reads the command result.
func (s *Session) SendCommand(ctx context.Context, cmd Command, opts CommandOptions) (CommandResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.sendCommandLocked(ctx, cmd, opts)
}

func (s *Session) sendCommandLocked(ctx context.Context, cmd Command, opts CommandOptions) (CommandResult, error) {
	encoded, err := encodeCommand(cmd)
	if err != nil {
		return CommandResult{}, &OperationError{Operation: "send_command", DeviceID: s.deviceID, Command: cmd.Code, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, opts.Timeout)
	defer cancel()
	if err := s.conn.Write(ctx, charCommand, encoded); err != nil {
		return CommandResult{}, wrapContextErr("send_command", s.deviceID, "", cmd.Code, err)
	}
	status, err := s.readCommandStatusFor(ctx, cmd.Code, "send_command")
	if err != nil {
		return CommandResult{Command: cmd, Status: status, Payload: append([]byte(nil), status.Raw...)}, err
	}
	result := CommandResult{Command: cmd, Status: status, Payload: append([]byte(nil), status.Raw...)}
	if !status.OK {
		return result, &OperationError{Operation: "send_command", DeviceID: s.deviceID, Command: cmd.Code, Err: ErrCommandRejected}
	}
	return result, nil
}

func (s *Session) readCommandStatusFor(ctx context.Context, code CommandCode, operation string) (CommandStatus, error) {
	var last []byte
	for polls := 0; polls < maxCommandStatusPolls; polls++ {
		payload, err := s.conn.Read(ctx, charCommand)
		if err != nil {
			return CommandStatus{}, wrapContextErr(operation, s.deviceID, "", code, err)
		}
		last = append(last[:0], payload...)
		if len(payload) > 0 && CommandCode(payload[0]) == code {
			status, err := decodeCommandStatus(payload)
			if err != nil {
				return CommandStatus{Code: code, Raw: append([]byte(nil), payload...)}, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: code, Err: err}
			}
			return status, nil
		}
		timer := time.NewTimer(commandPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return CommandStatus{Code: code, Raw: append([]byte(nil), last...)}, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: code, Err: newProtocolPayloadError("command characteristic acknowledgement beginning with requested command byte", last)}
		case <-timer.C:
		}
	}
	return CommandStatus{Code: code, Raw: append([]byte(nil), last...)}, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: code, Err: newProtocolPayloadError("command characteristic acknowledgement beginning with requested command byte within poll budget", last)}
}

func (s *Session) readCommandOutputFor(ctx context.Context, code CommandCode, operation string, accept func([]byte) bool) ([]byte, error) {
	var last []byte
	for polls := 0; polls < maxCommandOutputPolls; polls++ {
		payload, err := s.conn.Read(ctx, charCommandResult)
		if err != nil {
			return nil, wrapContextErr(operation, s.deviceID, "", code, err)
		}
		last = append(last[:0], payload...)
		if len(payload) > 0 && CommandCode(payload[0]) == code {
			return append([]byte(nil), payload...), nil
		}
		if isPasswordWrongResult(payload) {
			return nil, &OperationError{
				Operation: operation,
				DeviceID:  s.deviceID,
				Command:   code,
				Err:       fmt.Errorf("%w: command-result characteristic reports password check failed; authorize with the current password before sending commands", ErrAuthorizationFailed),
			}
		}
		if accept != nil && accept(payload) {
			return append([]byte(nil), payload...), nil
		}
		timer := time.NewTimer(commandPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: code, Err: newProtocolPayloadError("command-result output beginning with requested command byte", last)}
		case <-timer.C:
		}
	}
	return nil, &OperationError{Operation: operation, DeviceID: s.deviceID, Command: code, Err: newProtocolPayloadError("command-result output beginning with requested command byte within poll budget", last)}
}

func isUnassignedCommandResult(payload []byte) bool {
	return len(payload) == 2 && payload[0] == 0x19 && payload[1] == 0x00
}

func isTrackerStatusPayload(payload []byte) bool {
	_, err := decodeTrackerStatus(payload)
	return err == nil
}

func isPasswordWrongResult(payload []byte) bool {
	return len(payload) == 1 && payload[0] == 0x01
}

// SetPassword sets a new six-byte password.
func (s *Session) SetPassword(ctx context.Context, password string, opts CommandOptions) (CommandResult, error) {
	if len(password) != 6 {
		return CommandResult{}, &OperationError{Operation: "set_password", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	return s.SendCommand(ctx, Command{Code: cmdSetPassword, Payload: []byte(password)}, opts)
}

// SetName sets the device name.
func (s *Session) SetName(ctx context.Context, name string, opts CommandOptions) (CommandResult, error) {
	maxLen := 18
	if s.protocol == ProtocolV3 {
		maxLen = 19
	}
	if len(name) == 0 || len(name) > maxLen {
		return CommandResult{}, &OperationError{Operation: "set_name", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	return s.SendCommand(ctx, Command{Code: cmdName, Payload: append([]byte{byte(len(name))}, []byte(name)...)}, opts)
}

// SetLock enables or disables lock mode.
func (s *Session) SetLock(ctx context.Context, enabled bool, opts CommandOptions) (CommandResult, error) {
	return s.SendCommand(ctx, commandBool(cmdLock, enabled), opts)
}

// SetPause enables or disables pause mode.
func (s *Session) SetPause(ctx context.Context, enabled bool, opts CommandOptions) (CommandResult, error) {
	return s.SendCommand(ctx, commandBool(cmdPause, enabled), opts)
}

// SetAutoPause configures auto-pause delay in minutes.
func (s *Session) SetAutoPause(ctx context.Context, delayMinutes uint16, opts CommandOptions) (CommandResult, error) {
	return s.SendCommand(ctx, commandUint16(cmdAutoPause, delayMinutes), opts)
}

// SetLED configures LED brightness and blink period.
func (s *Session) SetLED(ctx context.Context, brightnessPercent uint8, blinkSeconds uint8, opts CommandOptions) (CommandResult, error) {
	if brightnessPercent < 1 || brightnessPercent > 100 || blinkSeconds < 5 || blinkSeconds > 60 {
		return CommandResult{}, &OperationError{Operation: "set_led", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	if _, err := s.SendCommand(ctx, Command{Code: cmdBrightness, Payload: []byte{brightnessPercent}}, opts); err != nil {
		return CommandResult{}, err
	}
	return s.SendCommand(ctx, Command{Code: cmdBlinkPeriod, Payload: []byte{blinkSeconds}}, opts)
}

// SetFacetColor configures a facet color.
func (s *Session) SetFacetColor(ctx context.Context, facet FacetID, color RGB, opts CommandOptions) (CommandResult, error) {
	payload := []byte{byte(facet), byte(color.R >> 8), byte(color.R), byte(color.G >> 8), byte(color.G), byte(color.B >> 8), byte(color.B)}
	return s.SendCommand(ctx, Command{Code: cmdFacetColor, Payload: payload}, opts)
}

// SetTaskParameters configures task parameters for a facet.
func (s *Session) SetTaskParameters(ctx context.Context, params TaskParameters, opts CommandOptions) (CommandResult, error) {
	payload := make([]byte, 6)
	payload[0] = byte(params.Facet)
	payload[1] = params.Mode
	binary.BigEndian.PutUint32(payload[2:6], params.PomodoroLimitSeconds)
	return s.SendCommand(ctx, Command{Code: cmdTaskParameters, Payload: payload}, opts)
}

// SetTapSettings configures double-tap accelerometer settings.
func (s *Session) SetTapSettings(ctx context.Context, settings TapSettings, opts CommandOptions) (CommandResult, error) {
	payload := []byte{0x3A, settings.Threshold, 0x3B, settings.Limit, 0x3C, settings.Latency, 0x3D, settings.Window}
	return s.SendCommand(ctx, Command{Code: cmdTapWrite, Payload: payload}, opts)
}

// ResetTaskInfo resets task information to defaults.
func (s *Session) ResetTaskInfo(ctx context.Context, opts CommandOptions) (CommandResult, error) {
	return s.SendCommand(ctx, commandNoPayload(cmdResetTaskInfo), opts)
}

// FactoryReset erases device flash and resets the tracker to factory settings.
func (s *Session) FactoryReset(ctx context.Context, opts CommandOptions) (CommandResult, error) {
	return s.SendCommand(ctx, commandNoPayload(cmdFactoryReset), opts)
}

// Events subscribes to device notifications and emits typed technical events.
func (s *Session) Events(ctx context.Context, opts EventOptions) (<-chan Event, <-chan error, error) {
	if opts.Buffer < 0 {
		return nil, nil, &OperationError{Operation: "events", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	subs := eventSubscriptionCharacteristics(opts)
	streams := make([]<-chan Notification, 0, len(subs))
	s.opMu.Lock()
	defer s.opMu.Unlock()
	for _, ch := range subs {
		stream, err := s.conn.Subscribe(ctx, ch)
		if err != nil {
			return nil, nil, wrapContextErr("events", s.deviceID, "", 0, err)
		}
		streams = append(streams, stream)
	}
	events := make(chan Event, opts.Buffer)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	outCtx, cancel := context.WithCancel(ctx)
	for _, stream := range streams {
		wg.Add(1)
		go func(stream <-chan Notification) {
			defer wg.Done()
			for {
				select {
				case <-outCtx.Done():
					return
				case <-s.done:
					return
				case n, ok := <-stream:
					if !ok {
						return
					}
					event, err := s.decodeNotification(n, opts.IncludeRaw)
					if err != nil {
						select {
						case errs <- err:
						default:
						}
						continue
					}
					select {
					case events <- event:
					case <-outCtx.Done():
						return
					case <-s.done:
						return
					}
				}
			}
		}(stream)
	}
	go func() {
		<-outCtx.Done()
		cancel()
		wg.Wait()
		close(events)
		close(errs)
	}()
	return events, errs, nil
}

func eventSubscriptionCharacteristics(opts EventOptions) []CharacteristicID {
	subs := []CharacteristicID{charFacets, charDoubleTap, charBattery, charSystemState, charEvents}
	if opts.IncludeHistory {
		subs = append(subs, charHistory)
	}
	return subs
}

func (s *Session) decodeNotification(n Notification, includeRaw bool) (Event, error) {
	event := Event{DeviceID: s.deviceID, Source: n.Characteristic, ReceivedAt: time.Now()}
	if includeRaw {
		event.Raw = append([]byte(nil), n.Payload...)
	}
	switch n.Characteristic {
	case charFacets:
		payload, err := decodeFacet(n.Payload)
		if !includeRaw {
			payload.Raw = nil
		}
		event.Kind, event.Payload = EventFacet, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charDoubleTap:
		payload, err := decodeDoubleTap(n.Payload)
		if !includeRaw {
			payload.Raw = nil
		}
		event.Kind, event.Payload = EventDoubleTap, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charBattery:
		payload, err := decodeBattery(n.Payload)
		if !includeRaw {
			payload.Raw = nil
		}
		event.Kind, event.Payload = EventBattery, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charSystemState:
		payload, err := decodeSystemState(n.Payload)
		if !includeRaw {
			payload.Raw = nil
		}
		event.Kind, event.Payload = EventSystemState, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charEvents:
		payload := append([]byte(nil), n.Payload...)
		if kind, decoded, ok := decodeTimeFlipTextEvent(payload, includeRaw); ok {
			event.Kind, event.Payload = kind, decoded
		} else {
			event.Kind, event.Payload = EventRaw, payload
		}
		if event.Kind == EventRaw && len(event.Raw) == 0 {
			event.Raw = payload
		}
		return event, nil
	case charHistory:
		payload, _, err := decodeHistory(n.Payload)
		if !includeRaw {
			for i := range payload {
				payload[i].Raw = nil
			}
		}
		event.Kind, event.Payload = EventHistory, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	default:
		if includeRaw {
			event.Kind, event.Payload = EventRaw, append([]byte(nil), n.Payload...)
			return event, nil
		}
		return Event{}, &OperationError{Operation: "events", DeviceID: s.deviceID, Stage: NotificationSourceName(n.Characteristic), Err: ErrProtocol}
	}
}

func decodeTimeFlipTextEvent(payload []byte, includeRaw bool) (EventKind, any, bool) {
	text := cleanString(payload)
	if paused, ok := parsePauseStateEventText(text); ok {
		var raw []byte
		if includeRaw {
			raw = append([]byte(nil), payload...)
		}
		return EventPauseState, PauseStateEvent{Paused: paused, Raw: raw}, true
	}
	side, tap, ok := parseSideEventText(text)
	if !ok {
		return "", nil, false
	}
	var raw []byte
	if includeRaw {
		raw = append([]byte(nil), payload...)
	}
	if tap || side >= 128 {
		pause := side >= 128
		if pause {
			side -= 128
		}
		return EventDoubleTap, DoubleTapEvent{Facet: FacetID(side), Pause: pause, Raw: raw}, true
	}
	return EventFacet, FacetEvent{Facet: FacetID(side), Undefined: side == 0, WrongPassword: side == 0, Raw: raw}, true
}

func parsePauseStateEventText(text string) (paused bool, ok bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch normalized {
	case "pause on", "pause: on":
		return true, true
	case "pause off", "pause: off":
		return false, true
	default:
		return false, false
	}
}

func parseSideEventText(text string) (side uint64, tap bool, ok bool) {
	text = strings.TrimSpace(text)
	normalized := strings.ToLower(text)
	for _, candidate := range []struct {
		prefix string
		tap    bool
	}{
		{prefix: "new side:"},
		{prefix: "side:"},
		{prefix: "double tap:", tap: true},
		{prefix: "doubletap:", tap: true},
		{prefix: "tap:", tap: true},
	} {
		if !strings.HasPrefix(normalized, candidate.prefix) {
			continue
		}
		value := strings.TrimSpace(text[len(candidate.prefix):])
		side, err := strconv.ParseUint(value, 0, 8)
		return side, candidate.tap, err == nil
	}
	return 0, false, false
}

func notificationDecodeErr(deviceID DeviceID, ch CharacteristicID, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{Operation: "events", DeviceID: deviceID, Stage: NotificationSourceName(ch), Err: err}
}

// NotificationSourceName returns a readable label for a notification characteristic.
func NotificationSourceName(ch CharacteristicID) string {
	switch ch {
	case charFacets:
		return "facet"
	case charDoubleTap:
		return "double_tap"
	case charBattery:
		return "battery"
	case charSystemState:
		return "system_state"
	case charEvents:
		return "timeflip_events"
	case charCommandResult:
		return "command_result"
	case charHistory:
		return "history"
	default:
		return string(ch)
	}
}

func tapSettingsFromPayload(payload []byte) (TapSettings, error) {
	if len(payload) < 9 || CommandCode(payload[0]) != cmdTapRead {
		return TapSettings{}, newProtocolPayloadError("tap settings response 0x17 plus register/value pairs", payload)
	}
	return TapSettings{Configured: true, Threshold: payload[2], Limit: payload[4], Latency: payload[6], Window: payload[8], Raw: append([]byte(nil), payload...)}, nil
}
