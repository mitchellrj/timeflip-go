package transport

import (
	"context"
	"errors"
	"sync"

	timeflip "github.com/mitchellrj/timeflip-go"
)

// FakeTransport is a deterministic transport for tests.
type FakeTransport struct {
	Peripherals  []timeflip.Peripheral
	Connections  map[timeflip.DeviceID]*FakeConnection
	PairResult   timeflip.OSActionResult
	UnpairResult timeflip.OSActionResult
	ScanErr      error
	ConnectErr   error
	PairErr      error
	UnpairErr    error
}

// Scan returns configured peripherals.
func (f *FakeTransport) Scan(context.Context, timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	if f.ScanErr != nil {
		return nil, f.ScanErr
	}
	return append([]timeflip.Peripheral(nil), f.Peripherals...), nil
}

// Connect returns a configured fake connection.
func (f *FakeTransport) Connect(_ context.Context, id timeflip.DeviceID) (timeflip.Connection, error) {
	if f.ConnectErr != nil {
		return nil, f.ConnectErr
	}
	if f.Connections == nil {
		return nil, errors.New("connection not found")
	}
	conn, ok := f.Connections[id]
	if !ok {
		return nil, errors.New("connection not found")
	}
	return conn, nil
}

// PairOS returns the configured pairing result.
func (f *FakeTransport) PairOS(context.Context, timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return f.PairResult, f.PairErr
}

// UnpairOS returns the configured unpairing result.
func (f *FakeTransport) UnpairOS(context.Context, timeflip.DeviceID) (timeflip.OSActionResult, error) {
	return f.UnpairResult, f.UnpairErr
}

// FakeConnection is a deterministic connection for tests.
type FakeConnection struct {
	mu            sync.Mutex
	Reads         map[timeflip.CharacteristicID][]byte
	Writes        []Write
	Subscriptions map[timeflip.CharacteristicID]chan timeflip.Notification
	ReadErr       error
	WriteErr      error
	SubscribeErr  error
	CloseErr      error
	Closed        bool
}

// Write records a fake characteristic write.
type Write struct {
	Characteristic timeflip.CharacteristicID
	Payload        []byte
}

// Read returns a configured characteristic payload.
func (f *FakeConnection) Read(_ context.Context, ch timeflip.CharacteristicID) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ReadErr != nil {
		return nil, f.ReadErr
	}
	return append([]byte(nil), f.Reads[ch]...), nil
}

// Write records a characteristic write.
func (f *FakeConnection) Write(_ context.Context, ch timeflip.CharacteristicID, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.WriteErr != nil {
		return f.WriteErr
	}
	f.Writes = append(f.Writes, Write{Characteristic: ch, Payload: append([]byte(nil), payload...)})
	return nil
}

// Subscribe returns or creates a notification channel.
func (f *FakeConnection) Subscribe(_ context.Context, ch timeflip.CharacteristicID) (<-chan timeflip.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SubscribeErr != nil {
		return nil, f.SubscribeErr
	}
	if f.Subscriptions == nil {
		f.Subscriptions = map[timeflip.CharacteristicID]chan timeflip.Notification{}
	}
	stream, ok := f.Subscriptions[ch]
	if !ok {
		stream = make(chan timeflip.Notification, 8)
		f.Subscriptions[ch] = stream
	}
	return stream, nil
}

// Close marks the connection closed.
func (f *FakeConnection) Close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Closed = true
	return f.CloseErr
}
