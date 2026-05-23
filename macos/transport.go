// Package macos contains the first platform adapter boundary for MacOS BLE.
package macos

import (
	"context"

	timeflip "github.com/mitchellrj/timeflip-go"
)

// Transport is a MacOS BLE transport placeholder.
//
// It intentionally compiles without external BLE dependencies. A concrete
// CoreBluetooth-backed implementation can replace these methods while keeping
// the public timeflip.Transport contract stable.
type Transport struct{}

// NewTransport creates a MacOS transport.
func NewTransport() *Transport {
	return &Transport{}
}

// Scan reports that the concrete MacOS BLE bridge is not configured.
func (t *Transport) Scan(context.Context, timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	return nil, &timeflip.OperationError{Operation: "macos_scan", Err: timeflip.ErrUnsupportedOperation}
}

// Connect reports that the concrete MacOS BLE bridge is not configured.
func (t *Transport) Connect(context.Context, timeflip.DeviceID) (timeflip.Connection, error) {
	return nil, &timeflip.OperationError{Operation: "macos_connect", Err: timeflip.ErrUnsupportedOperation}
}

// PairOS returns a manual action because direct OS pairing support depends on the selected bridge.
func (t *Transport) PairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSPair, id, "Pair the TimeFlip2 device in macOS Bluetooth settings, then retry the library operation.")
}

// UnpairOS returns a manual action because direct OS unpairing support depends on the selected bridge.
func (t *Transport) UnpairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSUnpair, id, "Remove the TimeFlip2 device in macOS Bluetooth settings.")
}

func unsupportedOSAction(kind timeflip.ManualActionKind, id timeflip.DeviceID, description string) (timeflip.OSActionResult, error) {
	return timeflip.OSActionResult{
		Unsupported: true,
		ManualAction: &timeflip.ManualAction{
			Kind:        kind,
			Description: description,
			Inputs: map[string]string{
				"device_id": string(id),
			},
		},
	}, timeflip.ErrUnsupportedOperation
}
