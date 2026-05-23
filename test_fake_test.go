package timeflip

import (
	"context"
	"errors"
	"sync"
)

type fakeTransport struct {
	peripherals  []Peripheral
	connections  map[DeviceID]*fakeConnection
	pairResult   OSActionResult
	unpairResult OSActionResult
	scanErr      error
	connectErr   error
	pairErr      error
	unpairErr    error
}

func (f *fakeTransport) Scan(context.Context, ScanFilter) ([]Peripheral, error) {
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return append([]Peripheral(nil), f.peripherals...), nil
}

func (f *fakeTransport) Connect(_ context.Context, id DeviceID) (Connection, error) {
	if f.connectErr != nil {
		return nil, f.connectErr
	}
	if f.connections == nil {
		return nil, errors.New("connection not found")
	}
	conn, ok := f.connections[id]
	if !ok {
		return nil, errors.New("connection not found")
	}
	return conn, nil
}

func (f *fakeTransport) PairOS(context.Context, DeviceID) (OSActionResult, error) {
	return f.pairResult, f.pairErr
}

func (f *fakeTransport) UnpairOS(context.Context, DeviceID) (OSActionResult, error) {
	return f.unpairResult, f.unpairErr
}

type fakeConnection struct {
	mu            sync.Mutex
	reads         map[CharacteristicID][]byte
	readErrs      map[CharacteristicID]error
	writes        []fakeWrite
	subscriptions map[CharacteristicID]chan Notification
	readErr       error
	writeErr      error
	subscribeErr  error
	closeErr      error
	closed        bool
}

type fakeWrite struct {
	characteristic CharacteristicID
	payload        []byte
}

func (f *fakeConnection) Read(_ context.Context, ch CharacteristicID) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return nil, f.readErr
	}
	if err := f.readErrs[ch]; err != nil {
		return nil, err
	}
	return append([]byte(nil), f.reads[ch]...), nil
}

func (f *fakeConnection) Write(_ context.Context, ch CharacteristicID, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes = append(f.writes, fakeWrite{characteristic: ch, payload: append([]byte(nil), payload...)})
	return nil
}

func (f *fakeConnection) Subscribe(_ context.Context, ch CharacteristicID) (<-chan Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subscribeErr != nil {
		return nil, f.subscribeErr
	}
	if f.subscriptions == nil {
		f.subscriptions = map[CharacteristicID]chan Notification{}
	}
	stream, ok := f.subscriptions[ch]
	if !ok {
		stream = make(chan Notification, 8)
		f.subscriptions[ch] = stream
	}
	return stream, nil
}

func (f *fakeConnection) Close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}
