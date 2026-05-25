package timeflip

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
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
	cmdHistoryRead    CommandCode = CommandCode(protocol.CommandHistoryRead)
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
	ack := commandAcknowledgementBytes(payload)
	code, ok, err := protocol.DecodeCommandStatus(ack)
	status := CommandStatus{Code: CommandCode(code), OK: ok, Raw: append([]byte(nil), ack...)}
	if err != nil {
		return status, newProtocolPayloadError("command acknowledgement status 0x02 (OK) or 0x01 (rejected)", ack)
	}
	return status, nil
}

func commandAcknowledgementBytes(payload []byte) []byte {
	for i, b := range payload {
		if b == 0x00 {
			payload = payload[:i]
			break
		}
	}
	if len(payload) <= 4 {
		return payload
	}
	return payload[:4]
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
	statusDescription, syncReason := systemStatusDescription(status)
	hardwareDescription := hardwareStatusDescription(hardware)
	return SystemState{
		StatusCode:          status,
		StatusDescription:   statusDescription,
		HardwareCode:        hardware,
		HardwareDescription: hardwareDescription,
		SyncRequired:        syncReason != "",
		SyncReason:          syncReason,
		Reset:               status == 0x0100,
		HardwareIssue:       hardware != 0,
		Raw:                 append([]byte(nil), payload...),
	}, nil
}

func decodeAccelerometer(payload []byte) (AccelerometerSample, error) {
	if len(payload) != 6 {
		return AccelerometerSample{}, ErrProtocol
	}
	return AccelerometerSample{
		X:   int16(binary.BigEndian.Uint16(payload[0:2])),
		Y:   int16(binary.BigEndian.Uint16(payload[2:4])),
		Z:   int16(binary.BigEndian.Uint16(payload[4:6])),
		Raw: append([]byte(nil), payload...),
	}, nil
}

func systemStatusDescription(status uint16) (description string, syncReason string) {
	switch status {
	case 0x0000:
		return "all ok", ""
	case 0x0100:
		return "factory reset state", ""
	case 0x0201:
		return "time synchronization required", "time"
	case 0x0202:
		return "facet color synchronization required", "facet_color"
	case 0x0203:
		return "LED brightness synchronization required", "led_brightness"
	case 0x0204:
		return "blink interval synchronization required", "blink_interval"
	case 0x0205:
		return "task parameters synchronization required", "task_parameters"
	case 0x0206:
		return "auto-pause synchronization required", "auto_pause"
	default:
		if status>>8 == 0x02 {
			return "unknown synchronization required", "unknown"
		}
		return "unknown status", ""
	}
}

func hardwareStatusDescription(hardware uint16) string {
	switch hardware {
	case 0x0000:
		return "all ok"
	case 0x0201:
		return "accelerometer error"
	case 0x0202:
		return "flash memory error"
	case 0x0203:
		return "accelerometer and flash memory error"
	default:
		return "unknown hardware status"
	}
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

func decodeHistoryV3(payload []byte) ([]HistoryEntry, HistoryStreamState, error) {
	rawEntries, complete, err := protocol.DecodeHistoryPacketV3(payload)
	if err != nil {
		return nil, HistoryStreamState{}, ErrProtocol
	}
	entries := make([]HistoryEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entry := HistoryEntry{
			Facet:              FacetID(raw.Side),
			UndefinedFacet:     raw.Side == 0,
			AccelerometerError: raw.Side == 66,
			Pause:              raw.Side == 63,
			DurationSeconds:    raw.DurationSeconds,
			Raw:                append([]byte(nil), raw.Raw...),
		}
		entries = append(entries, entry)
	}
	return entries, HistoryStreamState{Complete: complete}, nil
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
		ProtocolVersion:  inferProtocolVersion(cleanString(values[charFirmwareRevision])),
		Raw:              raw,
	}
}

func inferProtocolVersion(firmware string) ProtocolVersion {
	normalized := strings.ToLower(firmware)
	if strings.Contains(normalized, "tfv4") || strings.Contains(normalized, "fw_v4") {
		return ProtocolV4
	}
	if version, ok := parseFirmwareNumber(normalized); ok {
		if version >= 3.47 {
			return ProtocolV4
		}
		return ProtocolV3
	}
	if strings.Contains(normalized, "tfv3") || strings.Contains(normalized, "fw_v3") {
		return ProtocolV3
	}
	return ProtocolAuto
}

func parseFirmwareNumber(firmware string) (float64, bool) {
	for _, marker := range []string{"fw_v", "tfv"} {
		idx := strings.Index(firmware, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		end := start
		for end < len(firmware) {
			ch := firmware[end]
			if (ch < '0' || ch > '9') && ch != '.' {
				break
			}
			end++
		}
		if end == start {
			continue
		}
		version, err := strconv.ParseFloat(firmware[start:end], 64)
		return version, err == nil
	}
	return 0, false
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
