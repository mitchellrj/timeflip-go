package protocol

import "encoding/binary"

// DecodeBattery returns battery percentage.
func DecodeBattery(payload []byte) (uint8, error) {
	if len(payload) != 1 || payload[0] == 0 || payload[0] > 100 {
		return 0, ErrInvalidPayload
	}
	return payload[0], nil
}

// DecodeFacet returns facet and whether it is undefined.
func DecodeFacet(payload []byte) (uint8, bool, error) {
	if len(payload) != 1 {
		return 0, false, ErrInvalidPayload
	}
	return payload[0], payload[0] == 0, nil
}

// DecodeDoubleTap returns facet and pause flag.
func DecodeDoubleTap(payload []byte) (uint8, bool, error) {
	if len(payload) != 1 {
		return 0, false, ErrInvalidPayload
	}
	if payload[0] >= 128 {
		return payload[0] - 128, true, nil
	}
	return payload[0], false, nil
}

// DecodeSystemState decodes four system-state bytes.
func DecodeSystemState(payload []byte) (uint16, uint16, error) {
	if len(payload) != 4 {
		return 0, 0, ErrInvalidPayload
	}
	return binary.BigEndian.Uint16(payload[0:2]), binary.BigEndian.Uint16(payload[2:4]), nil
}

// DecodeHistoryPacket decodes v4 history packets.
func DecodeHistoryPacket(payload []byte) (entries []HistoryEntry, complete bool, previous uint32, err error) {
	if len(payload) != 17 && len(payload) != 20 {
		return nil, false, 0, ErrInvalidPayload
	}
	allZeroPrefix := true
	for i := 0; i < 17; i++ {
		if payload[i] != 0 {
			allZeroPrefix = false
			break
		}
	}
	if allZeroPrefix {
		if len(payload) == 17 {
			return nil, true, 0, nil
		}
		return nil, true, uint32(payload[17])<<16 | uint32(payload[18])<<8 | uint32(payload[19]), nil
	}
	if len(payload) == 20 {
		previous = uint32(payload[17])<<16 | uint32(payload[18])<<8 | uint32(payload[19])
	}
	entry := HistoryEntry{
		EventNumber:         binary.BigEndian.Uint32(payload[0:4]),
		Side:                payload[4],
		MomentUnix:          binary.BigEndian.Uint64(payload[5:13]),
		DurationSeconds:     binary.BigEndian.Uint32(payload[13:17]),
		PreviousEventNumber: previous,
		Raw:                 append([]byte(nil), payload...),
	}
	return []HistoryEntry{entry}, false, entry.PreviousEventNumber, nil
}

// HistoryEntry is the protocol-level history representation.
type HistoryEntry struct {
	EventNumber         uint32
	Side                uint8
	MomentUnix          uint64
	DurationSeconds     uint32
	PreviousEventNumber uint32
	Raw                 []byte
}
