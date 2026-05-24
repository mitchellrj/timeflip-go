//go:build darwin

// Package macos contains a CoreBluetooth-backed BLE transport for MacOS.
package macos

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
	"tinygo.org/x/bluetooth"
)

type connectResult struct {
	device bluetooth.Device
	err    error
}

// Transport is a CoreBluetooth-backed MacOS BLE transport.
type Transport struct {
	adapter *bluetooth.Adapter

	mu         sync.Mutex
	enabled    bool
	scanning   bool
	discovered map[timeflip.DeviceID]bluetooth.ScanResult

	// AllowNameFallback permits Connect to match an undiscovered peripheral by
	// advertised name. Leave false to avoid connecting to a spoofed name when a
	// stable device ID/address was requested.
	AllowNameFallback bool
}

// NewTransport creates a MacOS BLE transport.
func NewTransport() *Transport {
	return &Transport{
		adapter:    bluetooth.DefaultAdapter,
		discovered: map[timeflip.DeviceID]bluetooth.ScanResult{},
	}
}

func (t *Transport) ensureAdapter(ctx context.Context) error {
	t.mu.Lock()
	if t.enabled {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	errs := make(chan error, 1)
	go func() {
		errs <- t.adapter.Enable()
	}()

	select {
	case <-ctx.Done():
		return operationErr("macos_adapter", "", ctx.Err())
	case err := <-errs:
		if err != nil {
			return operationErr("macos_adapter", "", err)
		}
		t.mu.Lock()
		t.enabled = true
		t.mu.Unlock()
		return nil
	}
}

// Scan scans for BLE peripherals and returns platform-neutral descriptions.
func (t *Transport) Scan(ctx context.Context, filter timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	if err := t.ensureAdapter(ctx); err != nil {
		return nil, err
	}

	var peripherals []timeflip.Peripheral
	err := t.scan(ctx, func(result bluetooth.ScanResult) bool {
		peripheral := t.remember(result)
		if filter.IncludeUnsupported || timeflip.IsSupportedPeripheral(peripheral) {
			peripherals = appendOrReplacePeripheral(peripherals, peripheral)
		}
		return false
	})
	if err != nil {
		return nil, operationErr("macos_scan", "", err)
	}
	return peripherals, nil
}

// AdvertisedName returns the latest BLE advertised name known for a peripheral.
func (t *Transport) AdvertisedName(ctx context.Context, id timeflip.DeviceID) (string, bool) {
	if id == "" {
		return "", false
	}
	var fallback string
	if result, ok := t.lookup(id); ok {
		fallback = result.LocalName()
	}
	if err := t.ensureAdapter(ctx); err != nil {
		return fallback, fallback != ""
	}
	var found string
	_ = t.scan(ctx, func(result bluetooth.ScanResult) bool {
		peripheral := t.remember(result)
		if peripheral.ID != id {
			return false
		}
		if peripheral.Name != "" {
			found = peripheral.Name
			return true
		}
		return false
	})
	if found != "" {
		return found, true
	}
	return fallback, fallback != ""
}

func (t *Transport) scan(ctx context.Context, handle func(bluetooth.ScanResult) bool) error {
	t.mu.Lock()
	if t.scanning {
		t.mu.Unlock()
		return errors.New("scan already in progress")
	}
	t.scanning = true
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.scanning = false
		t.mu.Unlock()
	}()

	scanErr := make(chan error, 1)
	go func() {
		scanErr <- t.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			if handle(result) {
				_ = adapter.StopScan()
			}
		})
	}()

	select {
	case err := <-scanErr:
		return err
	case <-ctx.Done():
		if err := t.adapter.StopScan(); err != nil {
			return err
		}
		if err := <-scanErr; err != nil {
			return err
		}
		return nil
	}
}

func (t *Transport) remember(result bluetooth.ScanResult) timeflip.Peripheral {
	id := timeflip.DeviceID(result.Address.String())
	peripheral := timeflip.Peripheral{
		ID:                 id,
		Name:               result.LocalName(),
		RSSI:               int(result.RSSI),
		AdvertisedServices: advertisedServices(result),
		ManufacturerData:   manufacturerData(result),
		Metadata: map[string]string{
			"address": result.Address.String(),
		},
	}

	t.mu.Lock()
	t.discovered[id] = result
	t.mu.Unlock()
	return peripheral
}

