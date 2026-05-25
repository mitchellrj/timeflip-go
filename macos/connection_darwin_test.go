//go:build darwin

package macos

import (
	"context"
	"errors"
	"testing"

	timeflip "github.com/mitchellrj/timeflip-go"
	"tinygo.org/x/bluetooth"
)

func TestConnectionResolveMissingCharacteristic(t *testing.T) {
	conn := &Connection{
		deviceID:        "tf",
		characteristics: map[timeflip.CharacteristicID]bluetooth.DeviceCharacteristic{},
		done:            make(chan struct{}),
	}
	_, err := conn.resolve("0x2A19")
	if !errors.Is(err, timeflip.ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestConnectionCloseIsIdempotent(t *testing.T) {
	conn := &Connection{
		deviceID:        "tf",
		characteristics: map[timeflip.CharacteristicID]bluetooth.DeviceCharacteristic{},
		subscriptions:   map[timeflip.CharacteristicID]*subscription{},
		done:            make(chan struct{}),
	}
	if err := conn.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConnectionCloseReturnsContextErrorFromSubscriptionShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn := &Connection{
		deviceID:        "tf",
		characteristics: map[timeflip.CharacteristicID]bluetooth.DeviceCharacteristic{},
		subscriptions: map[timeflip.CharacteristicID]*subscription{
			"ch": {
				out: make(chan timeflip.Notification),
				disable: func(ctx context.Context) error {
					return ctx.Err()
				},
			},
		},
		done: make(chan struct{}),
	}
	err := conn.Close(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
