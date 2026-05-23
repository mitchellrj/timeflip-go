//go:build !darwin

package macos

import (
	"context"
	"errors"
	"testing"

	timeflip "github.com/mitchellrj/timeflip-go"
)

func TestUnsupportedTransportScanAndConnect(t *testing.T) {
	transport := NewTransport()
	if _, err := transport.Scan(context.Background(), timeflip.ScanFilter{}); !errors.Is(err, timeflip.ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported scan, got %v", err)
	}
	if _, err := transport.Connect(context.Background(), "tf"); !errors.Is(err, timeflip.ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported connect, got %v", err)
	}
}

func TestUnsupportedTransportManualActions(t *testing.T) {
	transport := NewTransport()
	pair, err := transport.PairOS(context.Background(), "tf")
	if !errors.Is(err, timeflip.ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported pair, got %v", err)
	}
	if pair.ManualAction == nil || pair.ManualAction.Kind != timeflip.ManualActionOSPair || pair.ManualAction.Inputs["device_id"] != "tf" {
		t.Fatalf("unexpected pair action: %+v", pair)
	}

	unpair, err := transport.UnpairOS(context.Background(), "tf")
	if !errors.Is(err, timeflip.ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported unpair, got %v", err)
	}
	if unpair.ManualAction == nil || unpair.ManualAction.Kind != timeflip.ManualActionOSUnpair || unpair.ManualAction.Inputs["device_id"] != "tf" {
		t.Fatalf("unexpected unpair action: %+v", unpair)
	}
}
