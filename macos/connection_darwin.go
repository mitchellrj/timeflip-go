//go:build darwin

package macos

import (
	"context"
	"sync"

	timeflip "github.com/mitchellrj/timeflip-go"
	"tinygo.org/x/bluetooth"
)

type readResult struct {
	n   int
	err error
}

type writeResult struct {
	err error
}

type discoverResult struct {
	err error
}

// Connection provides GATT operations for one connected TimeFlip2 peripheral.
type Connection struct {
	deviceID timeflip.DeviceID
	device   bluetooth.Device

	mu              sync.Mutex
	connected       bool
	closed          bool
	closeOnce       sync.Once
	characteristics map[timeflip.CharacteristicID]bluetooth.DeviceCharacteristic
	subscriptions   map[timeflip.CharacteristicID]*subscription
	done            chan struct{}
}

func (c *Connection) discoverCharacteristics() error {
	services, err := c.device.DiscoverServices(nil)
	if err != nil {
		return err
	}
	foundTimeFlipService := false
	for _, service := range services {
		if normalizeServiceID(serviceID(service.UUID())) == normalizeServiceID(timeflip.TimeFlipService) {
			foundTimeFlipService = true
		}
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			return err
		}
		for _, char := range chars {
			c.characteristics[characteristicID(char.UUID())] = char
		}
	}
	if !foundTimeFlipService {
		return timeflip.ErrProtocol
	}
	for _, ch := range timeflip.RequiredCharacteristics() {
		if _, ok := c.characteristics[normalizeCharacteristicID(ch)]; !ok {
			return timeflip.ErrProtocol
		}
	}
	return nil
}

// Read reads the current characteristic value.
func (c *Connection) Read(ctx context.Context, ch timeflip.CharacteristicID) ([]byte, error) {
	characteristic, err := c.resolve(ch)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, operationErr("macos_read", c.deviceID, err)
	}
	buf := make([]byte, 512)
	result := make(chan readResult, 1)
	go func() {
		n, err := characteristic.Read(buf)
		result <- readResult{n: n, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, operationErr("macos_read", c.deviceID, ctx.Err())
	case res := <-result:
		if res.err != nil {
			return nil, operationErr("macos_read", c.deviceID, res.err)
		}
		return append([]byte(nil), buf[:res.n]...), nil
	}
}

// Write writes a characteristic value.
func (c *Connection) Write(ctx context.Context, ch timeflip.CharacteristicID, payload []byte) error {
	characteristic, err := c.resolve(ch)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return operationErr("macos_write", c.deviceID, err)
	}
	copied := append([]byte(nil), payload...)
	result := make(chan writeResult, 1)
	go func() {
		_, err := characteristic.Write(copied)
		result <- writeResult{err: err}
	}()
	select {
	case <-ctx.Done():
		return operationErr("macos_write", c.deviceID, ctx.Err())
	case res := <-result:
		if res.err != nil {
			return operationErr("macos_write", c.deviceID, res.err)
		}
		return nil
	}
}

// Subscribe enables characteristic notifications and returns a Go notification channel.
func (c *Connection) Subscribe(ctx context.Context, ch timeflip.CharacteristicID) (<-chan timeflip.Notification, error) {
	characteristic, err := c.resolve(ch)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, operationErr("macos_subscribe", c.deviceID, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &subscription{
		ch:     ch,
		out:    make(chan timeflip.Notification, 8),
		cancel: cancel,
		disable: func() {
			_ = enableNotifications(context.Background(), characteristic, nil)
		},
	}
	if err := enableNotifications(ctx, characteristic, func(payload []byte) {
		sub.deliver(payload)
	}); err != nil {
		cancel()
		return nil, operationErr("macos_subscribe", c.deviceID, err)
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		sub.close()
		return nil, operationErr("macos_subscribe", c.deviceID, timeflip.ErrDisconnected)
	}
	if existing := c.subscriptions[normalizeCharacteristicID(ch)]; existing != nil {
		existing.close()
	}
	c.subscriptions[normalizeCharacteristicID(ch)] = sub
	c.mu.Unlock()

	go func() {
		select {
		case <-subCtx.Done():
		case <-c.done:
		}
		c.cancelSubscription(ch)
	}()
	return sub.out, nil
}

// Close disconnects from the BLE peripheral and closes active subscriptions.
func (c *Connection) Close(context.Context) error {
	var err error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		if c.done != nil {
			close(c.done)
		}
		subs := make([]*subscription, 0, len(c.subscriptions))
		for _, sub := range c.subscriptions {
			subs = append(subs, sub)
		}
		c.subscriptions = map[timeflip.CharacteristicID]*subscription{}
		connected := c.connected
		c.connected = false
		c.mu.Unlock()

		for _, sub := range subs {
			sub.close()
		}
		if connected {
			err = c.device.Disconnect()
			if err != nil {
				err = operationErr("macos_close", c.deviceID, err)
			}
		}
	})
	return err
}

func (c *Connection) resolve(ch timeflip.CharacteristicID) (bluetooth.DeviceCharacteristic, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return bluetooth.DeviceCharacteristic{}, operationErr("macos_characteristic", c.deviceID, timeflip.ErrDisconnected)
	}
	characteristic, ok := c.characteristics[normalizeCharacteristicID(ch)]
	if !ok {
		return bluetooth.DeviceCharacteristic{}, operationErr("macos_characteristic", c.deviceID, timeflip.ErrProtocol)
	}
	return characteristic, nil
}

func (c *Connection) cancelSubscription(ch timeflip.CharacteristicID) {
	c.mu.Lock()
	sub := c.subscriptions[normalizeCharacteristicID(ch)]
	delete(c.subscriptions, normalizeCharacteristicID(ch))
	c.mu.Unlock()
	if sub != nil {
		sub.close()
	}
}

func enableNotifications(ctx context.Context, characteristic bluetooth.DeviceCharacteristic, callback func([]byte)) error {
	result := make(chan error, 1)
	go func() {
		result <- characteristic.EnableNotifications(callback)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-result:
		return err
	}
}

type subscription struct {
	mu      sync.Mutex
	ch      timeflip.CharacteristicID
	out     chan timeflip.Notification
	cancel  context.CancelFunc
	disable func()
	closed  bool
}

func (s *subscription) deliver(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	notification := timeflip.Notification{
		Characteristic: s.ch,
		Payload:        append([]byte(nil), payload...),
	}
	select {
	case s.out <- notification:
	default:
	}
}

func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	if s.disable != nil {
		s.disable()
	}
	close(s.out)
}
