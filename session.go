package timeflip

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

// Session manages one active TimeFlip2 connection.
type Session struct {
	deviceID       DeviceID
	conn           Connection
	defaultTimeout time.Duration
	closeOnce      sync.Once
	done           chan struct{}
}

// Authorize writes a six-byte password to the device password characteristic.
func (s *Session) Authorize(ctx context.Context, password string) (AuthorizationResult, error) {
	if len(password) != 6 {
		return AuthorizationResult{}, &OperationError{Operation: "authorize", DeviceID: s.deviceID, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, 0)
	defer cancel()
	if err := s.conn.Write(ctx, charPassword, []byte(password)); err != nil {
		return AuthorizationResult{}, wrapContextErr("authorize", s.deviceID, "", 0, err)
	}
	payload, err := s.conn.Read(ctx, charCommandResult)
	if err == nil && len(payload) > 0 && payload[0] == 0x02 {
		return AuthorizationResult{}, &OperationError{Operation: "authorize", DeviceID: s.deviceID, Err: ErrAuthorizationFailed}
	}
	return AuthorizationResult{Authorized: true}, nil
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
	for _, ch := range []CharacteristicID{charDeviceName, charManufacturerName, charModelNumber, charHardwareRevision, charFirmwareRevision, charSystemID} {
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
	return decodeDeviceInfo(values), nil
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

// ReadHistory reads device history entries.
func (s *Session) ReadHistory(ctx context.Context, req HistoryRequest) ([]HistoryEntry, error) {
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
		return nil, &OperationError{Operation: "read_history", DeviceID: s.deviceID, Err: err}
	}
	return entries, nil
}

// ReadTaskParameters reads task parameters for a facet.
func (s *Session) ReadTaskParameters(ctx context.Context, facet FacetID, opts CommandOptions) (TaskParameters, error) {
	result, err := s.SendCommand(ctx, Command{Code: cmdReadTask, Payload: []byte{byte(facet)}}, opts)
	if err != nil {
		return TaskParameters{}, err
	}
	if len(result.Payload) < 11 {
		return TaskParameters{}, &OperationError{Operation: "read_task_parameters", DeviceID: s.deviceID, Err: ErrProtocol}
	}
	return TaskParameters{
		Facet:                FacetID(result.Payload[1]),
		Mode:                 result.Payload[2],
		PomodoroLimitSeconds: binary.BigEndian.Uint32(result.Payload[3:7]),
		ElapsedSeconds:       binary.BigEndian.Uint32(result.Payload[7:11]),
	}, nil
}

// ReadTapSettings reads double-tap accelerometer settings.
func (s *Session) ReadTapSettings(ctx context.Context, opts CommandOptions) (TapSettings, error) {
	result, err := s.SendCommand(ctx, commandNoPayload(cmdTapRead), opts)
	if err != nil {
		return TapSettings{}, err
	}
	return tapSettingsFromPayload(result.Payload)
}

// SendCommand writes a supported command and reads the command result.
func (s *Session) SendCommand(ctx context.Context, cmd Command, opts CommandOptions) (CommandResult, error) {
	encoded, err := encodeCommand(cmd)
	if err != nil {
		return CommandResult{}, &OperationError{Operation: "send_command", DeviceID: s.deviceID, Command: cmd.Code, Err: ErrInvalidInput}
	}
	ctx, cancel := timeoutFrom(ctx, s.defaultTimeout, opts.Timeout)
	defer cancel()
	if err := s.conn.Write(ctx, charCommand, encoded); err != nil {
		return CommandResult{}, wrapContextErr("send_command", s.deviceID, "", cmd.Code, err)
	}
	payload, err := s.conn.Read(ctx, charCommandResult)
	if err != nil {
		return CommandResult{}, wrapContextErr("send_command", s.deviceID, "", cmd.Code, err)
	}
	status, err := decodeCommandStatus(payload)
	if err != nil {
		return CommandResult{Command: cmd, Payload: payload}, &OperationError{Operation: "send_command", DeviceID: s.deviceID, Command: cmd.Code, Err: err}
	}
	result := CommandResult{Command: cmd, Status: status, Payload: append([]byte(nil), payload...)}
	if !status.OK {
		return result, &OperationError{Operation: "send_command", DeviceID: s.deviceID, Command: cmd.Code, Err: ErrCommandRejected}
	}
	return result, nil
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
	if len(name) == 0 || len(name) > 18 {
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
	subs := []CharacteristicID{charFacets, charDoubleTap, charBattery, charSystemState, charEvents, charHistory}
	streams := make([]<-chan Notification, 0, len(subs))
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

func (s *Session) decodeNotification(n Notification, includeRaw bool) (Event, error) {
	event := Event{DeviceID: s.deviceID, Source: n.Characteristic, ReceivedAt: time.Now()}
	if includeRaw {
		event.Raw = append([]byte(nil), n.Payload...)
	}
	switch n.Characteristic {
	case charFacets:
		payload, err := decodeFacet(n.Payload)
		event.Kind, event.Payload = EventFacet, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charDoubleTap:
		payload, err := decodeDoubleTap(n.Payload)
		event.Kind, event.Payload = EventDoubleTap, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charBattery:
		payload, err := decodeBattery(n.Payload)
		event.Kind, event.Payload = EventBattery, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charSystemState:
		payload, err := decodeSystemState(n.Payload)
		event.Kind, event.Payload = EventSystemState, payload
		return event, notificationDecodeErr(s.deviceID, n.Characteristic, err)
	case charEvents:
		payload := append([]byte(nil), n.Payload...)
		event.Kind, event.Payload = EventRaw, payload
		if len(event.Raw) == 0 {
			event.Raw = payload
		}
		return event, nil
	case charHistory:
		payload, _, err := decodeHistory(n.Payload)
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
	case charHistory:
		return "history"
	default:
		return string(ch)
	}
}

func tapSettingsFromPayload(payload []byte) (TapSettings, error) {
	if len(payload) < 9 {
		return TapSettings{}, ErrProtocol
	}
	return TapSettings{Threshold: payload[2], Limit: payload[4], Latency: payload[6], Window: payload[8]}, nil
}
