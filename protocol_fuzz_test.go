package timeflip

import (
	"bytes"
	"testing"
)

func FuzzDecodeProtocolPayloads(f *testing.F) {
	seeds := [][]byte{
		{},
		{0x00},
		{0x01},
		{0x64},
		{0x65},
		{0x84},
		{byte(cmdName), 0x02},
		{byte(cmdName), 0x02, 0x00, 0xAA},
		{0x02, 0x01, 0x02, 0x00},
		make([]byte, 17),
		make([]byte, 20),
		make([]byte, 21),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		if status, err := decodeCommandStatus(payload); err == nil {
			ack := commandAcknowledgementBytes(payload)
			if status.Code != CommandCode(ack[0]) {
				t.Fatalf("command status code %d does not match acknowledgement byte %d", status.Code, ack[0])
			}
			if !bytes.Equal(status.Raw, ack) {
				t.Fatalf("command status raw mismatch: got %X want %X", status.Raw, ack)
			}
		}

		if battery, err := decodeBattery(payload); err == nil {
			if battery.Percentage == 0 || battery.Percentage > 100 {
				t.Fatalf("battery percentage out of range: %d", battery.Percentage)
			}
			if !bytes.Equal(battery.Raw, payload) {
				t.Fatalf("battery raw mismatch: got %X want %X", battery.Raw, payload)
			}
		}

		if facet, err := decodeFacet(payload); err == nil {
			if facet.Undefined != (facet.Facet == 0) {
				t.Fatalf("facet undefined mismatch: %+v", facet)
			}
			if facet.WrongPassword != facet.Undefined {
				t.Fatalf("facet wrong-password mismatch: %+v", facet)
			}
			if !bytes.Equal(facet.Raw, payload) {
				t.Fatalf("facet raw mismatch: got %X want %X", facet.Raw, payload)
			}
		}

		if tap, err := decodeDoubleTap(payload); err == nil {
			wantPause := payload[0] >= 128
			wantFacet := FacetID(payload[0])
			if wantPause {
				wantFacet -= 128
			}
			if tap.Pause != wantPause || tap.Facet != wantFacet {
				t.Fatalf("double-tap mismatch: got %+v want pause=%v facet=%d", tap, wantPause, wantFacet)
			}
			if !bytes.Equal(tap.Raw, payload) {
				t.Fatalf("double-tap raw mismatch: got %X want %X", tap.Raw, payload)
			}
		}

		if state, err := decodeSystemState(payload); err == nil {
			if state.StatusDescription == "" || state.HardwareDescription == "" {
				t.Fatalf("system state descriptions should be populated: %+v", state)
			}
			if !bytes.Equal(state.Raw, payload) {
				t.Fatalf("system state raw mismatch: got %X want %X", state.Raw, payload)
			}
		}

		if entries, stream, err := decodeHistory(payload); err == nil {
			if len(entries) > 1 {
				t.Fatalf("v4 history packet produced too many entries: %d", len(entries))
			}
			if stream.Complete && len(entries) != 0 {
				t.Fatalf("complete v4 history packet should not produce entries: %+v", entries)
			}
			for _, entry := range entries {
				if !bytes.Equal(entry.Raw, payload) {
					t.Fatalf("v4 history raw mismatch: got %X want %X", entry.Raw, payload)
				}
			}
		}

		if entries, stream, err := decodeHistoryV3(payload); err == nil {
			if len(entries) > 7 {
				t.Fatalf("v3 history packet produced too many entries: %d", len(entries))
			}
			if stream.Complete && len(entries) != 0 {
				t.Fatalf("complete v3 history packet should not produce entries: %+v", entries)
			}
			for _, entry := range entries {
				if len(entry.Raw) != 3 {
					t.Fatalf("v3 history entry raw length = %d, want 3", len(entry.Raw))
				}
			}
		}
	})
}
