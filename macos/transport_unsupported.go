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

// Connect reports that the MacOS BLE adapter is unsupported on this platform.
func (t *Transport) Connect(context.Context, timeflip.DeviceID) (timeflip.Connection, error) {
	return nil, &timeflip.OperationError{Operation: "macos_connect", Err: timeflip.ErrUnsupportedOperation}
}

// PairOS returns a manual pairing action because direct OS pairing is unsupported.
func (t *Transport) PairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSPair, id, "Pair the TimeFlip2 device in macOS Bluetooth settings, then retry the library operation.")
}

// UnpairOS returns a manual unpairing action because direct OS unpairing is unsupported.
func (t *Transport) UnpairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSUnpair, id, "Remove the TimeFlip2 device in macOS Bluetooth settings.")
}
