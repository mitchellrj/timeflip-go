package protocol

import (
	"encoding/binary"
	"errors"
)

const (
	CommandHistoryRead    byte = 0x01
	CommandLock           byte = 0x04
	CommandAutoPause      byte = 0x05
	CommandPause          byte = 0x06
	CommandReadTime       byte = 0x07
	CommandSetTime        byte = 0x08
	CommandBrightness     byte = 0x09
	CommandBlinkPeriod    byte = 0x0A
	CommandStatus         byte = 0x10
	CommandFacetColor     byte = 0x11
	CommandTaskParameters byte = 0x13
	CommandReadTask       byte = 0x14
	CommandName           byte = 0x15
	CommandTapWrite       byte = 0x16
	CommandTapRead        byte = 0x17
	CommandSetPassword    byte = 0x30
	CommandResetTaskInfo  byte = 0xFE
	CommandFactoryReset   byte = 0xFF
)

var ErrInvalidPayload = errors.New("invalid payload")

// EncodeCommand prefixes code to payload after basic size validation.
func EncodeCommand(code byte, payload []byte) ([]byte, error) {
	if len(payload) > 19 {
		return nil, ErrInvalidPayload
	}
	out := make([]byte, 1, len(payload)+1)
	out[0] = code
	out = append(out, payload...)
	return out, nil
}

// DecodeCommandStatus decodes a command-characteristic acknowledgement.
func DecodeCommandStatus(payload []byte) (byte, bool, error) {
	if len(payload) < 2 {
		return 0, false, ErrInvalidPayload
	}
	result := payload[1]
	switch result {
	case 0x02:
		return payload[0], true, nil
	case 0x01:
		return payload[0], false, nil
	default:
		return payload[0], false, ErrInvalidPayload
	}
}

// DecodeTrackerStatus decodes command 0x10 response bytes.
func DecodeTrackerStatus(payload []byte) (bool, bool, uint16, error) {
	if len(payload) < 4 {
		return false, false, 0, ErrInvalidPayload
	}
	if (payload[0] != 0x01 && payload[0] != 0x02) || (payload[1] != 0x01 && payload[1] != 0x02) {
		return false, false, 0, ErrInvalidPayload
	}
	lock := payload[0] == 0x01
	pause := payload[1] == 0x01
	autoPause := binary.BigEndian.Uint16(payload[2:4])
	return lock, pause, autoPause, nil
}
