package timeflip

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient(nil, Config{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if _, err := NewClient(&fakeTransport{}, Config{CommunicationTimeout: -time.Second}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid timeout, got %v", err)
	}
}

func TestListDevicesFiltersUnsupported(t *testing.T) {
	ft := &fakeTransport{peripherals: []Peripheral{
		{ID: "tf", Name: "TimeFlip2", RSSI: -50},
		{ID: "kb", Name: "Keyboard", RSSI: -30},
	}}
	client, err := NewClient(ft, Config{})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := client.ListDevices(context.Background(), ScanFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ID != "tf" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestPairManualOSActionAndPasswordChange(t *testing.T) {
	conn := &fakeConnection{reads: map[CharacteristicID][]byte{
		charCommandResult: {0x30, 0x02},
		charSystemState:   {0x00, 0x00, 0x00, 0x00},
	}}
	ft := &fakeTransport{
		connections: map[DeviceID]*fakeConnection{"tf": conn},
		pairResult: OSActionResult{
			Unsupported: true,
			ManualAction: &ManualAction{
				Kind:        ManualActionOSPair,
				Description: "pair manually",
				Inputs:      map[string]string{"device_id": "tf"},
			},
		},
		pairErr: ErrUnsupportedOperation,
	}
	client, err := NewClient(ft, Config{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Pair(context.Background(), PairRequest{
		DeviceID:       "tf",
		Password:       "000000",
		NewPassword:    "111111",
		AllowOSPairing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.ManualAction == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(conn.writes) < 2 {
		t.Fatalf("expected authorization and password writes, got %d", len(conn.writes))
	}
}

func TestUnpairUnsupportedOSReturnsManualAction(t *testing.T) {
	ft := &fakeTransport{
		unpairResult: OSActionResult{
			Unsupported:  true,
			ManualAction: &ManualAction{Kind: ManualActionOSUnpair, Inputs: map[string]string{"device_id": "tf"}},
		},
		unpairErr: ErrUnsupportedOperation,
	}
	client, err := NewClient(ft, Config{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Unpair(context.Background(), UnpairRequest{DeviceID: "tf", AllowOSUnpairing: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.ManualAction == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}
