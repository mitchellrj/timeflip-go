package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	timeflip "github.com/mitchellrj/timeflip-go"
)

type traceLogger struct {
	out io.Writer
	mu  sync.Mutex
}

// NewTracingTransport wraps a transport and logs raw BLE operations.
func NewTracingTransport(inner timeflip.Transport, out io.Writer) timeflip.Transport {
	return &tracingTransport{inner: inner, log: &traceLogger{out: out}}
}

type tracingTransport struct {
	inner timeflip.Transport
	log   *traceLogger
}

func (t *tracingTransport) Scan(ctx context.Context, filter timeflip.ScanFilter) ([]timeflip.Peripheral, error) {
	t.log.event("scan", "", "", nil, nil)
	devices, err := t.inner.Scan(ctx, filter)
	t.log.event("scan_result", "", "", nil, err)
	return devices, err
}

func (t *tracingTransport) Connect(ctx context.Context, id timeflip.DeviceID) (timeflip.Connection, error) {
	t.log.event("connect", id, "", nil, nil)
	conn, err := t.inner.Connect(ctx, id)
	t.log.event("connect_result", id, "", nil, err)
	if err != nil {
		return nil, err
	}
	return &tracingConnection{inner: conn, deviceID: id, log: t.log}, nil
}

func (t *tracingTransport) PairOS(ctx context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	t.log.event("os_pair", id, "", nil, nil)
	result, err := t.inner.PairOS(ctx, id)
	t.log.event("os_pair_result", id, "", nil, err)
	return result, err
}

func (t *tracingTransport) UnpairOS(ctx context.Context, id timeflip.DeviceID) (timeflip.OSActionResult, error) {
	t.log.event("os_unpair", id, "", nil, nil)
	result, err := t.inner.UnpairOS(ctx, id)
	t.log.event("os_unpair_result", id, "", nil, err)
	return result, err
}

type tracingConnection struct {
	inner    timeflip.Connection
	deviceID timeflip.DeviceID
	log      *traceLogger
}

func (c *tracingConnection) Read(ctx context.Context, ch timeflip.CharacteristicID) ([]byte, error) {
	c.log.event("read", c.deviceID, ch, nil, nil)
	payload, err := c.inner.Read(ctx, ch)
	if expectedOptionalCharacteristicMiss(ch, err) {
		c.log.event("read_optional_miss", c.deviceID, ch, nil, nil)
		return payload, err
	}
	c.log.event("read_result", c.deviceID, ch, payload, err)
	return payload, err
}

func (c *tracingConnection) Write(ctx context.Context, ch timeflip.CharacteristicID, payload []byte) error {
	c.log.event("write", c.deviceID, ch, payload, nil)
	err := c.inner.Write(ctx, ch, payload)
	c.log.event("write_result", c.deviceID, ch, nil, err)
	return err
}

func (c *tracingConnection) Subscribe(ctx context.Context, ch timeflip.CharacteristicID) (<-chan timeflip.Notification, error) {
	c.log.event("subscribe", c.deviceID, ch, nil, nil)
	notifications, err := c.inner.Subscribe(ctx, ch)
	c.log.event("subscribe_result", c.deviceID, ch, nil, err)
	if err != nil {
		return nil, err
	}
	out := make(chan timeflip.Notification)
	go func() {
		defer close(out)
		for notification := range notifications {
			c.log.event("notify", c.deviceID, notification.Characteristic, notification.Payload, nil)
			select {
			case out <- notification:
			case <-ctx.Done():
				c.log.event("notify_forward_cancelled", c.deviceID, notification.Characteristic, nil, ctx.Err())
				return
			}
		}
	}()
	return out, nil
}

func (c *tracingConnection) Close(ctx context.Context) error {
	c.log.event("close", c.deviceID, "", nil, nil)
	err := c.inner.Close(ctx)
	c.log.event("close_result", c.deviceID, "", nil, err)
	return err
}

func (l *traceLogger) event(name string, deviceID timeflip.DeviceID, characteristic timeflip.CharacteristicID, payload []byte, err error) {
	if l == nil || l.out == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	writef(l.out, "time=%s event=%s", time.Now().Format(time.RFC3339Nano), name)
	if deviceID != "" {
		writef(l.out, " device=%s", deviceID)
	}
	if characteristic != "" {
		writef(l.out, " characteristic=%s", characteristic)
	}
	if payload != nil {
		writef(l.out, " bytes=%d hex=0x%X", len(payload), payload)
	}
	if err != nil {
		writef(l.out, " error=%q", err)
	}
	writeLine(l.out)
}

func expectedOptionalCharacteristicMiss(ch timeflip.CharacteristicID, err error) bool {
	return err != nil && errors.Is(err, timeflip.ErrProtocol) && ch == "0x2A00"
}
