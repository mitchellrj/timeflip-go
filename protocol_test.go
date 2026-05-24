package timeflip

import (
	"encoding/binary"
	"testing"
)

func TestIsSupportedPeripheral(t *testing.T) {
	tests := []struct {
		name string
		p    Peripheral
		want bool
	}{
		{
			name: "advertised service",
			p: Peripheral{
				ID:                 "a",
				AdvertisedServices: []ServiceID{TimeFlipService},
			},
			want: true,
		},
		{
			name: "name heuristic fallback is supported but not authenticated identity",
			p:    Peripheral{ID: "b", Name: "TIMEFLIP2"},
			want: true,
		},
		{
			name: "metadata model heuristic fallback is supported but not authenticated identity",
			p:    Peripheral{ID: "model", Metadata: map[string]string{"model": "TimeFlip 2"}},
			want: true,
		},
		{
			name: "unsupported",
			p:    Peripheral{ID: "c", Name: "Keyboard"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedPeripheral(tt.p); got != tt.want {
				t.Fatalf("IsSupportedPeripheral()=%v want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeNotifications(t *testing.T) {
	facet, err := decodeFacet([]byte{7})
	if err != nil {
		t.Fatal(err)
	}
	if facet.Facet != 7 || facet.Undefined {
		t.Fatalf("unexpected facet: %+v", facet)
	}

	tap, err := decodeDoubleTap([]byte{132})
	if err != nil {
		t.Fatal(err)
	}
	if tap.Facet != 4 || !tap.Pause {
		t.Fatalf("unexpected tap: %+v", tap)
	}

	state, err := decodeSystemState([]byte{0x02, 0x01, 0x02, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	if !state.SyncRequired || state.SyncReason != "time" || state.StatusDescription != "time synchronization required" || !state.HardwareIssue || state.HardwareDescription != "unknown hardware status" {
		t.Fatalf("unexpected system state: %+v", state)
	}
}

func TestDecodeHistoryPacket(t *testing.T) {
	payload := make([]byte, 20)
	binary.BigEndian.PutUint32(payload[0:4], 12)
	payload[4] = 129
	binary.BigEndian.PutUint64(payload[5:13], 1000)
	binary.BigEndian.PutUint32(payload[13:17], 30)
	payload[19] = 11

	entries, stream, err := decodeHistory(payload)
	if err != nil {
		t.Fatal(err)
	}
	if stream.Complete || len(entries) != 1 {
		t.Fatalf("unexpected stream=%+v entries=%d", stream, len(entries))
	}
	if entries[0].Facet != 1 || !entries[0].Pause || entries[0].DurationSeconds != 30 {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}

	single := payload[:17]
	entries, stream, err = decodeHistory(single)
	if err != nil {
		t.Fatal(err)
	}
	if stream.Complete || stream.PreviousEventNumber != 0 || len(entries) != 1 {
		t.Fatalf("unexpected single history stream=%+v entries=%d", stream, len(entries))
	}
	if entries[0].Facet != 1 || !entries[0].Pause || entries[0].PreviousEventNumber != 0 {
		t.Fatalf("unexpected single entry: %+v", entries[0])
	}
}

func TestDecodeHistoryPacketV3(t *testing.T) {
	payload := make([]byte, 21)
	payload[0] = 0x00
	payload[1] = 0x01
	payload[2] = byte(7<<2) | 0x02

	entries, stream, err := decodeHistoryV3(payload)
	if err != nil {
		t.Fatal(err)
	}
	if stream.Complete || len(entries) != 1 {
		t.Fatalf("unexpected stream=%+v entries=%d", stream, len(entries))
	}
	if entries[0].Facet != 7 || entries[0].DurationSeconds != 258 {
		t.Fatalf("unexpected v3 entry: %+v", entries[0])
	}
}

func TestInferProtocolVersionFromFirmware(t *testing.T) {
	if got := inferProtocolVersion("TFv3.1"); got != ProtocolV3 {
		t.Fatalf("expected v3, got %q", got)
	}
	if got := inferProtocolVersion("FW_v3.47"); got != ProtocolV4 {
		t.Fatalf("expected v4 for firmware split, got %q", got)
	}
	if got := inferProtocolVersion("TFv4.0"); got != ProtocolV4 {
		t.Fatalf("expected v4, got %q", got)
	}
	if got := inferProtocolVersion("unknown"); got != ProtocolAuto {
		t.Fatalf("expected auto, got %q", got)
	}
}

func TestEncodeCommandValidation(t *testing.T) {
	_, err := encodeCommand(Command{Code: cmdName, Payload: make([]byte, 20)})
	if err == nil {
		t.Fatalf("expected payload validation error")
	}
}
