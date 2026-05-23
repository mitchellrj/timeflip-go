package timeflip

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mitchellrj/timeflip-go/internal/protocol"
)

const (
	// TimeFlipService is the primary TimeFlip2 BLE service UUID.
	TimeFlipService ServiceID = protocol.TimeFlipService
)

const (
	charDeviceName       CharacteristicID = protocol.DeviceNameCharacteristic
	charManufacturerName CharacteristicID = protocol.ManufacturerNameString
	charModelNumber      CharacteristicID = protocol.ModelNumberString
	charHardwareRevision CharacteristicID = protocol.HardwareRevisionString
	charFirmwareRevision CharacteristicID = protocol.FirmwareRevisionString
	charSystemID         CharacteristicID = protocol.SystemID
	charBattery          CharacteristicID = protocol.BatteryLevel
	charEvents           CharacteristicID = protocol.TimeFlipEvents
	charFacets           CharacteristicID = protocol.Facets
	charCommandResult    CharacteristicID = protocol.CommandResultOutput
	charCommand          CharacteristicID = protocol.Command
	charDoubleTap        CharacteristicID = protocol.DoubleTap
	charSystemState      CharacteristicID = protocol.SystemState
	charPassword         CharacteristicID = protocol.PasswordCheck
	charHistory          CharacteristicID = protocol.HistoryData
)

const (
	cmdLock           CommandCode = CommandCode(protocol.CommandLock)
	cmdAutoPause      CommandCode = CommandCode(protocol.CommandAutoPause)
	cmdPause          CommandCode = CommandCode(protocol.CommandPause)
	cmdReadTime       CommandCode = CommandCode(protocol.CommandReadTime)
	cmdSetTime        CommandCode = CommandCode(protocol.CommandSetTime)
	cmdBrightness     CommandCode = CommandCode(protocol.CommandBrightness)
	cmdBlinkPeriod    CommandCode = CommandCode(protocol.CommandBlinkPeriod)
	cmdStatus         CommandCode = CommandCode(protocol.CommandStatus)
	cmdFacetColor     CommandCode = CommandCode(protocol.CommandFacetColor)
	cmdTaskParameters CommandCode = CommandCode(protocol.CommandTaskParameters)
	cmdReadTask       CommandCode = CommandCode(protocol.CommandReadTask)
	cmdName           CommandCode = CommandCode(protocol.CommandName)
	cmdTapWrite       CommandCode = CommandCode(protocol.CommandTapWrite)
	cmdTapRead        CommandCode = CommandCode(protocol.CommandTapRead)
	cmdSetPassword    CommandCode = CommandCode(protocol.CommandSetPassword)
	cmdResetTaskInfo  CommandCode = CommandCode(protocol.CommandResetTaskInfo)
	cmdFactoryReset   CommandCode = CommandCode(protocol.CommandFactoryReset)
)

// RequiredCharacteristics returns the TimeFlip2 characteristics used by the library.
func RequiredCharacteristics() []CharacteristicID {
	raw := protocol.RequiredCharacteristics()
	out := make([]CharacteristicID, len(raw))
	for i, id := range raw {
		out[i] = CharacteristicID(id)
	}
	return out
}

// IsSupportedPeripheral reports whether p appears to be a TimeFlip2 device.
func IsSupportedPeripheral(p Peripheral) bool {
	for _, svc := range p.AdvertisedServices {
		if strings.EqualFold(string(svc), protocol.TimeFlipService) {
			return true
		}
	}
	name := strings.ToLower(p.Name)
	if strings.Contains(name, "timeflip") || strings.Contains(name, "time flip") {
		return true
	}
	if model := strings.ToLower(p.Metadata["model"]); strings.Contains(model, "timeflip") {
		return true
	}
	return false
}

func encodeCommand(cmd Command) ([]byte, error) {
	return protocol.EncodeCommand(byte(cmd.Code), cmd.Payload)
}

func decodeCommandStatus(payload []byte) (CommandStatus, error) {
	code, ok, err := protocol.DecodeCommandStatus(payload)
	status := CommandStatus{Code: CommandCode(code), OK: ok, Raw: append([]byte(nil), payload...)}
	if err != nil {
		return status, ErrProtocol
	}
	return status, nil
}