// Connect opens a BLE connection to a discovered peripheral.
func (t *Transport) Connect(ctx context.Context, id timeflip.DeviceID) (timeflip.Connection, error) {
	if id == "" {
		return nil, operationErr("macos_connect", id, timeflip.ErrInvalidInput)
	}
	if err := t.ensureAdapter(ctx); err != nil {
		return nil, err
	}

	result, ok := t.lookup(id)
	if !ok {
		var found bool
		err := t.scan(ctx, func(candidate bluetooth.ScanResult) bool {
			peripheral := t.remember(candidate)
			if matchesConnectCandidate(peripheral, id, t.AllowNameFallback) {
				result = candidate
				found = true
				return true
			}
			return false
		})
		if err != nil {
			return nil, operationErr("macos_connect", id, err)
		}
		if !found {
			return nil, operationErr("macos_connect", id, fmt.Errorf("device not found"))
		}
	}

	t.remember(result)

	params := bluetooth.ConnectionParams{}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, operationErr("macos_connect", id, context.DeadlineExceeded)
		}
		params.ConnectionTimeout = bluetooth.NewDuration(remaining)
	}
	connect := make(chan connectResult, 1)
	go func() {
		device, err := t.adapter.Connect(result.Address, params)
		connect <- connectResult{device: device, err: err}
	}()
	var device bluetooth.Device
	select {
	case <-ctx.Done():
		return nil, operationErr("macos_connect", id, ctx.Err())
	case result := <-connect:
		if result.err != nil {
			return nil, operationErr("macos_connect", id, result.err)
		}
		device = result.device
	}
	conn := &Connection{
		deviceID:        id,
		device:          device,
		connected:       true,
		characteristics: map[timeflip.CharacteristicID]bluetooth.DeviceCharacteristic{},
		subscriptions:   map[timeflip.CharacteristicID]*subscription{},
		done:            make(chan struct{}),
	}
	discover := make(chan discoverResult, 1)
	go func() {
		discover <- discoverResult{err: conn.discoverCharacteristics()}
	}()
	select {
	case <-ctx.Done():
		_ = conn.Close(context.Background())
		return nil, operationErr("macos_connect", id, ctx.Err())
	case result := <-discover:
		if result.err == nil {
			return conn, nil
		}
		_ = conn.Close(context.Background())
		return nil, operationErr("macos_connect", id, result.err)
	}
}

func matchesConnectCandidate(peripheral timeflip.Peripheral, requestedID timeflip.DeviceID, allowNameFallback bool) bool {
	if requestedID == "" {
		return false
	}
	if peripheral.ID == requestedID {
		return true
	}
	return allowNameFallback && peripheral.Name != "" && timeflip.DeviceID(peripheral.Name) == requestedID
}

func (t *Transport) lookup(id timeflip.DeviceID) (bluetooth.ScanResult, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	result, ok := t.discovered[id]
	return result, ok
}

// PairOS returns a manual pairing action because direct OS pairing is not exposed safely.
func (t *Transport) PairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSPair, id, "Automatic macOS pairing is not available from this adapter. To complete pairing: 1. Keep the TimeFlip2 powered on and nearby. 2. If macOS shows a Bluetooth pairing prompt during connect, authorize, read, or write, approve it. 3. If no prompt appears, open System Settings > Bluetooth, find the TimeFlip2 device, and click Connect or Pair. 4. Return to this demo and run the suggested connect/read commands.")
}

// UnpairOS returns a manual unpairing action because direct OS unpairing is not exposed safely.
func (t *Transport) UnpairOS(_ context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return unsupportedOSAction(timeflip.ManualActionOSUnpair, id, "Automatic macOS unpairing is not available from this adapter. To complete unpairing: 1. Open System Settings > Bluetooth. 2. Find the TimeFlip2 device. 3. Open the device details and choose Forget This Device or Remove. 4. Return to this demo and run list or pair again.")
}

func advertisedServices(result bluetooth.ScanResult) []timeflip.ServiceID {
	uuids := result.ServiceUUIDs()
	services := make([]timeflip.ServiceID, 0, len(uuids))
	for _, uuid := range uuids {
		services = append(services, serviceID(uuid))
	}
	return services
}

func manufacturerData(result bluetooth.ScanResult) []byte {
	elements := result.ManufacturerData()
	var out []byte
	for _, element := range elements {
		out = append(out, byte(element.CompanyID), byte(element.CompanyID>>8))
		out = append(out, element.Data...)
	}
	return out
}

func appendOrReplacePeripheral(peripherals []timeflip.Peripheral, next timeflip.Peripheral) []timeflip.Peripheral {
	for i := range peripherals {
		if peripherals[i].ID == next.ID {
			peripherals[i] = next
			return peripherals
		}
	}
	return append(peripherals, next)
}
