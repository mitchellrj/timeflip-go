//go:build !darwin

// Package macos contains the MacOS BLE adapter boundary.
package macos

import (
	"context"

	timeflip "github.com/mitchellrj/timeflip-go"
)

// Transport reports unsupported BLE behavior on non-Darwin builds.
type Transport struct{}

// NewTransport creates a transport value that reports unsupported behavior.
func NewTransport() *Transport {
	return &Transport{}
}

// Scan reports that the MacOS BLE adapter is unsupported on this platform.
func (t *Transport) Scan(context.Context, timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	return nil, &timeflip.OperationError{Operation: "macos_scan", Err: timeflip.ErrUnsupportedOperation}
}

// AdvertisedName reports that advertised-name lookup is unsupported on this platform.
func (t *Transport) AdvertisedName(context.Context, timeflip.DeviceID) (string, bool) {
	return "", false
}

// Connect reports that the MacOS BLE adapter is unsupported on this platform.
func (t *Transport) Connect(context.Context, timeflip.DeviceID) (timeflip.Connection, error) {
	return nil, &timeflip.OperationError{Operation: "macos_connect", Err: timeflip.ErrUnsupportedOperation}
}

// PairOS returns a manual pairing action because direct OS pairing is unsupported.
func (t *Transport) PairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSPair, id, "Automatic macOS pairing is not available from this adapter. To complete pairing: 1. Run the demo on macOS with Bluetooth enabled. 2. Open System Settings > Bluetooth. 3. Find the TimeFlip2 device and click Connect or Pair. 4. Return to this demo and run the suggested connect/read commands.")
}

// UnpairOS returns a manual unpairing action because direct OS unpairing is unsupported.
func (t *Transport) UnpairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSUnpair, id, "Automatic macOS unpairing is not available from this adapter. To complete unpairing: 1. Open System Settings > Bluetooth. 2. Find the TimeFlip2 device. 3. Open the device details and choose Forget This Device or Remove. 4. Return to this demo and run list or pair again.")
}
