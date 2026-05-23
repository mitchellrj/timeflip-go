package timeflip

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput indicates invalid caller input.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUnsupportedDevice indicates a peripheral is not a supported TimeFlip2 device.
	ErrUnsupportedDevice = errors.New("unsupported device")
	// ErrUnsupportedOperation indicates an OS operation or command is unsupported.
	ErrUnsupportedOperation = errors.New("unsupported operation")
	// ErrAuthorizationFailed indicates password authorization failed.
	ErrAuthorizationFailed = errors.New("authorization failed")
	// ErrCommandRejected indicates the device rejected a command.
	ErrCommandRejected = errors.New("command rejected")
	// ErrProtocol indicates malformed or unexpected protocol data.
	ErrProtocol = errors.New("protocol error")
	// ErrTimeout indicates an operation exceeded its communication timeout.
	ErrTimeout = errors.New("timeout")
	// ErrDisconnected indicates the active connection ended early.
	ErrDisconnected = errors.New("disconnected")
)

// OperationError adds operation context to an error.
type OperationError struct {
	Operation string
	DeviceID  DeviceID
	Stage     string
	Command   CommandCode
	Err       error
}

// ProtocolPayloadError describes malformed or unexpected protocol bytes.
type ProtocolPayloadError struct {
	Expected string
	Payload  []byte
}

// Error returns a diagnostic protocol error string.
func (e *ProtocolPayloadError) Error() string {
	if e == nil {
		return ErrProtocol.Error()
	}
	msg := ErrProtocol.Error()
	if e.Expected != "" {
		msg += ": expected " + e.Expected
	}
	msg += fmt.Sprintf("; got %d byte(s)", len(e.Payload))
	if len(e.Payload) > 0 {
		msg += fmt.Sprintf("; raw=0x%X", e.Payload)
	}
	return msg
}

// Unwrap returns the protocol sentinel.
func (e *ProtocolPayloadError) Unwrap() error {
	return ErrProtocol
}

// Error returns a contextual error string.
func (e *OperationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := e.Operation
	if e.Stage != "" {
		msg += " stage " + e.Stage
	}
	if e.DeviceID != "" {
		msg += " device " + string(e.DeviceID)
	}
	if e.Command != 0 {
		msg += fmt.Sprintf(" command 0x%02X", byte(e.Command))
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap returns the wrapped error.
func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsTimeout reports whether err represents a timeout.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}

func newProtocolPayloadError(expected string, payload []byte) error {
	return &ProtocolPayloadError{Expected: expected, Payload: append([]byte(nil), payload...)}
}

// IsUnsupported reports whether err represents an unsupported operation.
func IsUnsupported(err error) bool {
	return errors.Is(err, ErrUnsupportedOperation) || errors.Is(err, ErrUnsupportedDevice)
}

// IsAuthorization reports whether err represents authorization failure.
func IsAuthorization(err error) bool {
	return errors.Is(err, ErrAuthorizationFailed)
}