func decodeTrackerStatus(payload []byte) (TrackerStatus, error) {
	lock, pause, autoPause, err := protocol.DecodeTrackerStatus(payload)
	if err != nil {
		return TrackerStatus{}, ErrProtocol
	}
	return TrackerStatus{LockEnabled: lock, PauseEnabled: pause, AutoPauseMinutes: autoPause, Raw: append([]byte(nil), payload...)}, nil
}

func decodeBattery(payload []byte) (BatteryStatus, error) {
	pct, err := protocol.DecodeBattery(payload)
	if err != nil {
		return BatteryStatus{}, ErrProtocol
	}
	return BatteryStatus{Percentage: pct, Raw: append([]byte(nil), payload...)}, nil
}

func decodeFacet(payload []byte) (FacetEvent, error) {
	facet, undefined, err := protocol.DecodeFacet(payload)
	if err != nil {
		return FacetEvent{}, ErrProtocol
	}
	return FacetEvent{Facet: FacetID(facet), Undefined: undefined, WrongPassword: undefined, Raw: append([]byte(nil), payload...)}, nil
}

func decodeDoubleTap(payload []byte) (DoubleTapEvent, error) {
	facet, pause, err := protocol.DecodeDoubleTap(payload)
	if err != nil {
		return DoubleTapEvent{}, ErrProtocol
	}
	return DoubleTapEvent{Facet: FacetID(facet), Pause: pause, Raw: append([]byte(nil), payload...)}, nil
}

func decodeSystemState(payload []byte) (SystemState, error) {
	status, hardware, err := protocol.DecodeSystemState(payload)
	if err != nil {
		return SystemState{}, ErrProtocol
	}
	return SystemState{
		StatusCode:    status,
		HardwareCode:  hardware,
		SyncRequired:  status>>8 == 0x02,
		Reset:         status == 0x0100,
		HardwareIssue: hardware != 0,
		Raw:           append([]byte(nil), payload...),
	}, nil
}

func decodeHistory(payload []byte) ([]HistoryEntry, HistoryStreamState, error) {
	rawEntries, complete, previous, err := protocol.DecodeHistoryPacket(payload)
	if err != nil {
		return nil, HistoryStreamState{}, ErrProtocol
	}
	entries := make([]HistoryEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entry := HistoryEntry{
			EventNumber:         raw.EventNumber,
			Facet:               FacetID(raw.Side),
			UndefinedFacet:      raw.Side == 0,
			AccelerometerError:  raw.Side == 66,
			Pause:               raw.Side > 127,
			MomentUnix:          raw.MomentUnix,
			DurationSeconds:     raw.DurationSeconds,
			PreviousEventNumber: raw.PreviousEventNumber,
			Raw:                 append([]byte(nil), raw.Raw...),
		}
		if entry.Pause {
			entry.Facet = FacetID(raw.Side - 128)
		}
		entries = append(entries, entry)
	}
	return entries, HistoryStreamState{Complete: complete, PreviousEventNumber: previous}, nil
}

func decodeDeviceInfo(values map[CharacteristicID][]byte) DeviceInfo {
	raw := make(map[CharacteristicID][]byte, len(values))
	for k, v := range values {
		raw[k] = append([]byte(nil), v...)
	}
	return DeviceInfo{
		Name:             cleanString(values[charDeviceName]),
		ManufacturerName: cleanString(values[charManufacturerName]),
		ModelNumber:      cleanString(values[charModelNumber]),
		HardwareRevision: cleanString(values[charHardwareRevision]),
		FirmwareRevision: cleanString(values[charFirmwareRevision]),
		SystemID:         hexCode(values[charSystemID]),
		Raw:              raw,
	}
}

func hexCode(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return "0x" + strings.ToUpper(hex.EncodeToString(b))
}

func cleanString(b []byte) string {
	b = []byte(strings.TrimRight(string(b), "\x00 "))
	if utf8.Valid(b) {
		return string(b)
	}
	return fmt.Sprintf("%X", b)
}

// Command constructors.
func commandNoPayload(code CommandCode) Command { return Command{Code: code} }

func commandBool(code CommandCode, enabled bool) Command {
	if enabled {
		return Command{Code: code, Payload: []byte{0x01}}
	}
	return Command{Code: code, Payload: []byte{0x02}}
}

func commandUint16(code CommandCode, value uint16) Command {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, value)
	return Command{Code: code, Payload: payload}
}
